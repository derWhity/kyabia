package internal

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/derWhity/kyabia/internal/log"
	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// PlaylistService provides service functions for working with playlists
type PlaylistService interface {
	List(ctx context.Context, search *Search) ([]models.Playlist, uint, error)
	Get(ctx context.Context, id uint) (*models.Playlist, error)
	Create(ctx context.Context, playlist *models.Playlist) (*models.Playlist, error)
	Update(ctx context.Context, playlist *models.Playlist) error
	Delete(ctx context.Context, id uint) error
	ListEntries(ctx context.Context, id uint, offset uint, limit uint) ([]models.PlaylistVideoEntry, uint, error)
	AddEntry(ctx context.Context, id uint, entry *models.PlaylistEntry) error
	UpdateEntry(ctx context.Context, entry models.PlaylistEntry) error
	DeleteEntry(ctx context.Context, id uint) error
	PlaceEntryBefore(ctx context.Context, entryID uint, otherEntryID uint) error
	GetMain(ctx context.Context) (*models.Playlist, error)
	ListMainEntries(ctx context.Context, offset uint, limit uint) ([]models.PlaylistVideoEntry, uint, error)
	AddMainEntry(ctx context.Context, entry *models.PlaylistEntry) error
}

// -- PlaylistService implementation -----------------------------------------------------------------------------------

type playlistService struct {
	logger    *logrus.Entry
	repo      repos.PlaylistRepo
	videoRepo repos.VideoRepo
	events    EventService
	config    ConfigService
}

// NewPlaylistService creates a new PlaylistService instance
func NewPlaylistService(pRepo repos.PlaylistRepo, vRepo repos.VideoRepo, events EventService, cs ConfigService, logger *logrus.Entry) PlaylistService {
	return &playlistService{logger, pRepo, vRepo, events, cs}
}

// List returns a list of playlists matching the search term
func (s *playlistService) List(ctx context.Context, search *Search) ([]models.Playlist, uint, error) {
	lists, numRows, err := s.repo.Find(search.Search, search.Offset, search.Limit)
	if err != nil {
		return nil, 0, MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while searching playlists"),
			err,
		)
	}
	// Set a marker to the main playlist
	mainID := s.events.DefaultPlaylistID(ctx)
	for i, list := range lists {
		if list.ID == mainID {
			lists[i].IsMain = true // There are no pointers here - so we'll do it this way
		}
	}
	return lists, numRows, nil
}

// Get returns the playlist with the given ID
func (s *playlistService) Get(ctx context.Context, id uint) (*models.Playlist, error) {
	pl, err := s.repo.GetByID(id)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return nil, MakeError(
				http.StatusNotFound,
				ErrCodePlaylistNotFound,
				fmt.Sprintf("Playlist #%d does not exist", id),
			)
		}
		return nil, MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while retrieving playlist #%d", id),
			err,
		)
	}
	if s.events.DefaultPlaylistID(ctx) == pl.ID {
		pl.IsMain = true
	}
	return pl, nil
}

// Create creates a new playlist
func (s *playlistService) Create(ctx context.Context, playlist *models.Playlist) (*models.Playlist, error) {
	playlist.Name = strings.TrimSpace(playlist.Name)
	if playlist.Name == "" {
		return nil, MakeErrorWithData(
			http.StatusBadRequest,
			ErrCodeRequiredFieldMissing,
			"Playlist name missing",
			map[string]string{
				"field": "name",
			},
		)
	}
	err := s.repo.Create(playlist)
	if err != nil {
		return nil, err
	}
	return playlist, nil
}

// Update updates an existing playlist
func (s *playlistService) Update(ctx context.Context, playlist *models.Playlist) error {
	originalPlaylist, err := s.Get(ctx, playlist.ID)
	if err != nil {
		return err
	}
	if !models.ValidPlaylistStatus(playlist.Status) {
		return MakeErrorWithData(
			http.StatusBadRequest,
			ErrCodeIllegalValue,
			"Illegal status value",
			map[string]string{
				"value": "status",
			},
		)
	}
	originalPlaylist.Name = strings.TrimSpace(playlist.Name)
	originalPlaylist.Status = playlist.Status
	originalPlaylist.Message = strings.TrimSpace(playlist.Message)
	err = s.repo.Update(originalPlaylist)
	if err != nil {
		return MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while upating playlist #%d", playlist.ID),
			err,
		)
	}
	return nil
}

