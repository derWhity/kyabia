// Package sqlite contains a repository for playlists that stores its data inside a SQLite database
package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/derWhity/kyabia/internal/log"
	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	playlistFields = `name, status, message, createdAt, updatedAt`
	playlistSelect = `SELECT
						pl.id as id,
						pl.name as name,
						pl.createdAt as createdAt,
						pl.updatedAt as updatedAt,
						pl.status as status,
						pl.message as message,
						ifnull(ev.id, 0) AS eventId,
						ifnull(ev.name, '') AS eventName
					FROM
						Playlists pl
					LEFT OUTER JOIN
						Events ev
					ON
						ev.defaultPlaylist = pl.id`
	playlistEntryFields      = `videoHash, position, requestedBy, requesterIp, createdAt, updatedAt`
	playlistReorderFields    = `id, playlistId`
	fullPlaylistEntryFields  = `id, playlistId, position, videoHash, requestedBy, requesterIp, createdAt, updatedAt`
	playlistVideoEntryFields = `id, videoHash, requestedBy, createdAt, updatedAt`
	videoFields              = `sha512, title, artist, language, relatedMedium, mediumDetail, description, duration, identifier`
)

// entryMoveHelper is a data model used when moving playlist entries from one position in a playlist's order to another
type reorderHelper struct {
	EntryID    uint `db:"id"`
	PlaylistID uint `db:"playlistId"`
}

// Helper struct to get the count of things
type countHelper struct {
	Count uint `db:"count"`
}

// PlaylistRepo is a playlist repository that stores its data inside a SQLite database
type PlaylistRepo struct {
	db     *sqlx.DB
	logger *logrus.Entry
}

// New creates a new PlaylistRepo instance with the given DB and logger instances
func New(db *sqlx.DB, logger *logrus.Entry) repos.PlaylistRepo {
	return &PlaylistRepo{db, logger}
}

// -- Methods ----------------------------------------------------------------------------------------------------------

// Create creates a new playlist and updates the
func (r *PlaylistRepo) Create(pl *models.Playlist) error {
	r.logger.WithField("name", pl.Name).Debug("Adding new playlist")
	query := fmt.Sprintf("INSERT INTO Playlists(%s) VALUES(?, ?, ?, datetime('now'), datetime('now'))", playlistFields)
	res, err := r.db.Exec(query, pl.Name, pl.Status, pl.Message)
	if err != nil {
		return err
	}
	// Setting the dates like this should be enough for now
	pl.CreatedAt = time.Now()
	pl.UpdatedAt = time.Now()
	var id int64
	if id, err = res.LastInsertId(); err == nil {
		pl.ID = uint(id)
	}
	return err
}

// Update updates a playlist's base data (not the entries)
func (r *PlaylistRepo) Update(pl *models.Playlist) error {
	r.logger.WithField(log.FldID, pl.ID).Debug("Updating playlist")
	query := "UPDATE Playlists SET name = ?, status = ?, message = ?, updatedAt = datetime('now') WHERE id = ?"
	res, err := r.db.Exec(query, pl.Name, pl.Status, pl.Message, pl.ID)
	if err != nil {
		return err
	}
	pl.UpdatedAt = time.Now()
	var num int64
	if num, err = res.RowsAffected(); err == nil {
		if num == 0 {
			return repos.ErrEntityNotExisting
		}
	}
	return err
}

// Delete removes an existing playlist
func (r *PlaylistRepo) Delete(id uint) error {
	r.logger.WithField(log.FldID, id).Debug("Deleting playlist")
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("Delete: Failed to start transaction: %v", err)
	}
	query := "DELETE FROM Playlists WHERE id = ?"
	res, err := tx.Exec(query, id)
	if err != nil {
		return repos.DoRollback(tx, err)
	}
	var num int64
	if num, err = res.RowsAffected(); err == nil {
		if num == 0 {
			return repos.DoRollback(tx, repos.ErrEntityNotExisting)
		}
	}
	// Remove all those playlist entries belonging to the deleted playlist
	query = "DELETE FROM PlaylistEntries WHERE playlistId = ?"
	if _, err = tx.Exec(query, id); err != nil {
		return repos.DoRollback(tx, fmt.Errorf("Delete: Failed to remove playlist entries: %v", err))
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("Delete: Failed to commit transaction: %v", err)
	}
	return nil
}

// GetByID returns the playlist with the given ID
func (r *PlaylistRepo) GetByID(id uint) (*models.Playlist, error) {
	r.logger.WithField(log.FldID, id).Debug("Loading playlist")
	query := fmt.Sprintf("%s WHERE pl.id = ?", playlistSelect)
	var pl models.Playlist
	err := r.db.Get(&pl, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			// Nothing found
			return nil, repos.ErrEntityNotExisting
		}
		return nil, err
	}
	return &pl, nil
}

