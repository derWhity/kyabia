package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/derWhity/kyabia/internal/ctxhelper"
	"github.com/derWhity/kyabia/internal/log"
	"github.com/derWhity/kyabia/internal/models"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/kardianos/osext"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	apiBasePath = "/api"
)

// Defines an error that defines the HTTP status that should be returned
type httpStatuser interface {
	Status() int
}

// Defines an error that returns a machine-readable error code
type errorCoder interface {
	ErrorCode() string
}

// Defines an error that contains a data field with additional information
type dataBearer interface {
	Data() interface{}
}

type errorResponse struct {
	basicResponse
	// The error code
	Error   string      `json:"error"`
	Message string      `json:"errorMessage"`
	Details interface{} `json:"errorDetails,omitempty"`
}

// MakeHTTPHandler creates the main HTTP handler for the Kyabia service
func MakeHTTPHandler(
	ss ScrapingService,
	vs VideoService,
	ps PlaylistService,
	es EventService,
	sServ SessionService,
	cs ConfigService,
	logger *logrus.Entry,
) http.Handler {
	r := mux.NewRouter()

	options := []httptransport.ServerOption{
		httptransport.ServerErrorEncoder(encodeError),
		httptransport.ServerBefore(makeContextInjector(logger)),
		httptransport.ServerBefore(makeSessionDecoder(sServ)),
	}

	// -- Config service -------------------------------
	{
		configEndpoints := MakeConfigEndpoints(cs)

		// GetWhitelist
		r.Methods(http.MethodGet).Path(apiBasePath + "/config/restrictions/whitelist").Handler(httptransport.NewServer(
			configEndpoints.GetWhitelist,
			decodeNilRequest,
			encodeJSONResponse,
			options...,
		))

		// AddToWhitelist
		r.Methods(http.MethodPost).Path(apiBasePath + "/config/restrictions/whitelist").Handler(httptransport.NewServer(
			configEndpoints.AddToWhitelist,
			decodeIPAddressFromJSONBody,
			encodeJSONResponse,
			options...,
		))

		// RemoveFromWhitelist
		r.Methods(http.MethodDelete).Path(apiBasePath + "/config/restrictions/whitelist/{ipAddress}").Handler(httptransport.NewServer(
			configEndpoints.RemoveFromWhitelist,
			decodeIPAddressFromPath,
			encodeJSONResponse,
			options...,
		))
	}

	// -- Scraping service -----------------------------
	{
		scrapingEndpoints := MakeScrapingEndpoints(ss)

		// ListDirs
		r.Methods(http.MethodGet).Path(apiBasePath + "/dirs{pathName:\\/?.*}").Handler(httptransport.NewServer(
			scrapingEndpoints.ListDirs,
			decodePathName,
			encodeJSONResponse,
			options...,
		))

		// ListScrapes
		r.Methods(http.MethodGet).Path(apiBasePath + "/scrapes").Handler(httptransport.NewServer(
			scrapingEndpoints.ListScrapes,
			decodeNilRequest,
			encodeJSONResponse,
			options...,
		))

		// GetScrape
		r.Methods(http.MethodGet).Path(apiBasePath + "/scrape{pathName:\\/?.*}").Handler(httptransport.NewServer(
			scrapingEndpoints.GetScrape,
			decodePathName,
			encodeJSONResponse,
			options...,
		))

		// Start (scrape)
		r.Methods(http.MethodPost).Path(apiBasePath + "/scrape{pathName:\\/?.*}").Handler(httptransport.NewServer(
			scrapingEndpoints.Start,
			decodePathName,
			encodeJSONResponse,
			options...,
		))
	}

	// -- Video service --------------------------------
	{
		vEp := MakeVideoEndpoints(vs)

		// Find
		r.Methods(http.MethodGet).Path(apiBasePath + "/videos").Handler(httptransport.NewServer(
			vEp.List,
			decodeSearchRequest,
			encodeJSONResponse,
			options...,
		))

		// Get
		r.Methods(http.MethodGet).Path(apiBasePath + "/videos/{id}").Handler(httptransport.NewServer(
			vEp.Get,
			decodeVideoHashFromPath,
			encodeJSONResponse,
			options...,
		))

		// Update
		r.Methods(http.MethodPut).Path(apiBasePath + "/videos/{id}").Handler(httptransport.NewServer(
			vEp.Update,
			decodeVideoUpdateRequest,
			encodeJSONResponse,
			options...,
		))

		// Delete
		r.Methods(http.MethodDelete).Path(apiBasePath + "/videos/{id}").Handler(httptransport.NewServer(
			vEp.Delete,
			decodeVideoHashFromPath,
			encodeJSONResponse,
			options...,
		))
	}

	// -- Playlist service -----------------------------
	{
		plEp := MakePlaylistEndpoints(ps)

		// Create
		r.Methods(http.MethodPost).Path(apiBasePath + "/playlists").Handler(httptransport.NewServer(
			plEp.Create,
			decodePlaylist,
			encodeJSONResponse,
			options...,
		))

		// Get (Read)
		r.Methods(http.MethodGet).Path(apiBasePath + "/playlists/{id:[0-9]+}").Handler(httptransport.NewServer(
			plEp.Get,
			decodeIDFromPath,
			encodeJSONResponse,
			options...,
		))

		// Update
		r.Methods(http.MethodPut).Path(apiBasePath + "/playlists/{id:[0-9]+}").Handler(httptransport.NewServer(
			plEp.Update,
			decodePlaylistUpdate,
			encodeJSONResponse,
			options...,
		))

		// Delete
		r.Methods(http.MethodDelete).Path(apiBasePath + "/playlists/{id:[0-9]+}").Handler(httptransport.NewServer(
			plEp.Delete,
			decodeIDFromPath,
			encodeJSONResponse,
			options...,
		))

		// List
		r.Methods(http.MethodGet).Path(apiBasePath + "/playlists").Handler(httptransport.NewServer(
			plEp.List,
			decodeSearchRequest,
			encodeJSONResponse,
			options...,
		))

		// ListEntries
		r.Methods(http.MethodGet).Path(apiBasePath + "/playlists/{id:[0-9]+}/entries").Handler(httptransport.NewServer(
			plEp.ListEntries,
			decodePlaylistEntryListRequest,
			encodeJSONResponse,
			options...,
		))

		// AddEntry
		r.Methods(http.MethodPost).Path(apiBasePath + "/playlists/{id:[0-9]+}/entries").Handler(httptransport.NewServer(
			plEp.AddEntry,
			decodePlaylistEntry,
			encodeJSONResponse,
			options...,
		))

		// PlaceEntryBefore
		r.Methods(http.MethodPut).Path(apiBasePath + "/playlistEntries/{id:[0-9]+}/before/{otherId:[0-9]+}").Handler(httptransport.NewServer(
			plEp.PlaceEntryBefore,
			decodeReorderRequest,
			encodeJSONResponse,
			options...,
		))

		// UpdateEntry
		r.Methods(http.MethodPut).Path(apiBasePath + "/playlistEntries/{entryId:[0-9]+}").Handler(httptransport.NewServer(
			plEp.UpdateEntry,
			decodePlaylistEntry,
			encodeJSONResponse,
			options...,
		))

		// DeleteEntry
		r.Methods(http.MethodDelete).Path(apiBasePath + "/playlistEntries/{id:[0-9]+}").Handler(httptransport.NewServer(
			plEp.DeleteEntry,
			decodeIDFromPath,
			encodeJSONResponse,
			options...,
		))

		// -- Working with the main playlist

		// GetMain
		r.Methods(http.MethodGet).Path(apiBasePath + "/playlists/main").Handler(httptransport.NewServer(
			plEp.GetMain,
			decodeNilRequest,
			encodeJSONResponse,
			options...,
		))

		// ListMainEntries
		r.Methods(http.MethodGet).Path(apiBasePath + "/playlists/main/entries").Handler(httptransport.NewServer(
			plEp.ListMainEntries,
			decodePaginationRequest,
			encodeJSONResponse,
			options...,
		))

		// AddMainEntry
		r.Methods(http.MethodPost).Path(apiBasePath + "/playlists/main/entries").Handler(httptransport.NewServer(
			plEp.AddMainEntry,
			decodePlaylistEntry,
			encodeJSONResponse,
			options...,
		))

	}

	// -- Event Service --------------------------------
	{
		evEp := MakeEventEndpoints(es)

		// List
		r.Methods(http.MethodGet).Path(apiBasePath + "/events").Handler(httptransport.NewServer(
			evEp.List,
			decodeSearchRequest,
			encodeJSONResponse,
			options...,
		))

		// Get
		r.Methods(http.MethodGet).Path(apiBasePath + "/events/{id:[0-9]+}").Handler(httptransport.NewServer(
			evEp.Get,
			decodeIDFromPath,
			encodeJSONResponse,
			options...,
		))

		// Create
		r.Methods(http.MethodPost).Path(apiBasePath + "/events").Handler(httptransport.NewServer(
			evEp.Create,
			decodeEvent,
			encodeJSONResponse,
			options...,
		))

		// Update
		r.Methods(http.MethodPut).Path(apiBasePath + "/events/{id:[0-9]+}").Handler(httptransport.NewServer(
			evEp.Update,
			decodeEventUpdate,
			encodeJSONResponse,
			options...,
		))

		// Delete
		r.Methods(http.MethodDelete).Path(apiBasePath + "/events/{id:[0-9]+}").Handler(httptransport.NewServer(
			evEp.Delete,
			decodeIDFromPath,
			encodeJSONResponse,
			options...,
		))

		// SetCurrentEvent
		r.Methods(http.MethodPost).Path(apiBasePath + "/events/{id:[0-9]+}/makeCurrent").Handler(httptransport.NewServer(
			evEp.SetCurrentEvent,
			decodeIDFromPath,
			encodeJSONResponse,
			options...,
		))

		// CurrentEvent
		r.Methods(http.MethodGet).Path(apiBasePath + "/events/current").Handler(httptransport.NewServer(
			evEp.CurrentEvent,
			decodeNilRequest,
			encodeJSONResponse,
			options...,
		))
	}

	// -- Session Service ------------------------------
	{
		sEp := MakeSessionEndpoints(sServ)

		// Login
		r.Methods(http.MethodPost).Path(apiBasePath + "/login").Handler(httptransport.NewServer(
			sEp.Login,
			decodeLoginRequest,
			encodeJSONResponse,
			options...,
		))

		// Logout
		r.Methods(http.MethodPost).Path(apiBasePath + "/logout").Handler(httptransport.NewServer(
			sEp.Logout,
			decodeToken,
			encodeJSONResponse,
			options...,
		))

		// WhoAmI
		r.Methods(http.MethodGet).Path(apiBasePath + "/whoami").Handler(httptransport.NewServer(
			sEp.WhoAmI,
			decodeToken,
			encodeJSONResponse,
			options...,
		))
	}

	// Simple alive answer for checking if HTTP can be reached
	r.Methods(http.MethodGet).Path("/alive").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		data := map[string]bool{"ok": true}
		json.NewEncoder(w).Encode(data)
	})

	// Plain file service for the UI serving everything from the "ui" folder right beside the application executable
	execDir, err := osext.ExecutableFolder()
	if err != nil {
		panic(err)
	}
	uiDir := filepath.Join(execDir, "ui")
	r.Methods(http.MethodGet).PathPrefix("/").Handler(http.FileServer(http.Dir(uiDir)))

	return r
}