// Delete removes an existing playlist
func (s *playlistService) Delete(ctx context.Context, id uint) error {
	err := s.repo.Delete(id)
	if err == repos.ErrEntityNotExisting {
		return MakeError(
			http.StatusNotFound,
			ErrCodePlaylistNotFound,
			fmt.Sprintf("Playlist #%d does not exist", id),
		)
	}
	return nil
}

// ListEntries returns the playlist entries belonging to the list with the provided playlist ID
func (s *playlistService) ListEntries(ctx context.Context, id uint, offset uint, limit uint) ([]models.PlaylistVideoEntry, uint, error) {
	// Check if the playlist exists
	_, err := s.Get(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	// All right - get the entries
	list, numRows, err := s.repo.GetEntries(id, offset, limit)
	if err != nil {
		return nil, 0, MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while retrieving playlist entries for #%d", id),
			err,
		)
	}
	return list, numRows, nil
}

// AddEntry adds an entry to the playlist with the playlist ID provided
func (s *playlistService) AddEntry(ctx context.Context, id uint, entry *models.PlaylistEntry) error {
	// Check if the playlist exists
	_, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	entry.RequestedBy = strings.TrimSpace(entry.RequestedBy)
	if entry.RequestedBy == "" {
		return MakeErrorWithData(
			http.StatusBadRequest,
			ErrCodeRequiredFieldMissing,
			"RequestedBy must not be empty",
			map[string]string{
				"field": "requestedBy",
			},
		)
	}
	// Check if the video exists
	_, err = s.videoRepo.GetByID(entry.VideoHash)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return MakeError(
				http.StatusBadRequest,
				ErrCodeVideoNotFound,
				"The requested video does not exist",
			)
		}
		return MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to retrieve video information",
			err,
		)
	}
	if err := s.repo.AddEntry(id, entry); err != nil {
		return MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while adding entry to playlist #%d", id),
			err,
		)
	}
	// NumRequested++
	if err := s.videoRepo.BumpNumRequested(entry.VideoHash); err != nil {
		// Do not report the error back, but log it!
		s.logger.WithError(err).WithField(log.FldVideo, entry.VideoHash).Error("Failed to update request counter for video")
	}
	return nil
}

// UpdateEntry updates the data of the given playlist entry
func (s *playlistService) UpdateEntry(ctx context.Context, entry models.PlaylistEntry) error {
	originalEntry, err := s.repo.GetEntryByID(entry.ID)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return MakeError(
				http.StatusNotFound,
				ErrCodePlaylistEntryNotFound,
				fmt.Sprintf("UpdateEntry: Playlist entry #%d does not exist", entry.ID),
			)
		}
	}
	// Update only the supported fields on the original entry
	// Requester
	if strings.TrimSpace(entry.RequestedBy) != "" {
		originalEntry.RequestedBy = strings.TrimSpace(entry.RequestedBy)
	}
	// Playlist ID
	needsReorder := false
	if entry.PlaylistID > 0 && originalEntry.PlaylistID != entry.PlaylistID {
		_, err = s.repo.GetByID(entry.PlaylistID)
		if err != nil {
			if err == repos.ErrEntityNotExisting {
				return MakeError(
					http.StatusBadRequest,
					ErrCodePlaylistNotFound,
					fmt.Sprintf(
						"UpdateEntry: Cannot update entry to reference non-existing playlist '%d'",
						entry.PlaylistID,
					),
				)
			}
			return MakeErrorWithData(
				http.StatusInternalServerError,
				ErrCodeRepoError,
				"Error while loading playlist data",
				err,
			)
		}
		// Playlist exists - continue
		needsReorder = true
		originalEntry.PlaylistID = entry.PlaylistID
	}
	// Video hash
	if entry.VideoHash != originalEntry.VideoHash {
		_, err = s.videoRepo.GetByID(entry.VideoHash)
		if err != nil {
			if err == repos.ErrEntityNotExisting {
				return MakeError(
					http.StatusBadRequest,
					ErrCodeVideoNotFound,
					fmt.Sprintf(
						"UpdateEntry: Cannot update entry to reference non-existing video '%s'",
						entry.VideoHash,
					),
				)
			}
			return MakeErrorWithData(
				http.StatusInternalServerError,
				ErrCodeRepoError,
				"Error while loading video data",
				err,
			)
		}
		// Video exists - continue
		originalEntry.VideoHash = entry.VideoHash
	}
	// Do the update
	if err := s.repo.UpdateEntry(originalEntry); err != nil {
		return MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Error while updating playlist entry",
			err,
		)
	}
	if needsReorder {
		// Place at the end of the new playlist
		return s.PlaceEntryBefore(ctx, entry.ID, 0)
	}
	return nil
}

