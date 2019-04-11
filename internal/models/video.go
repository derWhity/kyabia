package models

import "time"

// Dimensions defines a width and a height
type Dimensions struct {
	Width  int `db:"width" json:"width"`
	Height int `db:"height" json:"height"`
}

// Video holds metadata about a video file somewhere in the filesystem
// it contains everything needed for display in the video search site.
type Video struct {
	Dimensions
	VideoSummary
	// The file name of the video file
	Filename string `db:"filename" json:"fileName"`
	// The video format used for encoding this video
	VideoFormat string `db:"videoFormat" json:"videoFormat"`
	// The bitrate of the primary video stream
	VideoBitrate int `db:"videoBitrate" json:"videoBitrate"`
	// The audio format used for encoding this video
	AudioFormat string `db:"audioFormat" json:"audioFormat"`
	// This bitrate of the primary audio stream
	AudioBitrate int `db:"audioBitrate" json:"audioBitrate"`
	// Timestamp of the creation of this metadata record
	CreatedAt time.Time `db:"createdAt" json:"createdAt"`
	// Timestamp of the last change of this metadata record
	UpdatedAt time.Time `db:"updatedAt" json:"updatedAt"`
	// The number of times this video file has been played globally
	NumPlayed uint `db:"numPlayed" json:"numPlayed"`
	// The number of times this video file has been requested by players globally
	NumRequested uint `db:"numRequested" json:"numRequested"`
}

// VideoSummary is a shortened version of the video data type that is used to send to non-admin users hiding some of
// the internal fields
type VideoSummary struct {
	// SHA-512 hash of the video file
	// Used to find duplicates or already scraped video files
	// The hash is also the unique ID for the video
	SHA512 string `db:"sha512" json:"sha512"`
	// Title of the video
	Title string `db:"title" json:"title"`
	// Artist performing in the video
	Artist string `db:"artist" json:"artist"`
	// The language (ISO code) the video is in - good for singers that need some warning before they sing the song ;)
	Language string `db:"language" json:"language"`
	// If this music video is something from a game or a show, the name of this
	// related meduim is stored here
	RelatedMedium string `db:"relatedMedium" json:"relatedMedium"`
	// More detail about the related medium. For example "Opening 1" of the related medium
	MediumDetail string `db:"mediumDetail" json:"mediumDetail"`
	// Further description for the file
	Description string `db:"description" json:"description"`
	// Length of the video file
	Duration time.Duration `db:"duration" json:"duration"`
	// Internal identifier for this file
	Identifier string `db:"identifier" json:"identifier"`
}

// A VideoStatistics entry represents the statistics for one video for a specific event
// Inside this record, all statistical data will be held for later analysis
type VideoStatistics struct {
	// The internal ID
	ID uint
	// The ID of the event associated with this entry
	EventID uint
	// The hash of the video associated with this entry
	VideoHash string
	// Times this video was fully played during this event
	NumPlayed uint
	// Times this video was requested to be played during this event
	NumRequested uint
}