// decodeNilRequest just does nothing with the request. It is used for endpoints that don't need anything to be passed
func decodeNilRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	return nil, nil
}

// decodeIPAddressfromJSONBody reads an IP address from a provided JSON body
func decodeIPAddressFromJSONBody(_ context.Context, r *http.Request) (interface{}, error) {
	data := map[string]string{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return nil, MakeError(
			http.StatusBadRequest,
			ErrCodeIllegalJSON,
			fmt.Sprintf("Failed to decode JSON body: %v", err),
		)
	}
	ip, ok := data["ip"]
	if !ok {
		return nil, MakeError(
			http.StatusBadRequest,
			ErrCodeIllegalJSON,
			"Missing IP address parameter",
		)
	}
	return ip, nil
}

func decodeIPAddressFromPath(_ context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	str, ok := vars["ipAddress"]
	if !ok {
		return 0, MakeError(http.StatusBadRequest, ErrCodeRequiredFieldMissing, "Missing IP address")
	}
	return str, nil
}

// decodeLoginRequest decodes a login request from the JSON body
func decodeLoginRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req loginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return nil, MakeError(
			http.StatusBadRequest,
			ErrCodeIllegalJSON,
			fmt.Sprintf("Failed to decode JSON body: %v", err),
		)
	}
	return req, nil
}

