package internal

import (
	"net/http"

	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// VideoService provides functionality for listing scraped videos
type VideoService interface {
	// List searches for videos matching the provided search and returns a list of paged results
	List(ctx context.Context, search *Search) ([]models.Video, uint, error)
	// Get returns the video with the given ID (SHA-512 hash)
	Get(ctx context.Context, id string) (*models.Video, error)
	// Create will be added later
	// Create(ctx context.Context, video *models.Video) (*models.Video, error)

	// Update updates the given video in the database with the video data provided
	Update(ctx context.Context, video *models.Video) error
	// Delete removes the video with the given ID (SHA-512 hash) from the database
	Delete(ctx context.Context, id string) error
}

// -- VideoService implementation --------------------------------------------------------------------------------------

type videoService struct {
	logger *logrus.Entry
	repo   repos.VideoRepo
}

// NewVideoService creates a new videoService instance to use for creating endpoints
func NewVideoService(vRepo repos.VideoRepo, logger *logrus.Entry) VideoService {
	return &videoService{logger, vRepo}
}

// List searches for videos matching the provided search and returns a list of paged results
func (s *videoService) List(ctx context.Context, search *Search) ([]models.Video, uint, error) {
	vids, numRows, err := s.repo.Find(search.Search, search.Offset, search.Limit)
	if err != nil {
		s.logger.WithError(err).Error("Video list query failed")
		return nil, 0, MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to load video information from storage",
		)
	}
	return vids, numRows, nil
}

// Get returns the video with the given ID (SHA-512 hash)
func (s *videoService) Get(ctx context.Context, id string) (*models.Video, error) {
	vid, err := s.repo.GetByID(id)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return nil, err
		}
		s.logger.WithError(err).Error("Video query failed")
		return nil, MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to load video information from storage",
		)
	}
	return vid, nil
}

// Update updates the given video in the database with the video data provided
func (s *videoService) Update(ctx context.Context, video *models.Video) error {
	vid, err := s.Get(ctx, video.SHA512)
	if err != nil {
		return err
	}
	// Update only the fields that are currently supported
	vid.Artist = video.Artist
	vid.Title = video.Title
	vid.Description = video.Description
	vid.RelatedMedium = video.RelatedMedium
	vid.MediumDetail = video.MediumDetail
	vid.Language = video.Language
	err = s.repo.Update(vid)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return err
		}
		s.logger.WithError(err).Error("Video update failed")
		return MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to write video information to storage",
		)
	}
	return nil
}

// Delete removes the video with the given ID (SHA-512 hash) from the database
func (s *videoService) Delete(ctx context.Context, id string) error {
	err := s.repo.Delete(id)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return err
		}
		s.logger.WithError(err).Error("Video deletion failed")
		return MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to delete video from storage",
		)
	}
	return nil
}
