package models

import "time"

// Event describes a Karaoke event
// The events will be stored in the database for statistical reasons
type Event struct {
	// Internal ID
	ID uint `db:"id" json:"id"`
	// Name of the event
	Name string `db:"name" json:"name"`
	// A little description of the event
	Description string `db:"description" json:"description,omitempty"`
	// The ID of the main playlist which contains the files played on stage
	MainPlaylistID uint `db:"defaultPlaylist" json:"defaultPlaylist"`
	// When does/did the event start?
	StartsAt time.Time `db:"startsAt" json:"startsAt"`
	// When does/did the event end?
	EndsAt time.Time `db:"endsAt" json:"endsAt"`
	// Creation date of this entry
	CreatedAt time.Time `db:"createdAt" json:"createdAt"`
	// Date of the last update of this entry
	UpdatedAt time.Time `db:"updatedAt" json:"updatedAt"`
}