// decodeToken gets the token from the call's context
func decodeToken(ctx context.Context, r *http.Request) (request interface{}, err error) {
	session := ctxhelper.Session(ctx)
	if session == nil {
		return nil, MakeError(
			http.StatusBadRequest,
			ErrCodeNotLoggedIn,
			"You need an active session for this operation",
		)
	}
	return session.ID, nil
}

// decodePaginationRequest reads the pagination information from the request's query variables
func decodePaginationRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	val := r.URL.Query()
	pag := Pagination{
		Limit: 50,
	}
	if i, err := strconv.ParseUint(val.Get("offset"), 10, 64); err == nil {
		pag.Offset = uint(i)
	}
	if i, err := strconv.ParseUint(val.Get("limit"), 10, 64); err == nil {
		pag.Limit = uint(i)
	}
	return pag, nil
}

// decodeVideoUpdateRequest decodes information of the video to update from the JSON body and gets the video's ID (hash)
// from the path
func decodeVideoUpdateRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	tmp, err := decodeVideoRequest(ctx, r)
	if err != nil {
		return nil, err
	}
	vid := tmp.(models.Video)
	id, err := decodeVideoHashFromPath(ctx, r)
	if err != nil {
		return nil, err
	}
	vid.SHA512 = id.(string)
	return vid, nil
}