// Find searches for playlists matching the given search string - supports pagination
func (r *PlaylistRepo) Find(search string, offset uint, limit uint) ([]models.Playlist, uint, error) {
	if limit == 0 {
		limit = 50
	}
	r.logger.WithFields(logrus.Fields{
		log.FldSearch: search,
		log.FldOffset: offset,
		log.FldLimit:  limit,
	}).Debug("Searching for playlist")
	// For now, we're using a simple LIKE search
	search = "%" + search + "%"
	query := fmt.Sprintf(`%s WHERE
        pl.name LIKE $1
        LIMIT $2 OFFSET $3`, playlistSelect)
	var ret []models.Playlist
	err := r.db.Select(&ret, query, search, limit, offset)
	if err != nil {
		r.logger.WithError(err).Error("Failed to query playlists")
		return nil, 0, err
	}
	// Query the full count
	query = `SELECT COUNT(*) FROM Playlists WHERE name = ?`
	var numRows uint
	if err = r.db.Get(&numRows, query, search); err != nil {
		return nil, 0, err
	}
	return ret, numRows, nil
}

// AddEntry adds an entry to an existing playlist
func (r *PlaylistRepo) AddEntry(playlistID uint, entry *models.PlaylistEntry) error {
	query := fmt.Sprintf(
		"INSERT INTO PlaylistEntries(playlistId, %s) VALUES(?, ?, -1, ?, ?, datetime('now'), datetime('now'))",
		playlistEntryFields,
	)
	res, err := r.db.Exec(query, playlistID, entry.VideoHash, entry.RequestedBy, entry.RequesterIP)
	if err != nil {
		return fmt.Errorf("AddEntry: Failed to create entry: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("AddEntry: Failed to retrieve last insert ID: %v", err)
	}
	entry.ID = uint(id)
	// Set the position of all unsorted playlist entries to their ID - this way they should be the last entry in their
	// list
	query = "UPDATE PlaylistEntries SET position = id WHERE position < 0"
	if _, err = r.db.Exec(query); err != nil {
		return fmt.Errorf("AddEntry: Failed to reposition playlist entries: %v", err)
	}
	return nil
}

// GetEntryByID loads the playlist entry with the given ID from the database
func (r *PlaylistRepo) GetEntryByID(entryID uint) (*models.PlaylistEntry, error) {
	r.logger.WithField(log.FldID, entryID).Debug("Loading playlist entry")
	query := fmt.Sprintf(`SELECT %s FROM PlaylistEntries WHERE id = ?`, fullPlaylistEntryFields)
	var entry models.PlaylistEntry
	err := r.db.Get(&entry, query, entryID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Nothing found
			return nil, repos.ErrEntityNotExisting
		}
		return nil, err
	}
	return &entry, nil
}

