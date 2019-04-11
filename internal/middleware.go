package internal

import (
	"net/http"

	"github.com/derWhity/kyabia/internal/ctxhelper"
	"github.com/go-kit/kit/endpoint"
	"golang.org/x/net/context"
)

// EnsureUserLoggedIn is a middleware that checks if there is a valid user session for the current call
func EnsureUserLoggedIn(next endpoint.Endpoint) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		user := ctxhelper.User(ctx)
		if user == nil {
			// Nobody logged in
			return nil, MakeError(
				http.StatusForbidden,
				ErrCodeNotLoggedIn,
				"This function needs a logged-in user",
			)
		}
		return next(ctx, request)
	}
}
