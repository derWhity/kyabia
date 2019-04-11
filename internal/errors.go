package internal

import "net/http"

const (
	// ErrCodeUnknown is the error code for unknown errors
	ErrCodeUnknown = "UNKNOWN_ERROR"
	// ErrCodeIllegalPath is the error that is returned when the client did not send a valid path parameter
	ErrCodeIllegalPath = "ILLEGAL_PATH"
	// ErrCodeDirNotFound is returned when the client requests an operation that has been started with a nonexisting
	// directory
	ErrCodeDirNotFound = "DIR_NOT_FOUND"
	// ErrCodeScrapeRunning is returned when a new scrape is requested that will run in a directory that is already
	// inside the scraping queue
	ErrCodeScrapeRunning = "SCRAPE_ALREADY_QUEUED"
	// ErrCodeRepoError is returned when the request to a repo fails with an error
	ErrCodeRepoError = "STORAGE_QUERY_FAILED"
	// ErrCodeRequiredFieldMissing is returned when at least one required field has not been populated on an incoming
	// request
	ErrCodeRequiredFieldMissing = "REQUIRED_FIELD_MISSING"
	// ErrCodeIllegalJSON is returned when the request did not contain a valid JSON body
	ErrCodeIllegalJSON = "ILLEGAL_JSON_REQUEST"
	// ErrCodeIllegalValue is returned when any field in the transferred data does not validate for some reason
	ErrCodeIllegalValue = "ILLEGAL_VALUE"
	// ErrCodePlaylistNotFound is returned when an operation works on a playlist thas does not exist
	ErrCodePlaylistNotFound = "PLAYLIST_NOT_FOUND"
	// ErrCodePlaylistEntryNotFound is returned when an operation should be executed on a non-existing playlist entry
	ErrCodePlaylistEntryNotFound = "PLAYLIST_ENTRY_NOT_FOUND"
	// ErrCodePlaylistLockedForNewEntries is returned when a playlist is locked for adding new playlist entries
	ErrCodePlaylistLockedForNewEntries = "PLAYLIST_LOCKED_FOR_ADDING"
	// ErrCodeTooManyWishes is returned when an IP address requests more than the allowed number of videos
	ErrCodeTooManyWishes = "TOO_MANY_WISHES"
	// ErrCodeDuplicateWishesNotAllowed is returned when there are no duplicate wishes allowed for the main playlist and
	// a guest tries to add a video that has already been wished for
	ErrCodeDuplicateWishesNotAllowed = "NO_DUPLICATE_WISHES"
	// ErrCodeEventNotFound is returned when an operation works on an event that does not exist
	ErrCodeEventNotFound = "EVENT_NOT_FOUND"
	// ErrCodeInvalidUint is returned when an ID is required inside a request, but is not provided or in a wrong format
	ErrCodeInvalidUint = "INVALID_UINT"
	// ErrCodeNoCurrentEvent is returned when something depending on a currently active event is requested, but no
	// event is currently active
	ErrCodeNoCurrentEvent = "NO_EVENT_SELECTED"
	// ErrCodeVideoNotFound is returned when a referenced video does not exist
	ErrCodeVideoNotFound = "VIDEO_NOT_FOUND"
	// ErrCodeLoginFailed is returned when the user fails to login for some reason
	ErrCodeLoginFailed = "LOGIN_FAILED"
	// ErrCodeNotLoggedIn is returned when the user tried to access an API that needs a logged-in user, but the user
	// has no authenticated session
	ErrCodeNotLoggedIn = "NOT_LOGGED_IN"
)

var (
	// ErrNoCurrentEvent is the default error returned when something requests an operation that depends on an event
	// being selected as current event, while no event has been selected
	ErrNoCurrentEvent = MakeError(
		http.StatusExpectationFailed,
		ErrCodeNoCurrentEvent,
		"No active event selected",
	)
)

// HTTPError is an error that contains information about the error message to return to the client
type HTTPError struct {
	message string
	code    string
	status  int
	data    interface{}
}

// MakeError creates a new HTTPError with the given contents
func MakeError(status int, code, message string) *HTTPError {
	return MakeErrorWithData(status, code, message, nil)
}

// MakeErrorWithData creates a new HTTPError with the given contents and an additional data element
func MakeErrorWithData(status int, code, message string, data interface{}) *HTTPError {
	return &HTTPError{message, code, status, data}
}

// Error implements the errorer interface
func (e *HTTPError) Error() string {
	return e.message
}

// Status returns the HTTP status that should be returned
func (e *HTTPError) Status() int {
	return e.status
}

// ErrorCode returns the machine-readable error code
func (e *HTTPError) ErrorCode() string {
	return e.code
}

// Data returns additional data about the error
func (e *HTTPError) Data() interface{} {
	return e.data
}