// decodeVideoRequest reads information about a video entry from the request's JSON body
func decodeVideoRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var vid models.Video
	err := json.NewDecoder(r.Body).Decode(&vid)
	if err != nil {
		return nil, MakeError(
			http.StatusBadRequest,
			ErrCodeIllegalJSON,
			fmt.Sprintf("Failed to decode JSON body: %v", err),
		)
	}
	return vid, nil
}

// decodePlaylistEntry reads information about a playlist entry from the request's body
func decodePlaylistEntry(ctx context.Context, r *http.Request) (interface{}, error) {
	var en models.PlaylistEntry
	err := json.NewDecoder(r.Body).Decode(&en)
	if err != nil {
		return nil, MakeError(
			http.StatusBadRequest,
			ErrCodeIllegalJSON,
			fmt.Sprintf("Failed to decode JSON body: %v", err),
		)
	}
	// Try to get the playlist ID from the path
	if id, err := getUintFromPath("id", r); err == nil {
		en.PlaylistID = id
	}
	// Try to get the entry ID from the path
	if id, err := getUintFromPath("entryId", r); err == nil {
		en.ID = id
	}
	// Add the IP address of the requester
	if fwdIP := r.Header.Get("X-Forwarded-For"); fwdIP != "" {
		// We have a X-Forwarded-For header that means we're behind a proxy
		en.RequesterIP = fwdIP
	} else {
		// Use the requesting host
		reg := regexp.MustCompile(":[0-9]+$")
		en.RequesterIP = reg.ReplaceAllString(r.RemoteAddr, "")
	}
	return en, nil
}

// Decodes a request for listing the entries of a specific playlist
func decodePlaylistEntryListRequest(ctx context.Context, r *http.Request) (request interface{}, err error) {
	pag, _ := decodePaginationRequest(ctx, r)
	id, err := decodeIDFromPath(ctx, r)
	if err != nil {
		return nil, err
	}
	return playlistEntryListRequest{
		Pagination: pag.(Pagination),
		PlaylistID: id.(uint),
	}, nil
}

// decodeSearchRequest decodes the parameters of a search by checking the GET variables "search", "limit" and "offset"
func decodeSearchRequest(ctx context.Context, r *http.Request) (request interface{}, err error) {
	val := r.URL.Query()
	pag, _ := decodePaginationRequest(ctx, r)
	search := Search{
		Search:     val.Get("search"),
		Pagination: pag.(Pagination),
	}
	return search, nil
}

// decodeDirsRequest decodes the parameters for the ListDirs service call
func decodePathName(_ context.Context, r *http.Request) (request interface{}, err error) {
	vars := mux.Vars(r)
	p, ok := vars["pathName"]
	if !ok {
		return nil, MakeError(http.StatusBadRequest, ErrCodeIllegalPath, "Provided path not valid")
	}
	p = path.Join("/", p)
	return p, nil
}

// decodePlaylist tries to load a playlist object from the provided HTTP request's body
func decodePlaylist(_ context.Context, r *http.Request) (interface{}, error) {
	var pl models.Playlist
	err := json.NewDecoder(r.Body).Decode(&pl)
	if err != nil {
		return nil, MakeError(
			http.StatusBadRequest,
			ErrCodeIllegalJSON,
			fmt.Sprintf("Failed to decode JSON body: %v", err),
		)
	}
	return pl, nil
}

// decodeEvent tries to load an event object from the provided HTTP request's body
func decodeEvent(_ context.Context, r *http.Request) (interface{}, error) {
	var ev models.Event
	err := json.NewDecoder(r.Body).Decode(&ev)
	if err != nil {
		return nil, MakeError(
			http.StatusBadRequest,
			ErrCodeIllegalJSON,
			fmt.Sprintf("Failed to decode JSON body: %v", err),
		)
	}
	return ev, nil
}

