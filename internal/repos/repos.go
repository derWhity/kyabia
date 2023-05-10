// Package repos contains the repository interfaces needed in Kyabia
// It exists to prevent circular dependencies between kyabia and the repo implementations
package repos

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/derWhity/kyabia/internal/models"
)

var (
	// ErrEntityNotExisting is fired by a repository when an entity that is updated or deleted does not exist
	ErrEntityNotExisting = fmt.Errorf("cannot update: Entity does not exist")
)

// VideoRepo defines a repository that handles storing and querying video information
type VideoRepo interface {
	// Create creates a new video entry
	Create(v *models.Video) error
	// Update updates an existing video entry
	Update(v *models.Video) error
	// Delete removes an existing video entry from the storage
	Delete(id string) error
	// GetByID returns the video entry having the given ID
	GetByID(id string) (*models.Video, error)
	// Find searches for videos matching the given search string - supports pagination
	Find(search string, offset uint, limit uint) ([]models.Video, uint, error)
	// BumpNumRequested increases the "numRequested" counter on the given video
	BumpNumRequested(id string) error
}

// UserRepo defines a repository that is able to store, query and authenticate users
type UserRepo interface {
	// Create creates a new user
	Create(u *models.User) error
	// Update updates an existing user
	Update(u *models.User) error
	// Delete removes an existing user from the user storage
	Delete(id uint) error
	// GetByID returns the user with the given ID
	GetByID(id uint) (*models.User, error)
	// GetByCredentials returns the user which has the given username and password - this is used for login
	GetByCredentials(username string, password string) (*models.User, error)
	// Find searches for users matching the given search string - supports pagination
	Find(search string, offset uint, limit uint) ([]*models.User, error)
}

// SessionRepo stores information about active API sessions
type SessionRepo interface {
	// CreateFor creates a new session for the given user ID
	CreateFor(userID uint) (*models.Session, error)
	// GetByID returns the session associated with the given session ID and extends it's expiry if requested
	GetByID(sessionID string, extend bool) (*models.Session, error)
	// Delete removes a session from the session storage
	Delete(sessionID string) error
}

// PlaylistRepo defines a repository that is able to store and query playlists and their contents
type PlaylistRepo interface {
	// Create creates a new playlist
	Create(pl *models.Playlist) error
	// Update updates a playlist's base data (not the entries)
	Update(pl *models.Playlist) error
	// Delete removes an existing playlist
	Delete(id uint) error
	// GetByID returns the playlist with the given ID
	GetByID(id uint) (*models.Playlist, error)
	// Find searches for playlists matching the given search string - supports pagination
	Find(search string, offset uint, limit uint) ([]models.Playlist, uint, error)
	// GetEntryByID loads the playlist entry with the given ID from the database
	GetEntryByID(entryID uint) (*models.PlaylistEntry, error)
	// AddEntry adds an entry to an existing playlist
	AddEntry(playlistID uint, entry *models.PlaylistEntry) error
	// RemoveEntry removes an entry
	RemoveEntry(entryID uint) error
	// UpdateEntry updates an entry - mainly used for internal updating
	UpdateEntry(entry *models.PlaylistEntry) error
	// GetEntries returns the entries for the given playlist - supports pagination
	GetEntries(playlistID uint, offset uint, limit uint) ([]models.PlaylistVideoEntry, uint, error)
	// PlaceEntryBefore reorders the playlist so that the given entry is placed before the other one
	// If the other entry is not found, the entry will be placed at the end of the list
	PlaceEntryBefore(entryID uint, otherEntryID uint) error
	// GetEntryCountByIP returns the number of playlist entries in the given playlist added by the given IP address
	GetEntryCountByIP(playlistID uint, ipAddr string) (uint, error)
	// GetEntryCountByVideo returns the number of playlist entries in the given playlist having the given video selected
	GetEntryCountByVideo(playlistID uint, videoHash string) (uint, error)
}

// EventRepo defines a repository that handles storing and querying events
type EventRepo interface {
	// Create creates a new event
	Create(ev *models.Event) error
	// Update updates the given event
	Update(ev *models.Event) error
	// Delete removes the given event
	Delete(id uint) error
	// GetByID returns the Event with the given ID
	GetByID(id uint) (*models.Event, error)
	// GetByDate returns the event or events that are valid for the given point in time
	GetByDate(date time.Time) ([]models.Event, error)
	// Find searches for events mathing the given search string - supports pagination
	Find(search string, offset uint, limit uint) ([]models.Event, uint, error)
}

// -- Helpers for SQLX repos -------------------------------------------------------------------------------------------

// DoRollback rolls back a transaction and catches any error resulting from it while appending the original error
func DoRollback(tx *sqlx.Tx, originalError error) error {
	if err := tx.Rollback(); err != nil {
		return fmt.Errorf("doRollback: Transaction rollback failed: %v; Recent error: %v", err, originalError)
	}
	return originalError
}
