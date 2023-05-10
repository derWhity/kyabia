package scraper

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/derWhity/kyabia/internal/models"
	"golang.org/x/text/language"
)

// -- Scraping functions -----------------------------------------------------------------------------------------------

const (
	ffTypeVideo = "video"
	ffTypeAudio = "audio"
	ffTypeSub   = "subtitle"
)

// FFProbeData is the data format the tool ffprobe generates in JSON
type FFProbeData struct {
	Format  *FFFormatInfo   `json:"format"`
	Streams []*FFStreamInfo `json:"streams"`
}

// GetFirstSteamByType returns the first stream in the media file's streams that has the given type
func (d *FFProbeData) GetFirstSteamByType(t string) *FFStreamInfo {
	if len(d.Streams) > 0 {
		for _, s := range d.Streams {
			if s.CodecType == t {
				return s
			}
		}
	}
	return nil
}

// FFFormatInfo contains information about the media format.
type FFFormatInfo struct {
	Filename       string            `json:"filename"`
	NumStreams     uint              `json:"nb_streams"`
	NumPrograms    uint              `json:"nb_programs"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	StartTime      string            `json:"start_time"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	Bitrate        string            `json:"bit_rate"`
	ProbeScore     uint              `json:"probe_score"`
	Tags           map[string]string `json:"tags"`
}

// FFStreamInfo contains information about a stream inside a media file
// This struct does not contain all of the fields returned by ffprobe
type FFStreamInfo struct {
	CodecType     string `json:"codec_type"`
	CodecName     string `json:"codec_name"`
	CodecLongName string `json:"codec_long_name"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Bitrate       string `json:"bit_rate"`
}

// ScrapeFFProbe uses the ffprobe commandline tool to scrape the video metadata from its JSON output
func ScrapeFFProbe(filename string, vid *models.Video, logger *logrus.Entry) error {
	logger = logger.WithField("scraper", "FFProbe")
	logger.Debug("Start scraping")
	data, err := exec.Command(
		"ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", filename,
	).Output()
	if err != nil {
		logger.WithError(err).Error("Could not execute ffprobe")
		return fmt.Errorf("failed to execute ffprobe for %s: %v", filename, err)
	}
	probeData := &FFProbeData{}
	if err := json.Unmarshal(data, probeData); err != nil {
		logger.WithError(err).Error("Failed to parse ffprobe JSON output")
		return fmt.Errorf("failed to read ffprobe output for %s: %v", filename, err)
	}
	// Get general info
	if probeData.Format != nil {
		if i, err := strconv.ParseInt(
			probeData.Format.Duration[0:strings.Index(probeData.Format.Duration, ".")], 10, 0,
		); err == nil {
			vid.Duration = time.Duration(i) * time.Second
		}
	}
	// Get video info
	if str := probeData.GetFirstSteamByType(ffTypeVideo); str != nil {
		vid.VideoFormat = str.CodecName
		vid.Width = str.Width
		vid.Height = str.Height
		if i, err := strconv.ParseInt(str.Bitrate, 10, 0); err == nil {
			vid.VideoBitrate = int(i)
		}
	}
	// Get audio info
	if str := probeData.GetFirstSteamByType(ffTypeAudio); str != nil {
		vid.AudioFormat = str.CodecName
		if i, err := strconv.ParseInt(str.Bitrate, 10, 0); err == nil {
			vid.AudioBitrate = int(i)
		}
	}
	logger.Debug("Scraping finished")
	return nil
}

// ScrapeSHA512 calculates the SHA-512 sum of the video file and adds it to the video metadata provided
func ScrapeSHA512(filename string, vid *models.Video, logger *logrus.Entry) error {
	logger = logger.WithField("scraper", "SHA-512")
	logger.Debug("Start scraping")
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	// Read only 1 MiB from the video file
	readLen := 1024 * 1024
	b := make([]byte, readLen)
	f.Read(b)
	buf := bytes.NewBuffer(b)

	sha := sha512.New()
	_, err = io.Copy(sha, buf)
	if err != nil {
		return fmt.Errorf("failed to calculate SHA512 sum of file %s: %v", filename, err)
	}
	vid.SHA512 = hex.EncodeToString(sha.Sum(nil))
	logger.Debug("Scraping finished")
	return nil
}

// MakeFileNameScraper returns a scraping function that uses a regular expression to extract data from a file's name
// using capturing groups. These extracted data fields are then mapped to fields of the video struct, resulting in
// filling them with the appropriate data
//
// Regex and field mappings are derived by taking them from the presets stored in this
func MakeFileNameScraper(presetName string) (ScrapingFunc, error) {
	preset, ok := fileNameScrapingPresets[presetName]
	if !ok {
		return nil, fmt.Errorf("MakeFileNameScraper: Cannot find preset '%s'", presetName)
	}
	reg, err := regexp.Compile(preset.Regex)
	if err != nil {
		return nil, fmt.Errorf("MakeFileSchemaScraper: Cannot create scraper: %v", err)
	}
	return func(filename string, vid *models.Video, logger *logrus.Entry) error {
		filename = path.Base(filename)
		logger = logger.WithField("scraper", "FileName")
		logger.WithField("regex", preset.Regex).Debug("Start scraping")
		matches := reg.FindStringSubmatch(filename)
		// We will only do anything if there is at least one match
		if len(matches) != 0 {
			logger.Debugf("Found %d match(es)", len(matches))
			for fieldName, idx := range preset.FieldMap {
				if idx >= 0 && idx < len(matches) {
					val := strings.TrimSpace(matches[idx])
					// I know this would also work with reflection, but... Maybe later
					switch fieldName {
					case FldIdentifier:
						vid.Identifier = val
					case FldArtist:
						vid.Artist = val
					case FldTitle:
						vid.Title = val
					case FldRelatedMedium:
						vid.RelatedMedium = val
					case FldMediumDetail:
						vid.MediumDetail = val
					case FldDescription:
						vid.Description = val
					case FldLanguage:
						tag, err := language.Parse(val)
						if err == nil {
							fmt.Printf("Language found: %v\n", tag)
							vid.Language = tag.String()
						}
					}
				}
			}
		} else {
			logger.Debug("No match found")
		}
		logger.Debug("Scraping finished")
		return nil
	}, nil
}

// MustMakeFileNameScraper is a version of MakeFileNameScraper that panics when creating the scraping function fails
func MustMakeFileNameScraper(presetName string) ScrapingFunc {
	fn, err := MakeFileNameScraper(presetName)
	if err != nil {
		panic(fmt.Sprintf("MustMakeFileNameScraper: Cannot create scraping function: %v", presetName))
	}
	return fn
}
