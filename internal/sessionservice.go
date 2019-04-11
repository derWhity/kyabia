package internal

import (
	"net/http"
	"strings"

	"github.com/derWhity/kyabia/internal/models"
	"github.com/derWhity/kyabia/internal/repos"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// SessionService provides functions for interacting with a user's session
type SessionService interface {
	// Login tries to log-in the user with the given credentials and returns the info about the created session if login
	// was successful
	Login(ctx context.Context, user string, password string) (*SessionInfo, error)
	// Logout logs out a currently active session
	Logout(ctx context.Context, sessionID string) error
	// WhoAmI returns information about the current session
	WhoAmI(ctx context.Context, sessionID string) (*SessionInfo, error)
	// GetContents returns the session and user data associated with the given session ID
	// This service function will be used internally and does not have an endpoint
	GetContents(ctx context.Context, sessionID string, extendExpiry bool) (*models.Session, *models.User, error)
}

// -- Session service implementation -----------------------------------------------------------------------------------

// SessionInfo is a session information object that is returned upon login. It contains both, the session ID and
// information about the user that is logged in
type SessionInfo struct {
	SessionID    string `json:"sessionId"`
	UserName     string `json:"userName"`
	UserFullName string `json:"userFullName"`
}

type sessionService struct {
	logger   *logrus.Entry
	sessions repos.SessionRepo
	users    repos.UserRepo
}

// NewSessionService creates a new session service instance with the provided repositories
func NewSessionService(sr repos.SessionRepo, ur repos.UserRepo, logger *logrus.Entry) SessionService {
	return &sessionService{
		logger:   logger,
		sessions: sr,
		users:    ur,
	}
}

// makeSessionInfo creates a session info object from the given session and user data
func makeSessionInfo(sess *models.Session, user *models.User) *SessionInfo {
	return &SessionInfo{
		SessionID:    sess.ID,
		UserName:     user.Name,
		UserFullName: user.FullName,
	}
}

// Login tries to log-in the user with the given credentials and returns the info about the created session if login
// was successful
func (s *sessionService) Login(ctx context.Context, user string, password string) (*SessionInfo, error) {
	user = strings.ToLower(strings.TrimSpace(user))
	u, err := s.users.GetByCredentials(user, password)
	if err != nil {
		s.logger.WithError(err).Error("Failed to load user data for auth")
		return nil, MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to authenticate user",
		)
	}
	if u == nil {
		// Login failed
		return nil, MakeError(
			http.StatusForbidden,
			ErrCodeLoginFailed,
			"Login failed",
		)
	}
	sess, err := s.sessions.CreateFor(u.ID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create session")
		return nil, MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to create session",
		)
	}
	return makeSessionInfo(sess, u), nil
}

// Logout logs out a currently active session
func (s *sessionService) Logout(ctx context.Context, sessionID string) error {
	err := s.sessions.Delete(sessionID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to delete session")
		return MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to logout. Error in the data store",
		)
	}
	return nil
}

// WhoAmI returns information about the current session
func (s *sessionService) WhoAmI(ctx context.Context, sessionID string) (*SessionInfo, error) {
	sess, u, err := s.GetContents(ctx, sessionID, false)
	if err != nil {
		return nil, err
	}
	return makeSessionInfo(sess, u), nil
}

// GetContents returns the session and user data associated with the given session ID
// This service function will be used internally and does not have an endpoint
func (s *sessionService) GetContents(ctx context.Context, sessionID string, extendExpiry bool) (*models.Session, *models.User, error) {
	sess, err := s.sessions.GetByID(sessionID, extendExpiry)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return nil, nil, nil
		}
		s.logger.WithError(err).Error("Failed to retrieve session from repo")
		return nil, nil, MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to retrieve session information from storage",
		)
	}
	u, err := s.users.GetByID(sess.UserID)
	if err != nil {
		if err == repos.ErrEntityNotExisting {
			return nil, nil, nil
		}
		s.logger.WithError(err).Error("Failed to retrieve user data from repo")
		return nil, nil, MakeError(
			http.StatusInternalServerError,
			ErrCodeRepoError,
			"Failed to retrieve user information from storage",
		)
	}
	return sess, u, nil
}
