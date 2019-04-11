// Package sqlite provides an event repository that stores its data inside a SQLite database
package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/derWhity/kyabia/internal/log"
	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
	"github.com/jmoiron/sqlx"
)

const (
	eventFields = `name, description, defaultPlaylist, startsAt, endsAt, createdAt, updatedAt`
)

// EventRepo is an repository that stores its data inside a SQLite database
type EventRepo struct {
	db     *sqlx.DB
	logger *logrus.Entry
}

// New creates a new event repository instance with the given database and logger
func New(db *sqlx.DB, logger *logrus.Entry) *EventRepo {
	return &EventRepo{
		db:     db,
		logger: logger,
	}
}

// Create creates a new event
func (r *EventRepo) Create(ev *models.Event) error {
	r.logger.WithField("name", ev.Name).Debug("Adding new event")
	query := fmt.Sprintf("INSERT INTO Events(%s) VALUES(?, ?, ?, ?, ?, datetime('now'), datetime('now'))", eventFields)
	res, err := r.db.Exec(query, ev.Name, ev.Description, ev.MainPlaylistID, ev.StartsAt, ev.EndsAt)
	if err != nil {
		return err
	}
	// Setting the dates like this should be enough for now
	ev.CreatedAt = time.Now()
	ev.UpdatedAt = time.Now()
	var id int64
	if id, err = res.LastInsertId(); err == nil {
		ev.ID = uint(id)
	}
	return err
}

// Update updates the given event
func (r *EventRepo) Update(ev *models.Event) error {
	r.logger.WithField(log.FldID, ev.ID).Debug("Updating event")
	query := `UPDATE Events SET name = ?, description = ?, defaultPlaylist = ?, startsAt = ?, endsAt = ?, 
        updatedAt = datetime('now') WHERE id = ?`
	res, err := r.db.Exec(query, ev.Name, ev.Description, ev.MainPlaylistID, ev.StartsAt, ev.EndsAt, ev.ID)
	if err != nil {
		return err
	}
	ev.UpdatedAt = time.Now()
	var num int64
	if num, err = res.RowsAffected(); err == nil {
		if num == 0 {
			return repos.ErrEntityNotExisting
		}
	}
	return err
}

// Delete removes the given event
func (r *EventRepo) Delete(id uint) error {
	r.logger.WithField(log.FldID, id).Debug("Deleting event")
	query := "DELETE FROM Events WHERE id = ?"
	res, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}
	var num int64
	if num, err = res.RowsAffected(); err == nil {
		if num == 0 {
			return repos.ErrEntityNotExisting
		}
	}
	return err
}

// GetByID returns the Event with the given ID
func (r *EventRepo) GetByID(id uint) (*models.Event, error) {
	r.logger.WithField(log.FldID, id).Debug("Loading event")
	query := fmt.Sprintf("SELECT id, %s FROM Events WHERE id = ?", eventFields)
	var ev models.Event
	err := r.db.Get(&ev, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			// Nothing found
			return nil, repos.ErrEntityNotExisting
		}
		return nil, err
	}
	return &ev, nil
}

// GetByDate returns the event or events that are valid for the given point in time
func (r *EventRepo) GetByDate(date time.Time) ([]models.Event, error) {
	query := fmt.Sprintf(`SELECT id, %s FROM Events WHERE startsAt <= $1 AND endsAt >= $1 ORDER BY id`, eventFields)
	var ret []models.Event
	err := r.db.Select(&ret, query, date)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Find searches for events mathing the given search string - supports pagination
func (r *EventRepo) Find(search string, offset uint, limit uint) ([]models.Event, uint, error) {
	if limit == 0 {
		limit = 50
	}
	r.logger.WithFields(logrus.Fields{
		log.FldSearch: search,
		log.FldOffset: offset,
		log.FldLimit:  limit,
	}).Debug("Searching for event")
	// For now, we're using a simple LIKE search
	search = "%" + search + "%"
	query := fmt.Sprintf(`SELECT id, %s FROM Events WHERE
        name LIKE $1 OR description LIKE $1
        LIMIT $2 OFFSET $3`, eventFields)
	var ret []models.Event
	err := r.db.Select(&ret, query, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	// Query the full count
	query = `SELECT COUNT(*) FROM Events WHERE name LIKE $1 OR description LIKE $1`
	var numRows uint
	if err = r.db.Get(&numRows, query, search); err != nil {
		return nil, 0, err
	}
	return ret, numRows, nil
}
