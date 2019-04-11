package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// EventService provides service functions for working with events
type EventService interface {
	List(ctx context.Context, search *Search) ([]models.Event, uint, error)
	Get(ctx context.Context, id uint) (*models.Event, error)
	Create(ctx context.Context, event *models.Event) (*models.Event, error)
	Update(ctx context.Context, event *models.Event) error
	Delete(ctx context.Context, id uint) error
	SetCurrentEvent(ctx context.Context, id uint) error
	CurrentEvent(ctx context.Context) (*models.Event, error)
	DefaultPlaylistID(ctx context.Context) uint
}

// -- EventService implementation --------------------------------------------------------------------------------------

// EventService implementation
type eventService struct {
	repo              repos.EventRepo
	playlistRepo      repos.PlaylistRepo
	logger            *logrus.Entry
	currentEventID    uint
	defaultPlaylistID uint
}

// NewEventService creates a new event service instance
func NewEventService(repo repos.EventRepo, playlists repos.PlaylistRepo, logger *logrus.Entry) EventService {
	return &eventService{
		repo:         repo,
		playlistRepo: playlists,
		logger:       logger,
	}
}

// SetCurrentEvent sets the event currently active to the event with the given ID
func (s *eventService) SetCurrentEvent(ctx context.Context, id uint) error {
	// Check if the event exists
	ev, err := s.repo.GetByID(id)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return MakeError(http.StatusNotFound, ErrCodeEventNotFound,
				fmt.Sprintf("Event #%d does not exist", id),
			)
		}
		return MakeErrorWithData(http.StatusInternalServerError, ErrCodeRepoError,
			fmt.Sprintf("Error while retrieving event #%d", id), err,
		)
	}
	s.currentEventID = id
	s.defaultPlaylistID = ev.MainPlaylistID
	return nil
}

// CurrentEvent returns the event which is currently active
func (s *eventService) CurrentEvent(ctx context.Context) (*models.Event, error) {
	if s.currentEventID == 0 {
		return nil, ErrNoCurrentEvent
	}
	return s.Get(ctx, s.currentEventID)
}

// DefaultPlaylistID returns the ID of the currently active playlist
func (s *eventService) DefaultPlaylistID(_ context.Context) uint {
	return s.defaultPlaylistID
}

// List searches for events matching the given search term
func (s *eventService) List(ctx context.Context, search *Search) ([]models.Event, uint, error) {
	lists, numRows, err := s.repo.Find(search.Search, search.Offset, search.Limit)
	if err != nil {
		return nil, 0, MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while searching events"),
			err,
		)
	}
	return lists, numRows, nil
}

// Get returns the event with the given ID
func (s *eventService) Get(ctx context.Context, id uint) (*models.Event, error) {
	ev, err := s.repo.GetByID(id)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return nil, MakeError(http.StatusNotFound, ErrCodeEventNotFound,
				fmt.Sprintf("Event #%d does not exist", id),
			)
		}
		return nil, MakeErrorWithData(http.StatusInternalServerError, ErrCodeRepoError,
			fmt.Sprintf("Error while retrieving event #%d", id), err,
		)
	}
	return ev, nil
}

// Create creates a new event (and, optionally, a new default playlist)
func (s *eventService) Create(ctx context.Context, event *models.Event) (*models.Event, error) {
	event.Name = strings.TrimSpace(event.Name)
	if event.Name == "" {
		return nil, MakeErrorWithData(
			http.StatusBadRequest,
			ErrCodeRequiredFieldMissing,
			"Event name missing",
			map[string]string{
				"field": "name",
			},
		)
	}
	if event.MainPlaylistID == 0 {
		// Create a new playlist
		pl := models.Playlist{
			Name: event.Name,
		}
		err := s.playlistRepo.Create(&pl)
		if err != nil {
			return nil, fmt.Errorf("Create: Failed to auto-create playlist for new event: %v", err)
		}
		event.MainPlaylistID = pl.ID
	} else if err := s.checkPlaylist(event.MainPlaylistID); err != nil {
		return nil, err
	}
	err := s.repo.Create(event)
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *eventService) checkPlaylist(id uint) error {
	if _, err := s.playlistRepo.GetByID(id); err != nil {
		if err == repos.ErrEntityNotExisting {
			return MakeError(
				http.StatusNotFound,
				ErrCodePlaylistNotFound,
				fmt.Sprintf("Referenced playlist #%d does not exist", id),
			)
		}
		return MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while retrieving playlist #%d", id),
			err,
		)
	}
	return nil
}

// Update updates an existing event
func (s *eventService) Update(ctx context.Context, event *models.Event) error {
	originalEvent, err := s.Get(ctx, event.ID)
	if err != nil {
		return err
	}
	event.Name = strings.TrimSpace(event.Name)
	if event.Name != "" {
		originalEvent.Name = event.Name
	}
	originalEvent.Description = event.Description
	if event.MainPlaylistID > 0 {
		// Check if the playlist exists
		if err := s.checkPlaylist(event.MainPlaylistID); err != nil {
			return err
		}
		originalEvent.MainPlaylistID = event.MainPlaylistID
	}
	originalEvent.StartsAt = event.StartsAt
	originalEvent.EndsAt = event.EndsAt
	err = s.repo.Update(originalEvent)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return MakeError(
				http.StatusNotFound,
				ErrCodeEventNotFound,
				fmt.Sprintf("Event #%d does not exist", event.ID),
			)
		}
		return MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while upating event #%d", event.ID),
			err,
		)
	}
	// Did the default playlist change?
	if originalEvent.ID == s.currentEventID && originalEvent.MainPlaylistID != s.defaultPlaylistID {
		s.defaultPlaylistID = originalEvent.MainPlaylistID
	}
	return nil
}

// Delete removes an existing event from the repository
func (s *eventService) Delete(ctx context.Context, id uint) error {
	err := s.repo.Delete(id)
	if err == repos.ErrEntityNotExisting {
		return MakeError(
			http.StatusNotFound,
			ErrCodePlaylistNotFound,
			fmt.Sprintf("Event #%d does not exist", id),
		)
	}
	if id == s.currentEventID {
		// Now, we don't have a current event any more
		s.currentEventID = 0
		s.defaultPlaylistID = 0
	}
	return nil
}
