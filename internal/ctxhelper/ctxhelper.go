// Package ctxhelper provides helper functions for working with the context
package ctxhelper

import (
	"github.com/derWhity/kyabia/internal/models"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var (
	// KeySession is the context key for storing the session associated with the current call
	KeySession = ctxKey("session")
	// KeyUser is the context key for storing the user object associated with the current call
	KeyUser = ctxKey("user")
	// KeyLogger is the context key for storing the logger in the context
	KeyLogger = ctxKey("logger")
)

// internal context key
type ctxKey string

// Session returns the session from the current context, if available
func Session(ctx context.Context) *models.Session {
	if sess, ok := ctx.Value(KeySession).(models.Session); ok {
		return &sess
	}
	return nil
}

// User returns the user from the current context, if available
func User(ctx context.Context) *models.User {
	usr, ok := ctx.Value(KeyUser).(models.User)
	if ok {
		return &usr
	}
	return nil
}

// Logger returns the logger from the current context. If no logger is available, it panics
func Logger(ctx context.Context) *logrus.Entry {
	logger, ok := ctx.Value(KeyLogger).(*logrus.Entry)
	if ok {
		return logger
	}
	panic("No logger in context")
}
