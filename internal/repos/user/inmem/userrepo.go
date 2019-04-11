// Package inmem provides a user repository that works from memory.
package inmem

import (
	"fmt"

	"strings"

	"github.com/derWhity/kyabia/internal/models"
)

// UserRepo provides a simple in-memory user storage
type UserRepo struct {
	users map[uint]models.User
	// The maximum user ID currently in the storage
	maxUserID uint
}

// New creates a new user repository instance
func New() *UserRepo {
	return &UserRepo{
		users: make(map[uint]models.User),
	}
}

// Create creates a new user
func (r *UserRepo) Create(u *models.User) error {
	if u.ID > 0 {
		existing, err := r.GetByID(u.ID)
		if err != nil {
			return err
		}
		if existing != nil {
			return fmt.Errorf("Create: A user with the given ID does already exist")
		}
	} else {
		// ID is 0 - assign a new one
		u.ID = r.maxUserID + 1
	}
	if r.maxUserID < u.ID {
		// We have a new highest user ID
		r.maxUserID = u.ID
	}
	r.users[u.ID] = *u
	return nil
}

// Update updates an existing user
func (r *UserRepo) Update(u *models.User) error {
	existing, err := r.GetByID(u.ID)
	if err != nil {
		return fmt.Errorf("Update: Error retrieving original user: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("Update: Cannot update non-existing user")
	}
	r.users[u.ID] = *u
	return nil
}

// Delete removes an existing user from the user storage
func (r *UserRepo) Delete(id uint) error {
	var existing *models.User
	var err error
	if existing, err = r.GetByID(id); err != nil {
		return fmt.Errorf("Delete: Cannot get user: %v", err)
	}
	if existing != nil {
		delete(r.users, id)
	}
	return nil
}

// GetByID returns the user with the given ID
func (r *UserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := r.users[id]; ok {
		// Copy the user
		ret := u
		return &ret, nil
	}
	return nil, nil
}

// GetByCredentials returns the user which has the given username and password - this is used for login
func (r *UserRepo) GetByCredentials(username string, password string) (*models.User, error) {
	for _, u := range r.users {
		if u.Name == username && u.CheckPassword(password) == nil {
			ret := u // copy
			return &ret, nil
		}
	}
	return nil, nil
}

// Find searches for users matching the given search string - supports pagination
func (r *UserRepo) Find(search string, offset uint, limit uint) ([]*models.User, error) {
	var ret []*models.User
	for _, u := range r.users {
		if strings.Contains(u.Name, search) || strings.Contains(u.FullName, search) {
			copy := u
			ret = append(ret, &copy)
		}
	}
	return ret, nil
}
