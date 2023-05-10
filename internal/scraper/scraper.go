// Package scraper provides video file scraping functionality
package scraper

import (
	"fmt"
	"mime"
	"os"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"path/filepath"

	"github.com/derWhity/kyabia/internal/log"
	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
)

const (
	// FldTitle marks a match for the "Title"" field of a video
	FldTitle = "Title"
	// FldArtist marks a match for the "Artist"" field of a video
	FldArtist = "Artist"
	// FldRelatedMedium marks a match for the "RelatedMedium"" field of a video
	FldRelatedMedium = "RelatedMedium"
	// FldMediumDetail marks a match for the "MediumDetail"" field of a video
	FldMediumDetail = "MediumDetail"
	// FldDescription marks a match for the "Description"" field of a video
	FldDescription = "Description"
	// FldLanguage marks a match for the "Language"" field of a video
	FldLanguage = "Language"
	// FldIdentifier marks a match for the "Identifier" field of a video
	FldIdentifier = "Identifier"
	// StatusQueued is the status a scrape has when it waits to be started
	StatusQueued = iota
	// StatusRunning is the status of a scrape that is currently active
	StatusRunning
	// StatusFinished is the status of a scrape that has been finished successfully
	StatusFinished
	// StatusFailed is the status of a scrape that has failed for some reason. To look up the reason, see the Err field
	// of the scrape struct
	StatusFailed
	// StatusCancelled is the status of a scrape that has been cancelled by the user
	StatusCancelled
)

var (
	// The scraping presets available - can be used when constructing file name scraping functions
	fileNameScrapingPresets map[string]NameScrapingPreset
	// ErrAlreadyQueued is the error that is returned when scraping the same or a parent directory is already inside
	// the scraping queue
	ErrAlreadyQueued = fmt.Errorf("a scraping operation is already queued for this directory")
)

// FieldIndexMap describes the correlation between a field of a video struct and the index in the scraping result
// that will be used to fill this field. It is used in the "FileNameSchema" scraper for configuring which field
// will be filled with the result of which capture group of the regular expression
type FieldIndexMap map[string]int

// NameScrapingPreset defines a named preset for scraping file names
// It is used to make it easier for repeating scrapes of a special kind
type NameScrapingPreset struct {
	Name     string
	Regex    string
	FieldMap FieldIndexMap
}

// ScrapeStatus defines the status of a scrape
type ScrapeStatus uint

// Scrape describes a video scraping operation currently running
type Scrape struct {
	// The current status of the scrape. See the Status* constants for possible values
	Status ScrapeStatus `json:"status"`
	// The root directory where the scraping started - since multiple scrapes in the same directory are not allowed,
	// this can also be seen as a unique ID
	RootDir string `json:"rootDir"`
	// The directory currently being traversed
	CurrentDir string `json:"currentDir"`
	// The file currently being scanned
	CurrentFile string `json:"currentFile"`
	// The number of video files already scraped
	NumFiles uint `json:"filesScraped"`
	// The number of new video files scraped
	NumNewFiles uint `json:"newFiles"`
	// The number of already existing video files updated
	NumUpdatedFiles uint `json:"updatedFiles"`
	// The time the scape has started
	StartedAt time.Time `json:"startedAt"`
	// If the scrape has failed, this is the error that caused it
	Err error `json:"error"`
	// Internal channel that will be closed when the scraping operation needs to be stopped
	stopChan chan bool
	// The logger to use for this scrape
	logger *logrus.Entry
	// The list of scraping functions to execute during this scrape
	fns []ScrapingFunc
	// The video repo to use
	vRepo repos.VideoRepo
}

// A ScrapingFunc is a function that scrapes a file identified by its file name and writes the found meta data into the
// video struct provided
type ScrapingFunc func(filename string, vid *models.Video, logger *logrus.Entry) error

// A request that can be (depending on the channel it is used on) either a request for starting a new scrape or one
// to retrieve the current status of a running scrape
type scrapeRequest struct {
	// The root directory the scrape has started or should be started
	rootDir string
	// The Scrape object requested. If this one is nil, the requested scrape does not exist.
	// To check if anything bad happened, the scrape contains an err field that contains any error that cancelled the
	// scraping operations
	answer chan<- *Scrape
}