// RemoveEntry removes an entry from an existing playlist
func (r *PlaylistRepo) RemoveEntry(entryID uint) error {
	r.logger.WithField(log.FldID, entryID).Debug("Deleting playlist entry")
	query := "DELETE FROM PlaylistEntries WHERE id = ?"
	res, err := r.db.Exec(query, entryID)
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

// UpdateEntry updates an entry - mainly used for internal updating
func (r *PlaylistRepo) UpdateEntry(entry *models.PlaylistEntry) error {
	r.logger.WithField(log.FldID, entry.ID).Debug("Updating playlist entry")
	query := `UPDATE
				PlaylistEntries
			SET
				playlistId = ?,
				videoHash = ?,
				requestedBy = ?,
				updatedAt = datetime('now')
			WHERE id = ?`
	res, err := r.db.Exec(query, entry.PlaylistID, entry.VideoHash, entry.RequestedBy, entry.ID)
	if err != nil {
		return fmt.Errorf("UpdateEntry: Failed to update entry in database: %v", err)
	}
	if num, _ := res.RowsAffected(); num == 0 {
		return repos.ErrEntityNotExisting
	}
	return nil
}

// GetEntryCountByVideo returns the number of playlist entries in the given playlist having the given video selected
func (r *PlaylistRepo) GetEntryCountByVideo(playlistID uint, videoHash string) (uint, error) {
	query := `SELECT COUNT(*) as count FROM PlaylistEntries WHERE playlistId = ? AND videoHash = ?`
	var c countHelper
	err := r.db.Get(&c, query, playlistID, videoHash)
	if err != nil {
		return 0, errors.Wrap(err, "GetEntryCountByIP: Failed to query database")
	}
	return c.Count, nil
}

// GetEntryCountByIP returns the number of playlist entries in the given playlist added by the given IP address
func (r *PlaylistRepo) GetEntryCountByIP(playlistID uint, ipAddr string) (uint, error) {
	query := `SELECT COUNT(*) as count FROM PlaylistEntries WHERE playlistId = ? AND requesterIp = ?`
	var c countHelper
	err := r.db.Get(&c, query, playlistID, ipAddr)
	if err != nil {
		return 0, errors.Wrap(err, "GetEntryCountByIP: Failed to query database")
	}
	return c.Count, nil
}

// GetEntries returns the entries for the given playlist and the number of entries for the full result - supports
// pagination
func (r *PlaylistRepo) GetEntries(playlistID uint, offset uint, limit uint) ([]models.PlaylistVideoEntry, uint, error) {
	if limit == 0 {
		limit = 100
	}
	r.logger.WithFields(logrus.Fields{
		"playlist":    playlistID,
		log.FldOffset: offset,
		log.FldLimit:  limit,
	}).Debug("Listing playlist entries")
	query := fmt.Sprintf("SELECT %s FROM PlaylistEntries WHERE playlistId = ? ORDER BY position, id LIMIT ? OFFSET ?", playlistVideoEntryFields)
	var lst []models.PlaylistVideoEntry
	err := r.db.Select(&lst, query, playlistID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	// Load the video details
	shaMap := map[string]bool{}
	for _, ple := range lst {
		shaMap[ple.VideoHash] = true
	}
	if len(shaMap) > 0 {
		query := fmt.Sprintf(
			"SELECT %s FROM Videos WHERE sha512 IN (?%s)",
			videoFields, strings.Repeat(", ?", len(shaMap)-1),
		)
		params := []interface{}{}
		for sha := range shaMap {
			params = append(params, sha)
		}
		rows, err := r.db.Queryx(query, params...)
		if err != nil {
			return nil, 0, err
		}
		vidMap := map[string]models.VideoSummary{}
		for rows.Next() {
			var vid models.VideoSummary
			if err = rows.StructScan(&vid); err != nil {
				return nil, 0, err
			}
			vidMap[vid.SHA512] = vid
		}
		// Write the video details to the structs
		for i, plve := range lst {
			if vid, ok := vidMap[plve.VideoHash]; ok {
				lst[i].Video = &vid
			}
		}
	}
	// Query the full count
	query = `SELECT COUNT(*) FROM PlaylistEntries WHERE playlistId = ?`
	var numRows uint
	if err = r.db.Get(&numRows, query, playlistID); err != nil {
		return nil, 0, err
	}
	return lst, numRows, nil
}

// PlaceEntryBefore takes the playlist entry with the given ID and moves its position in the playlist to just before
// the other entry provided
// It otherEntryID is set to a value <= 0 or if the other entry is not found in the playlist of the first enty, the
// entry will be placed at the end of the playlist
func (r *PlaylistRepo) PlaceEntryBefore(entryID uint, otherEntryID uint) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("PlaceEntryBefore: Unable to start transaction: %v", err)
	}
	// Load the entry itself
	query := fmt.Sprintf(`SELECT %s FROM PlaylistEntries WHERE id = ?`, playlistReorderFields)
	entry := &reorderHelper{}
	err = tx.Get(entry, query, entryID)
	if err != nil {
		if err == sql.ErrNoRows {
			return repos.DoRollback(tx, repos.ErrEntityNotExisting)
		}
		return repos.DoRollback(tx, fmt.Errorf("PlaceEntryBefore: Failed to load playlist entry to reorder: %v", err))
	}
	// Load all the other entries from the same playlist
	query = fmt.Sprintf(
		`SELECT %s FROM PlaylistEntries WHERE playlistId = ? AND id <> ? ORDER BY position`,
		playlistReorderFields,
	)
	rest := []*reorderHelper{}
	err = tx.Select(&rest, query, entry.PlaylistID, entryID)
	if err != nil {
		return repos.DoRollback(tx, fmt.Errorf("PlaceEntryBefore: Failed to load playlist entries: %v", err))
	}
	// Do some reordering
	found := false
	newOrder := []*reorderHelper{}
	for _, e := range rest {
		if e.EntryID == otherEntryID {
			found = true
			newOrder = append(newOrder, entry)
		}
		newOrder = append(newOrder, e)
	}
	// Place at the end?
	if !found {
		newOrder = append(newOrder, entry)
	}
	// Write the newly ordered items back to the database
	for i, e := range newOrder {
		// ToDo: Find a more performant way to do this
		query := `UPDATE PlaylistEntries SET position = ? WHERE id = ?`
		if _, err := tx.Exec(query, i+1, e.EntryID); err != nil {
			return repos.DoRollback(tx, fmt.Errorf("PlaceEntryBefore: Failed to write new playlist position: %v", err))
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("PlaceEntryBefore: Failed to commit transaction: %v", err)
	}
	return nil
}
