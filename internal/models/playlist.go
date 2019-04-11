package models

import "time"

const (
	// PlaylistStatusOpen is the status for an open playlist where all actions can be taken
	PlaylistStatusOpen = iota
	// PlaylistStatusClosedForGuest is the status for a playlist that forbids adding entries for guest users
	PlaylistStatusClosedForGuest
)

// A PlaylistEntry describes a video (song) requested to be played
type PlaylistEntry struct {
	// Internal ID of the playlist entry
	ID uint `db:"id" json:"id"`
	// The hash of the video that has been requested
	VideoHash string `db:"videoHash" json:"videoHash"`
	// The position of the entry inside the list
	Position uint `db:"position" json:"-"`
	// Who requested the video? - Users can enter this name freely
	RequestedBy string `db:"requestedBy" json:"requestedBy"`
	// Creation timestamp of the entry == Timestamp of request
	CreatedAt time.Time `db:"createdAt" json:"createdAt"`
	// If updated - timestamp of the last update of the entry
	UpdatedAt time.Time `db:"updatedAt" json:"updatedAt"`
	// The playlist ID the entry belongs to
	PlaylistID uint `db:"playlistId" json:"playlistId,omitempty"`
	// The IP address of the machine this entry was requested from - not to be exported
	RequesterIP string `db:"requesterIp" json:"-"`
}

// A PlaylistVideoEntry contains the data about a playlist entry with additional information about the video referenced
// in it. This variant is used for showing PlaylistEntry data to the user
type PlaylistVideoEntry struct {
	PlaylistEntry
	Video *VideoSummary `json:"video"`
}

// A Playlist is simply a list of video files
type Playlist struct {
	ID uint `db:"id" json:"id"`
	// Name of the playlist
	Name string `db:"name" json:"name"`
	// The current status of this playlist - see "Status"- constants for possible values.
	Status uint `db:"status" json:"status"`
	// A message to display to the users - mainly used for the currently active wishlist
	Message string `db:"message" json:"message"`
	// Creation timestamp of the playlist
	CreatedAt time.Time `db:"createdAt" json:"createdAt"`
	// If updated - timestamp of the last update of the playlist
	UpdatedAt time.Time `db:"updatedAt" json:"updatedAt"`
	// Name of the event that has this playlist as default playlist
	EventName string `db:"eventName" json:"eventName,omitempty"`
	// ID of the event that has this playlist as default playlist
	EventID uint `db:"eventId" json:"eventId,omitempty"`
	// Is this playlist the main playlist?
	IsMain bool `json:"isMain"`
}

// ClosedForGuest checks if the playlist is closed for adding entries by guest users
func (p *Playlist) ClosedForGuest() bool {
	return p.Status == PlaylistStatusClosedForGuest
}

// ValidPlaylistStatus checks if the given value is a valid playlist status
func ValidPlaylistStatus(status uint) bool {
	return status == PlaylistStatusClosedForGuest || status == PlaylistStatusOpen
}
