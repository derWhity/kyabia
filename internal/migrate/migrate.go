// Package migrate handles SQL database migration for the internal Kyabia database
package migrate

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

var migrations []dbMigration

type dbMigration struct {
	Version uint
	Queries []string
}

// Execute runs the current DB migration on the given database
func (mig *dbMigration) Execute(db *sqlx.DB, logger *logrus.Entry) error {
	// Check if the migration has already run
	query := `SELECT success FROM Migrations WHERE version = $1`
	var success = false
	err := db.QueryRow(query, mig.Version).Scan(&success)
	if err != nil {
		switch {
		case err != sql.ErrNoRows:
			logger.WithError(err).Error("Failed to fetch version information")
			return err
		}
	}
	if !success {
		// We need to execute this migration
		logger.Infof("Executing DB migration #%d", mig.Version)
		for i, query := range mig.Queries {
			logger.Infof("Query %d of %d...", (i + 1), len(mig.Queries))
			if _, err := db.Exec(query); err != nil {
				logger.WithError(err).Errorf("Query #%d failed", (i + 1))
				db.Exec(`REPLACE INTO Migrations(version, success) VALUES($1, 0)`, mig.Version)
				return err
			}
		}
		// Queries executed successfully - save our status
		db.Exec(`REPLACE INTO Migrations(version, success) VALUES($1, 1)`, mig.Version)
	}
	return nil
}

// ExecuteMigrationsOnDb executes the database migrations on the given database instance
func ExecuteMigrationsOnDb(db *sqlx.DB, logger *logrus.Entry) error {
	// Create the migrations table if it does not exist, yet
	query := `CREATE TABLE IF NOT EXISTS Migrations (
                version   INTEGER NOT NULL,
                success   INTEGER NOT NULL DEFAULT 0,
                PRIMARY KEY(version)
            )`
	if _, err := db.Exec(query); err != nil {
		logger.WithError(err).Error("Failed to create migrations table")
		return err
	}
	for _, mig := range migrations {
		if err := mig.Execute(db, logger); err != nil {
			logger.WithError(err).Errorf("Failed to execute migration #%d", mig.Version)
			return err
		}
	}
	return nil
}

// For now, the migrations are part of the package...
func init() {
	migrations = []dbMigration{
		{
			Version: 1,
			Queries: []string{
				`CREATE TABLE "Videos" (
                    sha512 VARCHAR(128) NOT NULL PRIMARY KEY,
                    filename VARCHAR(255) NOT NULL,
                    title VARCHAR(128) NOT NULL DEFAULT '',
                    artist VARCHAR(128) NOT NULL DEFAULT '',
                    language VARCHAR(10) NOT NULL DEFAULT '',
                    relatedMedium VARCHAR(128) NOT NULL DEFAULT '',
                    mediumDetail VARCHAR(128) NOT NULL DEFAULT '',
                    description VARCHAR(1024) NOT NULL DEFAULT '',
                    duration INTEGER(8) NOT NULL DEFAULT 0,
                    width INTEGER(4) NOT NULL DEFAULT 0,
                    height INTEGER(4) NOT NULL DEFAULT 0,
                    videoFormat VARCHAR(128) NOT NULL DEFAULT '',
                    videoBitrate INTEGER(8) NOT NULL DEFAULT 0,
                    audioFormat VARCHAR(128) NOT NULL DEFAULT '',
                    audioBitrate INTEGER(8) NOT NULL DEFAULT 0,
                    numPlayed INTEGER(4) NOT NULL DEFAULT 0,
                    numRequested INTEGER(4) NOT NULL DEFAULT 0,
                    createdAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                    updatedAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
                );`,
				`CREATE TABLE "Events" (
                    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                    name VARCHAR(128) NOT NULL DEFAULT '',
                    description TEXT NOT NULL DEFAULT '',
					defaultPlaylist INTEGER NOT NULL DEFAULT 0,
                    startsAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                    endsAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                    createdAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                    updatedAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
                );`,
				`CREATE TABLE "Playlists" (
                    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                    name VARCHAR(128) NOT NULL DEFAULT '',
                    createdAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                    updatedAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
                );`,
				`CREATE TABLE "PlaylistEntries" (
                    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
					playlistId INTEGER NOT NULL,
					position INTEGER NOT NULL DEFAULT 0,
                    videoHash VARCHAR(128) NOT NULL DEFAULT '',
                    requestedBy VARCHAR(128) NOT NULL DEFAULT '',
                    createdAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                    updatedAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
                );`,
				`CREATE TABLE "Users" (
                    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                    name VARCHAR(64) NOT NULL,
                    passwordHash VARCHAR(128) NOT NULL DEFAULT '',
                    fullName VARCHAR(128) NOT NULL DEFAULT '',
                    createdAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
                    updatedAt DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
                );`,
				`CREATE INDEX idx_playlist_name ON Playlists (name ASC);`,
				`CREATE INDEX idx_playlistentry_playlist ON PlaylistEntries (playlistId ASC);`,
			},
		},
		{
			Version: 2,
			Queries: []string{
				`ALTER TABLE Videos ADD COLUMN identifier VARCHAR(32) NOT NULL DEFAULT '';`,
				`CREATE INDEX idx_video_search ON Videos (
					title ASC, artist ASC, relatedMedium ASC, mediumDetail ASC, description ASC, identifier ASC
				);`,
			},
		},
		{
			Version: 3,
			Queries: []string{
				`ALTER TABLE Playlists ADD COLUMN status INTEGER NOT NULL DEFAULT 0;`,
				`ALTER TABLE Playlists ADD COLUMN message VARCHAR(1024) NOT NULL DEFAULT '';`,
			},
		},
		{
			Version: 4,
			Queries: []string{
				`ALTER TABLE PlaylistEntries ADD COLUMN requesterIp VARCHAR(39) NOT NULL DEFAULT '';`,
				`CREATE INDEX idx_playlist_ip_search ON PlaylistEntries (playlistId ASC, requesterIp ASC)`,
			},
		},
		{
			Version: 5,
			Queries: []string{
				`CREATE INDEX idx_playlist_video_search ON PlaylistEntries (playlistId ASC, videoHash ASC)`,
			},
		},
	}
}
