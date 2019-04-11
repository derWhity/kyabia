package models

import (
	"time"
)

// Session contains data about an active API session
type Session struct {
	// The session ID (the API key that identifies this session)
	ID string
	// The ID of the user that has logged-in for this session
	UserID uint
	// When will the session expire?
	ExpiresAt time.Time
}

// Expired checks if the session has already expired
func (s *Session) Expired() bool {
	return s.ExpiresAt.Before(time.Now())
}

// UserCan checks if the user in this session has the given permission
//
// Note: Currently this function will always return true, since the permission system is not yet implemented
func (s *Session) UserCan(permission string) bool {
	return true
}