type scrapeMap map[string]Scrape

// A Scraper runs a set of Scraping functions on files fed to it
type Scraper struct {
	vRepo  repos.VideoRepo
	fns    []ScrapingFunc
	logger *logrus.Entry
	// The channel used for starting new scrapes
	startChan chan<- scrapeRequest
	// The channel used for stopping running scrapes
	stopChan chan<- scrapeRequest
	// The channel used to retrieve status information for
	statusChan chan<- scrapeRequest
	// This is a token semaphore that is used to only start a specific number of scraping operations at once
	queueSemaphore chan interface{}
}

// New returns a new scraper with the given functions set as scraping functions
func New(vRepo repos.VideoRepo, functions []ScrapingFunc, logger *logrus.Entry) *Scraper {
	return &Scraper{
		vRepo:          vRepo,
		fns:            functions,
		logger:         logger,
		queueSemaphore: make(chan interface{}, 2), // Only two scrapes are allowed in parallel
	}
}

// NewDefault creates a new scraper that is setup using the default scraping functions
func NewDefault(vRepo repos.VideoRepo, logger *logrus.Entry) *Scraper {
	return New(
		vRepo,
		[]ScrapingFunc{
			ScrapeSHA512,
			ScrapeFFProbe,
			MustMakeFileNameScraper("ID_Language_Artist_Title_Type_Anime"),
			MustMakeFileNameScraper("ID_Anime_Title (Type)"),
			// Disabled for now
			// MustMakeFileNameScraper("ID-Language-Artist-Title-Type-Anime"),
			// MustMakeFileNameScraper("ID-Anime-Title (Type)"),
		},
		logger,
	)
}

// Start begins scraping from the given root directory. It returns the ID of the
func (s *Scraper) Start(rootDir string) error {
	s.logger.WithField(log.FldPath, rootDir).Debug("Starting scrape")
	if s.startChan == nil {
		// We do not have a control method running right now so start one
		start := make(chan scrapeRequest)
		stop := make(chan scrapeRequest)
		status := make(chan scrapeRequest)
		s.startChan = start
		s.stopChan = stop
		s.statusChan = status
		go s.manage(start, stop, status)
	}
	ret := make(chan *Scrape)
	s.startChan <- scrapeRequest{
		rootDir: rootDir,
		answer:  ret,
	}
	// Retrieve the answer to check if there was an error
	scrape := <-ret
	if scrape.Status == StatusFailed {
		return scrape.Err
	}
	return nil
}

// Stop stops the given scrape in a controlled manner
// This method will block until the scrape is stopped
func (s *Scraper) Stop(rootDir string) {
	if s.stopChan == nil {
		return
	}
	c := make(chan *Scrape)
	s.stopChan <- scrapeRequest{rootDir, c}
	for range c {
		// Just wait until the channel is closed
	}
}

// StopAll stops all running scrapes and lets them exit in a controlled manner
// This method will block until all scrapes have stopped
func (s *Scraper) StopAll() {
	s.Stop("")
}

// Internal status retrieval function used by both, Status() and StatusAll()
func (s *Scraper) doGetStatus(rootDir string) []Scrape {
	var ret []Scrape
	if s.statusChan == nil {
		// Management goroutine not running
		return ret
	}
	answer := make(chan *Scrape)
	req := scrapeRequest{
		rootDir: rootDir,
		answer:  answer,
	}
	// Send the request to the management goroutine
	s.statusChan <- req
	for scr := range answer {
		ret = append(ret, *scr)
	}
	return ret
}

// StatusAll returns the status object for all scrapes currently available in the scrape list
func (s *Scraper) StatusAll() []Scrape {
	return s.doGetStatus("")
}

// Status retrieves the status of the scrape running for the given root directory
func (s *Scraper) Status(rootDir string) *Scrape {
	if rootDir == "" {
		// This would return all scrapes, but we'll handle this as if this was an illegal scrape path
		return nil
	}
	scrapes := s.doGetStatus(rootDir)
	if len(scrapes) > 0 {
		return &scrapes[0]
	}
	return nil
}

// scrapeRunning checks if a scrape for the same directory is already running inside the list of running scrapes
func scrapeRunning(runningScrapes scrapeMap, newRootDir string) bool {
	for _, scrape := range runningScrapes {
		if (scrape.Status == StatusRunning || scrape.Status == StatusQueued) &&
			strings.Contains(newRootDir, scrape.RootDir) {
			return true
		}
	}
	return false
}

