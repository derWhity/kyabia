package internal

import (
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/derWhity/kyabia/internal/scraper"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// ScrapingService is a service that provides functionality scraping videos from a directory into the database of Kyabia
type ScrapingService interface {
	ListDirs(ctx context.Context, parentDir string) ([]string, error)
	ListScrapes(ctx context.Context) ([]scraper.Scrape, error)
	GetScrape(ctx context.Context, rootDir string) *scraper.Scrape
	Start(ctx context.Context, rootDir string) error
}

// -- Helpers ----------------------------------------------------------------------------------------------------------

// RootDirSorter is a helper for sorting a scrape slice since Go decided to be complicated here
type RootDirSorter []scraper.Scrape

// Len is the number of elements in the collection.
func (n RootDirSorter) Len() int {
	return len(n)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (n RootDirSorter) Less(i, j int) bool {
	return n[i].RootDir < n[j].RootDir
}

// Swap swaps the elements with indexes i and j.
func (n RootDirSorter) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// -- ScrapingService implementation -----------------------------------------------------------------------------------

type scrapingService struct {
	logger          *logrus.Entry
	scraperInstance *scraper.Scraper
}

// NewScrapingService creates a new scraping service instance using the provided scraper and logger
func NewScrapingService(scr *scraper.Scraper, logger *logrus.Entry) ScrapingService {
	return &scrapingService{
		logger:          logger,
		scraperInstance: scr,
	}
}

// ListDirs returns a list of child directories, the selected directory has
func (s *scrapingService) ListDirs(ctx context.Context, parentDir string) ([]string, error) {
	fileInfos, err := ioutil.ReadDir(parentDir)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return nil, MakeError(
				http.StatusNotFound,
				ErrCodeDirNotFound,
				"Parent directory does not exist or entry is not allowed",
			)
		}
		return nil, err
	}
	var ret []string
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() && !strings.HasPrefix(fileInfo.Name(), ".") {
			ret = append(ret, fileInfo.Name())
		}
	}
	return ret, nil
}

// ListScrapes returns a list of scrapes that are currently available (running, finished, etc) in the scraper
func (s *scrapingService) ListScrapes(ctx context.Context) ([]scraper.Scrape, error) {
	list := s.scraperInstance.StatusAll()
	sorter := RootDirSorter(list)
	sort.Sort(sorter)
	// Sort by
	return sorter, nil
}

// GetScrape returns the scrape that has been started using the given root directory
func (s *scrapingService) GetScrape(ctx context.Context, rootDir string) *scraper.Scrape {
	return s.scraperInstance.Status(rootDir)
}

// Start starts a new scrape inside the scraper
func (s *scrapingService) Start(ctx context.Context, rootDir string) error {
	err := s.scraperInstance.Start(rootDir)
	if err != nil && err == scraper.ErrAlreadyQueued {
		return MakeError(http.StatusConflict, ErrCodeScrapeRunning, "A scrape for this directory is already running")
	}
	return err
}