// DeleteEntry removes the given playlist entry from the database
func (s *playlistService) DeleteEntry(ctx context.Context, id uint) error {
	if err := s.repo.RemoveEntry(id); err != nil {
		if err == repos.ErrEntityNotExisting {
			return MakeError(
				http.StatusNotFound,
				ErrCodePlaylistEntryNotFound,
				fmt.Sprintf(
					"DeleteEntry: Playlist entry %d does not exist",
					id,
				),
			)
		}
		return MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Error while deleting playlist entry",
			err,
		)
	}
	return nil
}

// PlaceEntryBefore moves an entry inside the playlist's order before another entry
// If the other entry is not found or does not belong to the same playlist, the entry is placed at the end of the
// playlist
func (s *playlistService) PlaceEntryBefore(ctx context.Context, entryID uint, otherEntryID uint) error {
	if err := s.repo.PlaceEntryBefore(entryID, otherEntryID); err != nil {
		if err == repos.ErrEntityNotExisting {
			return MakeError(
				http.StatusNotFound,
				ErrCodePlaylistEntryNotFound,
				fmt.Sprintf("Playlist entry #%d does not exist", entryID),
			)
		}
		return MakeErrorWithData(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			fmt.Sprintf("Error while reordering playlist entries"),
			err,
		)
	}
	return nil
}

// GetMain returns the main playlist for the event that is currently running
func (s *playlistService) GetMain(ctx context.Context) (*models.Playlist, error) {
	mainID := s.events.DefaultPlaylistID(ctx)
	if mainID == 0 {
		return nil, ErrNoCurrentEvent
	}
	pl, err := s.Get(ctx, mainID)
	if err != nil {
		return nil, err
	}
	if pl != nil {
		pl.IsMain = true
	}
	return pl, nil
}

// ListMainEntries returns the playlist entries for the main playlist for the currently active event
func (s *playlistService) ListMainEntries(ctx context.Context, offset uint, limit uint) ([]models.PlaylistVideoEntry, uint, error) {
	mainID := s.events.DefaultPlaylistID(ctx)
	if mainID == 0 {
		return nil, 0, ErrNoCurrentEvent
	}
	return s.ListEntries(ctx, mainID, offset, limit)
}

// AddMainEntry adds a playlist entry to the main playlist for the currently active event
func (s *playlistService) AddMainEntry(ctx context.Context, entry *models.PlaylistEntry) error {
	mainID := s.events.DefaultPlaylistID(ctx)
	if mainID == 0 {
		return ErrNoCurrentEvent
	}
	// Retrieve the playlist and check if it's allowed to add an entry
	pl, err := s.repo.GetByID(mainID)
	if err != nil {
		return err
	}
	if pl.ClosedForGuest() {
		return MakeError(
			http.StatusForbidden,
			ErrCodePlaylistLockedForNewEntries,
			"The playlist is locked for adding new entries",
		)
	}
	conf := s.config.GetConfig(ctx)
	// Check if the video has already been added
	if !conf.Restrictions.AllowDuplicateWishes {
		count, err := s.repo.GetEntryCountByVideo(s.events.DefaultPlaylistID(ctx), entry.VideoHash)
		if err != nil {
			return err
		}
		if count > 0 {
			return MakeError(
				http.StatusForbidden,
				ErrCodeDuplicateWishesNotAllowed,
				"Your desired video is already on the wishlist",
			)
		}
	}

	// Check if the IP can add any more entries
	if !s.config.IsWhitelisted(entry.RequesterIP) {
		count, err := s.repo.GetEntryCountByIP(s.events.DefaultPlaylistID(ctx), entry.RequesterIP)
		if err != nil {
			return err
		}
		if count >= conf.Restrictions.NumWishesFromSameIP {
			return MakeError(
				http.StatusForbidden,
				ErrCodeTooManyWishes,
				"You cannot add another wish, greedy one",
			)
		}
	}

	return s.AddEntry(ctx, mainID, entry)
}
