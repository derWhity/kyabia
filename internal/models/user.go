package models

import (
	"fmt"

	"github.com/elithrar/simple-scrypt"
)

const (
	// PermVideoSeeFullDetails is the permission to view all details of any video
	// If the user does not have the permission, only a small portion of a video's properties will be returned
	PermVideoSeeFullDetails = "video.fullDetails"
)

// User defines an (admin?) user of the application and his/her permissions inside this
// application
type User struct {
	// Internal user ID
	ID uint
	// The user name used to log-in
	Name string
	// The hashed password for authentication
	PasswordHash string
	// The full user name for display reasons
	FullName string
	// A list of rights this user has - accessed by functions - for now, all authenticated users are admins
	// rights []string
}

// SetPassword sets a new password creating a password hash from the incoming password and storing it in the user's
// PasswordHash property
func (u *User) SetPassword(pass string) error {
	hash, err := scrypt.GenerateFromPassword([]byte(pass), scrypt.DefaultParams)
	if err != nil {
		return fmt.Errorf("SetPassword: Error during password hashing: %v", err)
	}
	// The library already uses a string encoding here - so there is no need to encode further
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword checks if the given password corresponds to the hash stored in the user struct.
// It returns an error if the password does not match or an error occurs when loading the password hash from the user
func (u *User) CheckPassword(pass string) error {
	return scrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pass))
}
