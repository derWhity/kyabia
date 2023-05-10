// Package inmem provides a session repository that holds the session data in-memory
package inmem

import (
	"math/rand"
	"time"

	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
)

const (
	// How long does a session last after the last update?
	expireMinutes = 60
)

// sessionRequest is a generic session request that can be sent over one of the repo's channels to execute functions
// inside the control goroutine
type sessionRequest struct {
	sessionID string
	userID    uint
	extend    bool
	answer    chan<- sessionResponse
}

// sessionResponse is a generic response to a session request that contains the answer to the request made
type sessionResponse struct {
	session *models.Session
	err     error
}

// SessionRepo is a session repository that stores the session data in-memory
type SessionRepo struct {
	// make is a channel to trigger session creation
	make chan<- sessionRequest
	// get is a channel to request a session by ID (and to extend it optionally)
	get chan<- sessionRequest
	// del is a channel to request a session to be deleted
	del chan<- sessionRequest
}

// New creates a new session repository instance
func New() *SessionRepo {
	repo := &SessionRepo{}
	// Spin up the control goroutine
	m := make(chan sessionRequest)
	g := make(chan sessionRequest)
	d := make(chan sessionRequest)
	go repo.control(m, g, d)
	repo.make = m
	repo.get = g
	repo.del = d
	return repo
}

// -- Random string generator from http://stackoverflow.com/questions/22892120 -----------------------------------------

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var randSrc = rand.NewSource(time.Now().UnixNano())

// RandomString creates a random string with the given length
func RandomString(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

// ---------------------------------------------------------------------------------------------------------------------

// control is the control goroutine that runs endlessly waiting for requests for managing sessions
func (r *SessionRepo) control(make <-chan sessionRequest, get <-chan sessionRequest, del <-chan sessionRequest) {
	sessions := map[string]*models.Session{}
	// Purge channel to purge all expired sessions all ~1 minute
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	purge := ticker.C
	for { // To infinity and beyond!
		select {
		case req := <-make:
			// Create a new session
			sessionID := RandomString(64)
			sess := models.Session{
				ID:        sessionID,
				UserID:    req.userID,
				ExpiresAt: time.Now().Add(time.Minute * expireMinutes),
			}
			sessions[sessionID] = &sess
			copy := sess
			req.answer <- sessionResponse{
				session: &copy,
			}
		case req := <-get:
			// Get a session
			sess, ok := sessions[req.sessionID]
			if ok {
				if sess.Expired() {
					// Session expired
					delete(sessions, req.sessionID)
					req.answer <- sessionResponse{err: repos.ErrEntityNotExisting}
				} else {
					if req.extend {
						sess.ExpiresAt = time.Now().Add(time.Minute * expireMinutes)
					}
					copy := *sess
					req.answer <- sessionResponse{session: &copy}
				}
			} else {
				req.answer <- sessionResponse{err: repos.ErrEntityNotExisting}
			}
		case req := <-del:
			// Delete a session
			delete(sessions, req.sessionID)
			req.answer <- sessionResponse{}
		case <-purge:
			// Purge all expired sessions
			var toPurge []string
			for key, sess := range sessions {
				if sess.Expired() {
					toPurge = append(toPurge, key)
				}
			}
			for _, key := range toPurge {
				delete(sessions, key)
			}
		}
	}
}

func send(sessionID string, userID uint, extend bool, channel chan<- sessionRequest) sessionResponse {
	answer := make(chan sessionResponse)
	req := sessionRequest{
		sessionID: sessionID,
		userID:    userID,
		extend:    extend,
		answer:    answer,
	}
	channel <- req
	return <-answer
}

// CreateFor creates a new session for the given user ID
func (r *SessionRepo) CreateFor(userID uint) (*models.Session, error) {
	resp := send("", userID, false, r.make)
	if resp.err != nil {
		return nil, resp.err
	}
	return resp.session, nil
}

// GetByID returns the session associated with the given session ID and extends it's expiry if requested
func (r *SessionRepo) GetByID(sessionID string, extend bool) (*models.Session, error) {
	resp := send(sessionID, 0, extend, r.get)
	if resp.err != nil {
		return nil, resp.err
	}
	return resp.session, nil
}

// Delete removes a session from the session storage
func (r *SessionRepo) Delete(sessionID string) error {
	resp := send(sessionID, 0, false, r.del)
	if resp.err != nil {
		return resp.err
	}
	return nil
}