// getUintFromPath is a helper function that gets a uint from the given path variable
func getUintFromPath(varname string, r *http.Request) (uint, error) {
	errmsg := fmt.Sprintf("Value for '%s' is no valid unsigned integer", varname)
	vars := mux.Vars(r)
	str, ok := vars[varname]
	if !ok {
		return 0, MakeError(http.StatusBadRequest, ErrCodeInvalidUint, errmsg)
	}
	id, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return 0, MakeError(http.StatusBadRequest, ErrCodeInvalidUint, errmsg)
	}
	return uint(id), nil
}

// decodeReorderRequest loads the IDs needed for a reorder operation from the path variables
func decodeReorderRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	entryID, err := getUintFromPath("id", r)
	if err != nil {
		return nil, err
	}
	otherEntryID, err := getUintFromPath("otherId", r)
	if err != nil {
		return nil, err
	}
	return reorderRequest{Entry: entryID, OtherEntry: otherEntryID}, nil
}

// Decodes an ID from the "id" path variable provided by GoRilla
func decodeIDFromPath(ctx context.Context, r *http.Request) (interface{}, error) {
	return getUintFromPath("id", r)
}

// Decodes the hash of a video entry from the path variable "id"
func decodeVideoHashFromPath(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	str, ok := vars["id"]
	if !ok {
		return nil, MakeError(http.StatusBadRequest, ErrCodeInvalidUint, "No ID provided")
	}
	return str, nil
}

// Decodes a playlist from an update request where the ID of the playlist is in the path
func decodePlaylistUpdate(ctx context.Context, r *http.Request) (interface{}, error) {
	pl, err := decodePlaylist(ctx, r)
	if err != nil {
		return nil, err
	}
	id, err := decodeIDFromPath(ctx, r)
	if err != nil {
		return nil, err
	}
	ret := pl.(models.Playlist)
	ret.ID = id.(uint)
	return ret, nil
}

// Decodes an event from an update request where the ID of the event is in the path
func decodeEventUpdate(ctx context.Context, r *http.Request) (interface{}, error) {
	ev, err := decodeEvent(ctx, r)
	if err != nil {
		return nil, err
	}
	id, err := decodeIDFromPath(ctx, r)
	if err != nil {
		return nil, err
	}
	ret := ev.(models.Event)
	ret.ID = id.(uint)
	return ret, nil
}

// Encodes a typical JSON response
func encodeJSONResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

// Builds an error response based on the incoming error
func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if st, ok := err.(httpStatuser); ok {
		w.WriteHeader(st.Status())
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	ret := errorResponse{
		basicResponse: basicResponse{false, nil},
		Message:       err.Error(),
		Error:         ErrCodeUnknown,
	}
	if cd, ok := err.(errorCoder); ok {
		ret.Error = cd.ErrorCode()
	}
	if db, ok := err.(dataBearer); ok {
		if data := db.Data(); data != nil {
			if err, ok := data.(error); ok {
				ret.Details = err.Error()
			} else {
				ret.Details = data
			}
		}
	}
	json.NewEncoder(w).Encode(&ret)
}

// makeSessionDecoder returns a function that is used in every HTTP call to decode the session used, if a session
// token is sent by the client
func makeSessionDecoder(s SessionService) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		token := strings.TrimSpace(r.Header.Get("token"))
		logger := ctxhelper.Logger(ctx)
		if token != "" {
			// Try to load the session's data
			sess, user, err := s.GetContents(ctx, token, true)
			if err != nil {
				logger.WithError(err).WithField(log.FldSession, token).Error("Failed to retrieve session information")
				return ctx
			}
			if sess == nil || user == nil {
				// Nobody logged in
				return ctx
			}
			ctx = context.WithValue(ctx, ctxhelper.KeySession, *sess)
			ctx = context.WithValue(ctx, ctxhelper.KeyUser, *user)
			ctx = context.WithValue(ctx, ctxhelper.KeyLogger, logger.WithFields(logrus.Fields{
				log.FldSession: sess.ID,
				log.FldUser:    user.ID,
			}))
		}
		return ctx
	}
}

func makeContextInjector(logger *logrus.Entry) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		return context.WithValue(ctx, ctxhelper.KeyLogger, logger)
	}
}
