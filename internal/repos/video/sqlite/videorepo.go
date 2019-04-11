// Package sqlite provides a video repository that uses SQLite for storing video information
package sqlite

import (
	"fmt"

	"database/sql"

	"github.com/derWhity/kyabia/internal/log"
	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

const (
	// The field names in the video table
	fieldNames = `sha512, filename, title, artist, language, relatedMedium, mediumDetail, description, duration,
                    width, height, videoFormat, videoBitrate, audioFormat, audioBitrate, numPlayed, numRequested,
                    createdAt, updatedAt, identifier`
)

// VideoRepo implements kyabia.VideoRepo and provides access to video data stored inside a SQLlite database
type VideoRepo struct {
	logger *logrus.Entry
	db     *sqlx.DB
}

// New creates a new VideoRepo
func New(db *sqlx.DB, logger *logrus.Entry) repos.VideoRepo {
	return &VideoRepo{logger, db}
}

// Create creates a new video entry
func (r *VideoRepo) Create(v *models.Video) error {
	r.logger.WithFields(logrus.Fields{
		"sha512":    v.SHA512,
		log.FldFile: v.Filename,
	}).Debug("Creating video")
	query := fmt.Sprintf(`INSERT INTO Videos(%s) VALUES(
	    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, datetime('now'), datetime('now'), ?
	)`, fieldNames)
	_, err := r.db.Exec(
		query,
		v.SHA512, v.Filename, v.Title, v.Artist, v.Language, v.RelatedMedium, v.MediumDetail, v.Description, v.Duration,
		v.Width, v.Height, v.VideoFormat, v.VideoBitrate, v.AudioFormat, v.AudioBitrate, v.Identifier,
	)
	return err
}

// BumpNumRequested increases the "numRequested" counter on the given video
func (r *VideoRepo) BumpNumRequested(id string) error {
	query := `UPDATE Videos SET numRequested = numRequested+1 WHERE sha512 = ?`
	res, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("BumpNumRequested: Failed to update video entry: %v", err)
	}
	if num, _ := res.RowsAffected(); num == 0 {
		return repos.ErrEntityNotExisting
	}
	return nil
}

// Update updates an existing video entry
func (r *VideoRepo) Update(v *models.Video) error {
	r.logger.WithFields(logrus.Fields{
		"sha512":    v.SHA512,
		log.FldFile: v.Filename,
	}).Debug("Updating video")
	query := `UPDATE Videos SET
        filename= ?, title= ?, artist= ?, language= ?, relatedMedium= ?, mediumDetail= ?, description= ?, duration= ?,
        width= ?, height= ?, videoFormat= ?, videoBitrate= ?, audioFormat= ?, audioBitrate= ?, numPlayed= ?,
        numRequested= ?, updatedAt = datetime('now'), identifier = ?
    WHERE sha512 = ?`
	res, err := r.db.Exec(query,
		v.Filename, v.Title, v.Artist, v.Language, v.RelatedMedium, v.MediumDetail, v.Description, v.Duration, v.Width,
		v.Height, v.VideoFormat, v.VideoBitrate, v.AudioFormat, v.AudioBitrate, v.NumPlayed, v.NumRequested,
		v.Identifier, v.SHA512,
	)
	if err != nil {
		return err
	}
	if num, err := res.RowsAffected(); err != nil || num == 0 {
		if err != nil {
			return fmt.Errorf("Update: Failed to get number of updated rows: %v", err)
		}
		return repos.ErrEntityNotExisting
	}
	return nil
}

// Delete removes an existing video entry from the storage
func (r *VideoRepo) Delete(id string) error {
	r.logger.WithField(log.FldVideo, id).Debug("Deleting video", "sha512", id)
	query := "DELETE FROM Videos WHERE sha512 = ?"
	res, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}
	if num, err := res.RowsAffected(); err != nil || num == 0 {
		if err != nil {
			return err
		}
		return repos.ErrEntityNotExisting
	}
	// Also delete all playlist entries that reference this video
	query = "DELETE FROM PlaylistEntries WHERE videoHash = ?"
	if _, err := r.db.Exec(query, id); err != nil {
		// No need to return an error - but we'll log this
		r.logger.WithField(log.FldVideo, id).WithError(err).Error("Failed to delete playlist entries for deleted video")
	}
	return nil
}

// GetByID returns the video entry having the given ID
func (r *VideoRepo) GetByID(id string) (*models.Video, error) {
	r.logger.WithField(log.FldVideo, id).Debug("Loading video")
	query := fmt.Sprintf("SELECT %s FROM Videos WHERE sha512 = ?", fieldNames)
	var vid models.Video
	err := r.db.Get(&vid, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			// Nothing found
			return nil, repos.ErrEntityNotExisting
		}
		return nil, err
	}
	return &vid, nil
}

// Find searches for videos matching the given search string - supports pagination
// Returned is the requested page of the videos and the number of videos in the full result set
func (r *VideoRepo) Find(search string, offset uint, limit uint) ([]models.Video, uint, error) {
	if limit == 0 {
		limit = 50
	}
	r.logger.WithFields(logrus.Fields{
		log.FldSearch: search,
		log.FldOffset: offset,
		log.FldLimit:  limit,
	}).Debug("Searching for video")
	// For now, we're using a simple LIKE search
	search = "%" + search + "%"
	query := fmt.Sprintf(`SELECT %s FROM Videos WHERE
        title LIKE $1 OR
        artist LIKE $1 OR
        relatedMedium LIKE $1 OR
        mediumDetail LIKE $1 OR
        description LIKE $1 OR
		identifier LIKE $1
		ORDER BY title, artist, relatedMedium, mediumDetail
        LIMIT $2 OFFSET $3
    `, fieldNames)
	var ret []models.Video
	err := r.db.Select(&ret, query, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	// Query the full count
	query = `SELECT COUNT(*) FROM Videos WHERE
		title LIKE $1 OR
        artist LIKE $1 OR
        relatedMedium LIKE $1 OR
        mediumDetail LIKE $1 OR
        description LIKE $1 OR
		identifier LIKE $1`
	var numRows uint
	if err = r.db.Get(&numRows, query, search); err != nil {
		return nil, 0, err
	}
	return ret, numRows, nil
}