// manage is the host function that controls running scrapes until the last scrape has ended
func (s *Scraper) manage(start <-chan scrapeRequest, stop <-chan scrapeRequest, statusOut <-chan scrapeRequest) {
	s.logger.Debug("Starting scraper control goroutine")
	scrapes := make(scrapeMap)
	// Aggregate channel to receive status updates at
	status := make(chan Scrape)
	for {
		select {
		case s := <-status:
			// A status update arrived
			s.logger.WithField("scrape", s).Debug("Status update for scrape")
			scrapes[s.RootDir] = s
		case statusReq := <-statusOut:
			// A request for the current status of a scrape has been requested
			s.logger.WithField(log.FldPath, statusReq.rootDir).Debug("Status request received for scrape")
			if statusReq.rootDir == "" {
				// Special case: Return ALL scrapes available and then close the channel
				for _, scr := range scrapes {
					data := scr // Copy
					statusReq.answer <- &data
				}
			} else {
				if scr, ok := scrapes[statusReq.rootDir]; ok {
					data := scr // Copy
					statusReq.answer <- &data
				} else {
					statusReq.answer <- nil
				}
			}
			// Finished - always close the answer channel
			close(statusReq.answer)
		case startReq := <-start:
			// We need to start a new scrape
			scr := s.startScraping(startReq.rootDir, scrapes, status)
			startReq.answer <- &scr
		case stopReq := <-stop:
			// We'll need to stop the scrape having the given root directory
			s.logger.WithField(log.FldPath, stopReq.rootDir).Info("Stop request received")
			// The manage function needs to continue here - else we would have a deadlock
			go func() {
				if stopReq.rootDir == "" {
					// Stop all scrapes
					for _, scr := range scrapes {
						scr.Stop()
					}
				} else {
					// Stop only one scrape
					if scr, ok := scrapes[stopReq.rootDir]; ok {
						scr.Stop()
					}
				}
				// We're done - close the answer channel
				close(stopReq.answer)
			}()
		}
	}
}

// Internal function that is used to check the prerequisites for the intended scraping operation, retrieves
func (s *Scraper) startScraping(rootDir string, running map[string]Scrape, statusChan chan<- Scrape) Scrape {
	logger := s.logger.WithField(log.FldPath, rootDir)
	logger.Debug("Incoming scraping request")
	stop := make(chan bool)
	scr := Scrape{
		vRepo:     s.vRepo,
		RootDir:   rootDir,
		Status:    StatusQueued,
		StartedAt: time.Now(),
		stopChan:  stop,
		logger:    logger,
		fns:       s.fns,
	}
	if scrapeRunning(running, rootDir) {
		scr.Err = ErrAlreadyQueued
		scr.Status = StatusFailed
		return scr
	}
	// Everything all right - let's get a token from the semaphore and start after getting one
	go func() {
		// Make a copy to not bleed through to the manage goroutine
		scr := scr
		scr.logger.Info("Scraping operation queued")
		s.queueSemaphore <- 1 // Take a token from the semaphore
		scr.logger.Info("Scraping operation is starting")
		scr.Status = StatusRunning
		scr.CurrentDir = scr.RootDir
		statusChan <- scr
		err := scr.walkDir(statusChan, stop)
		// Reset the file status
		scr.CurrentDir = ""
		scr.CurrentFile = ""
		if err != nil {
			scr.Status = StatusFailed
			scr.Err = err
		} else if scr.Status != StatusCancelled {
			scr.Status = StatusFinished
		}
		scr.logger.Info("Scraping operation has finished")
		<-s.queueSemaphore // Return the token
		statusChan <- scr
	}()
	return scr
}

// Stop sends the stop signal to the current scrape's goroutine
// This method blocks until the scrape has stopped
func (scr Scrape) Stop() {
	switch scr.Status {
	case StatusRunning:
		scr.stopChan <- true
	case StatusQueued:
		// Do not block here - the channel will be stopped right after being starting as the queue clears up
		go func() {
			scr.stopChan <- true
		}()
	default:
		// All others do not need to be stopped
		return
	}
}

// walkDir traverses a directory tree beginning at the given dir and scrapes all video files it can find using the
// scraping functions configured
func (scr *Scrape) walkDir(status chan<- Scrape, stop <-chan bool) error {
	dir := scr.CurrentDir
	fileInfo, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return fmt.Errorf("directory '%s' does not exist or cannot be accessed", dir)
		}
		return fmt.Errorf("cannot get directory information for '%s': %v", dir, err)
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("target directory is no directory")
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read contents of directory %s", dir)
	}
	for _, file := range files {
		// Check, if the scraping operation needs to be stopped
		select {
		case <-stop:
			scr.logger.Warn("Received stop command. Finishing right now.")
			scr.Status = StatusCancelled
			scr.CurrentDir = ""
			scr.CurrentFile = ""
			go func() {
				status <- *scr
			}()
			return nil
		default:
			fileName := path.Join(dir, file.Name())
			if file.IsDir() {
				// Recurse deeper into the directory
				scr.CurrentDir = fileName
				scr.CurrentFile = ""
				status <- *scr
				err := scr.walkDir(status, stop)
				if err != nil {
					// This is not the root - so we'll just skip this directory
					scr.logger.WithField("dir", fileName).WithError(err).Warnf("Skipping directory")
				}
			} else {
				// We have a file - does it have a video file type?
				mime := mime.TypeByExtension(path.Ext(fileName))
				if strings.HasPrefix(mime, "video/") {
					// A video file!!! (probably...)
					scr.CurrentFile = fileName
					status <- *scr
					err := scr.file(status)
					if err != nil {
						scr.logger.WithField(log.FldFile, fileName).WithError(err).Warnf("Skipping video file")
					} else {
						// Update our status
						scr.NumFiles = scr.NumFiles + 1
						status <- *scr
					}
				}
			}
		}
	}
	// Just to make sure: Cleanup any waiting stop requests
	select {
	case <-stop:
	default:
	}
	return nil
}

// File takes one file name and scraped this file using all scraping functions configured
func (scr *Scrape) file(status chan<- Scrape) error {
	var vid = models.Video{
		Filename: scr.CurrentFile,
	}
	logger := scr.logger.WithField(log.FldFile, scr.CurrentFile)
	logger.Info("Scraping video file")
	for i, fn := range scr.fns {
		if err := fn(scr.CurrentFile, &vid, logger); err != nil {
			return fmt.Errorf("failed to execute scraper #%d (%v): %v", i, fn, err)
		}
	}
	if vid.SHA512 == "" {
		return fmt.Errorf("file: Cannot add video without a SHA512 hash")
	}
	// If there is no title, use the file name so that we have at least anything to display
	if strings.TrimSpace(vid.Title) == "" {
		fname := filepath.Base(vid.Filename)
		// Remove extension
		fname = fname[:len(fname)-len(filepath.Ext(fname))]
		vid.Title = fname
	}
	// Check if a video with the given SHA512 exists...
	exVid, err := scr.vRepo.GetByID(vid.SHA512)
	if err != nil && err != repos.ErrEntityNotExisting {
		return fmt.Errorf("file: Failed to load video data from repo: %v", err)
	}
	if exVid != nil {
		vid = mergeVideos(*exVid, vid)
		if err = scr.vRepo.Update(&vid); err == nil {
			scr.NumUpdatedFiles = scr.NumUpdatedFiles + 1
		}
	} else {
		if err = scr.vRepo.Create(&vid); err == nil {
			scr.NumNewFiles = scr.NumNewFiles + 1
		}
	}
	return err
}

// Converts the scrape status into a readable name
func (s ScrapeStatus) String() string {
	switch s {
	case StatusQueued:
		return "queued"
	case StatusRunning:
		return "running"
	case StatusFinished:
		return "finished"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	}
	return "unknown"
}

// MarshalJSON implements the json.marshaler interface returning the name of the status
func (s ScrapeStatus) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", s)), nil
}

func (scr Scrape) String() string {
	return fmt.Sprintf(
		"Scrape(%s)[ Status: %s | Started at: %s | Dir: %s | File: %s | Error: %v ]",
		scr.RootDir,
		scr.Status,
		scr.StartedAt,
		scr.CurrentDir,
		scr.CurrentFile,
		scr.Err,
	)
}

// Returns the matching value when matching strings
func mergeString(first string, second string) string {
	if first == "" {
		return second
	}
	return first
}

// Returns the matching value when matching integers
func mergeInt(first int, second int) int {
	if first == 0 {
		return second
	}
	return first
}

// Returns the matching value when matching durations
func mergeDuration(first time.Duration, second time.Duration) time.Duration {
	if first == 0 {
		return second
	}
	return first
}

// Merges the contents of two video entries by applying the fields of the second video to the ones of the first one
// while keeping already populated fields intact
func mergeVideos(first models.Video, second models.Video) models.Video {
	return models.Video{
		VideoSummary: models.VideoSummary{
			SHA512:        first.SHA512,
			Title:         mergeString(first.Title, second.Title),
			Artist:        mergeString(first.Artist, second.Artist),
			Language:      mergeString(first.Language, second.Language),
			RelatedMedium: mergeString(first.RelatedMedium, second.RelatedMedium),
			MediumDetail:  mergeString(first.MediumDetail, second.MediumDetail),
			Description:   mergeString(first.Description, second.Description),
			Duration:      mergeDuration(first.Duration, second.Duration),
			Identifier:    mergeString(first.Identifier, second.Identifier),
		},
		Filename: second.Filename, // Always updated
		Dimensions: models.Dimensions{
			Width:  mergeInt(first.Width, second.Width),
			Height: mergeInt(first.Height, second.Height),
		},
		VideoFormat:  mergeString(first.VideoFormat, second.VideoFormat),
		VideoBitrate: mergeInt(first.VideoBitrate, second.VideoBitrate),
		AudioFormat:  mergeString(first.AudioFormat, second.AudioFormat),
		AudioBitrate: mergeInt(first.AudioBitrate, second.AudioBitrate),
		// Number of plays ignored - they will always be taken from the original entry
	}
}

// -- Initalization ----------------------------------------------------------------------------------------------------

// getDefaultNameScrapingPresets returns the built-in sraping presets for the file name scraping function
func getDefaultNameScrapingPresets() map[string]NameScrapingPreset {
	return map[string]NameScrapingPreset{
		"ID-Language-Artist-Title-Type-Anime": {
			"ID-Language-Artist-Title-Type-Anime",
			`^([0-9]+)\-([^\-]+)\-([^\-]+)\-([^\-]+)\-?([^\-]*)?\-?([^\-]*)?(\-([^\-]*))*\.[^\.]+$`,
			FieldIndexMap{
				FldIdentifier:    1,
				FldLanguage:      2,
				FldArtist:        3,
				FldTitle:         4,
				FldMediumDetail:  5,
				FldRelatedMedium: 6,
			},
		},
		"ID-Anime-Title (Type)": {
			"ID-Anime-Title (Type)",
			`^([0-9]+)\-([^\-]+)\-([^-\(]+)(\(([^\(]+)\))?\.[^\.]+$`,
			FieldIndexMap{
				FldIdentifier:    1,
				FldArtist:        2,
				FldTitle:         3,
				FldMediumDetail:  5,
				FldRelatedMedium: 2,
			},
		},
		"ID_Language_Artist_Title_Type_Anime": {
			"ID_Language_Artist_Title_Type_Anime",
			`^([0-9]+)_([^_]+)_([^_]+)_([^_]+)_?([^_]*)?_?([^_]*)?(_([^_]*))*\.[^\.]+$`,
			FieldIndexMap{
				FldIdentifier:    1,
				FldLanguage:      2,
				FldArtist:        3,
				FldTitle:         4,
				FldMediumDetail:  5,
				FldRelatedMedium: 6,
			},
		},
		"ID_Anime_Title (Type)": {
			"ID_Anime_Title (Type)",
			`^([0-9]+)_([^_]+)\_([^_\(]+)(\(([^\(]+)\))?\.[^\.]+$`,
			FieldIndexMap{
				FldIdentifier:    1,
				FldArtist:        2,
				FldTitle:         3,
				FldMediumDetail:  5,
				FldRelatedMedium: 2,
			},
		},
	}
}

func init() {
	// Insert the default scraping presets
	fileNameScrapingPresets = getDefaultNameScrapingPresets()
}
