package internal

import (
	"fmt"

	"github.com/derWhity/kyabia/internal/ctxhelper"
	"github.com/derWhity/kyabia/internal/models"
	"github.com/go-kit/kit/endpoint"
	"golang.org/x/net/context"
)

// ScrapingEndpoints is a collection of endpoints to the scraping service
type ScrapingEndpoints struct {
	ListDirs    endpoint.Endpoint
	ListScrapes endpoint.Endpoint
	GetScrape   endpoint.Endpoint
	Start       endpoint.Endpoint
}

// VideoEndpoints is a collection of endpoints to the video service
type VideoEndpoints struct {
	List   endpoint.Endpoint
	Get    endpoint.Endpoint
	Update endpoint.Endpoint
	Delete endpoint.Endpoint
}

// PlaylistEndpoints is a collection of endpoints for working with the playlist service
type PlaylistEndpoints struct {
	Create           endpoint.Endpoint
	Get              endpoint.Endpoint
	Update           endpoint.Endpoint
	Delete           endpoint.Endpoint
	List             endpoint.Endpoint
	ListEntries      endpoint.Endpoint
	AddEntry         endpoint.Endpoint
	UpdateEntry      endpoint.Endpoint
	DeleteEntry      endpoint.Endpoint
	PlaceEntryBefore endpoint.Endpoint
	GetMain          endpoint.Endpoint
	ListMainEntries  endpoint.Endpoint
	AddMainEntry     endpoint.Endpoint
}

// EventEndpoints is a collection of endpoints for working with the event service
type EventEndpoints struct {
	List              endpoint.Endpoint
	Get               endpoint.Endpoint
	Create            endpoint.Endpoint
	Update            endpoint.Endpoint
	Delete            endpoint.Endpoint
	SetCurrentEvent   endpoint.Endpoint
	CurrentEvent      endpoint.Endpoint
	DefaultPlaylistID endpoint.Endpoint
}

// SessionEndpoints is a collection of endpoints for working with the session service
type SessionEndpoints struct {
	Login  endpoint.Endpoint
	Logout endpoint.Endpoint
	WhoAmI endpoint.Endpoint
}

// ConfigEndpoints is a collection of endpoints for changing the system's configuration
type ConfigEndpoints struct {
	GetWhitelist        endpoint.Endpoint
	AddToWhitelist      endpoint.Endpoint
	RemoveFromWhitelist endpoint.Endpoint
}

// The base for all responses which always contains an "ok" property to show if the call was successful and a
// data element containing the result of the request
type basicResponse struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data,omitempty"`
}

type pagingResponse struct {
	Rows uint        `json:"rows"`
	List interface{} `json:"list"`
}

type reorderRequest struct {
	// The entry to move in order
	Entry uint
	// The other entry the first one will be places before
	OtherEntry uint
}

// A request for listing the contents of a playlist
type playlistEntryListRequest struct {
	Pagination
	PlaylistID uint
}

// A request made when logging in
type loginRequest struct {
	User string `json:"user"`
	Pass string `json:"password"`
}

// -- Configuration ----------------------------------------------------------------------------------------------------

// MakeConfigEndpoints creates the endpoints needed to use the configuration service
func MakeConfigEndpoints(s ConfigService) ConfigEndpoints {
	return ConfigEndpoints{
		GetWhitelist:        EnsureUserLoggedIn(MakeGetWhitelistEndpoint(s)),
		AddToWhitelist:      EnsureUserLoggedIn(MakeAddToWhitelistEndpoint(s)),
		RemoveFromWhitelist: EnsureUserLoggedIn(MakeRemoveFromWhitelistEndpoint(s)),
	}
}

// MakeGetWhitelistEndpoint returns and endpoint calling the GetWhitelist method of the ConfigService
func MakeGetWhitelistEndpoint(s ConfigService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		return basicResponse{true, s.WhitelistedIPs(ctx)}, nil
	}
}

// MakeAddToWhitelistEndpoint returns and endpoint calling the AddToWhitelist method of the ConfigService
func MakeAddToWhitelistEndpoint(s ConfigService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		ipAddr, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("missing IP address parameter")
		}
		if err := s.AddToWhitelist(ctx, ipAddr); err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// MakeRemoveFromWhitelistEndpoint returns and endpoint calling the RemoveFromWhitelist method of the ConfigService
func MakeRemoveFromWhitelistEndpoint(s ConfigService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		ipAddr, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("missing IP address parameter")
		}
		if err := s.RemoveFromWhitelist(ctx, ipAddr); err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// -- Scraping ---------------------------------------------------------------------------------------------------------

// MakeScrapingEndpoints creates the endpoints needed to use the scraping service
func MakeScrapingEndpoints(s ScrapingService) ScrapingEndpoints {
	return ScrapingEndpoints{
		ListDirs:    EnsureUserLoggedIn(MakeListDirsEndpoint(s)),
		ListScrapes: EnsureUserLoggedIn(MakeListScrapesEndpoint(s)),
		GetScrape:   EnsureUserLoggedIn(MakeGetScrapeEndpoint(s)),
		Start:       EnsureUserLoggedIn(MakeStartEndpoint(s)),
	}
}

// MakeListDirsEndpoint returns an endpoint calling the ListDirs method on the provided ScrapingService
func MakeListDirsEndpoint(s ScrapingService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		parentDir, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("illegal path parameter")
		}
		lst, err := s.ListDirs(ctx, parentDir)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, lst}, nil
	}
}

// MakeListScrapesEndpoint returns an endpoint calling the ListScrapes method on the provided ScrapingService
func MakeListScrapesEndpoint(s ScrapingService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		lst, err := s.ListScrapes(ctx)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, lst}, nil
	}
}

// MakeGetScrapeEndpoint returns an endpoint calling the GetScrape method on the provided ScrapingService
func MakeGetScrapeEndpoint(s ScrapingService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rootDir, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("illegal path parameter")
		}
		return basicResponse{true, s.GetScrape(ctx, rootDir)}, nil
	}
}

// MakeStartEndpoint returns an endpoint calling the Start method on the provided ScrapingService
func MakeStartEndpoint(s ScrapingService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		rootDir, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("illegal path parameter")
		}
		err := s.Start(ctx, rootDir)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// -- Video data -------------------------------------------------------------------------------------------------------

// Repacks the videos into the guest-facing response type
func repackVideos(vids []models.Video) []models.VideoSummary {
	var res []models.VideoSummary
	for _, vid := range vids {
		res = append(res, vid.VideoSummary)
	}
	return res
}

// MakeVideoEndpoints creates the endpoints needed for using the video service
func MakeVideoEndpoints(s VideoService) VideoEndpoints {
	return VideoEndpoints{
		List:   MakeListVideosEndpoint(s),
		Get:    EnsureUserLoggedIn(MakeGetVideoEndpoint(s)),
		Update: EnsureUserLoggedIn(MakeUpdateVideoEndpoint(s)),
		Delete: EnsureUserLoggedIn(MakeDeleteVideoEndpoint(s)),
	}
}

// MakeListVideosEndpoint returns an endpoint calling the List method on the provided VideoService
func MakeListVideosEndpoint(s VideoService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		search, ok := request.(Search)
		if !ok {
			return nil, fmt.Errorf("illegal search parameter")
		}
		vids, numRows, err := s.List(ctx, &search)
		if err != nil {
			return nil, err
		}
		sess := ctxhelper.Session(ctx)
		if sess != nil && sess.UserCan(models.PermVideoSeeFullDetails) {
			// We have an admin - so he gets the full video data
			return basicResponse{true, pagingResponse{numRows, vids}}, nil
		}
		// Repack the videos to the guest-facing data type containing no internal information
		return basicResponse{true, pagingResponse{numRows, repackVideos(vids)}}, nil
	}
}

// MakeGetVideoEndpoint returns an endpoint calling the List method on the provided VideoService
func MakeGetVideoEndpoint(s VideoService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("illegal video ID parameter")
		}
		vid, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, vid}, nil
	}
}

// MakeUpdateVideoEndpoint returns an endpoint calling the List method on the provided VideoService
func MakeUpdateVideoEndpoint(s VideoService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		vid, ok := request.(models.Video)
		if !ok {
			return nil, fmt.Errorf("illegal video parameter")
		}
		if err := s.Update(ctx, &vid); err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// MakeDeleteVideoEndpoint returns an endpoint calling the List method on the provided VideoService
func MakeDeleteVideoEndpoint(s VideoService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("illegal video ID parameter")
		}
		if err := s.Delete(ctx, id); err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// -- Playlists --------------------------------------------------------------------------------------------------------

// MakePlaylistEndpoints creates the endpoints needed for using the playlist service
func MakePlaylistEndpoints(s PlaylistService) PlaylistEndpoints {
	return PlaylistEndpoints{
		Create:           EnsureUserLoggedIn(MakeCreatePlaylistEndpoint(s)),
		Update:           EnsureUserLoggedIn(MakeUpdatePlaylistEndpoint(s)),
		Delete:           EnsureUserLoggedIn(MakeDeletePlaylistEndpoint(s)),
		Get:              EnsureUserLoggedIn(MakeGetPlaylistEndpoint(s)),
		List:             EnsureUserLoggedIn(MakeListPlaylistsEndpoint(s)),
		ListEntries:      EnsureUserLoggedIn(MakeListPlaylistEntriesEndpoint(s)),
		AddEntry:         EnsureUserLoggedIn(MakeAddPlaylistEntryEndpoint(s)),
		PlaceEntryBefore: EnsureUserLoggedIn(MakePlaceEntryBeforeEndpint(s)),
		UpdateEntry:      EnsureUserLoggedIn(MakeUpdateEntryEndpoint(s)),
		DeleteEntry:      EnsureUserLoggedIn(MakeDeleteEntryEndpoint(s)),
		GetMain:          MakeGetMainPlaylistEndpoint(s),
		ListMainEntries:  MakeListMainPlaylistEntriesEndpoint(s),
		AddMainEntry:     MakeAddMainPlaylistEntryEndpoint(s),
	}
}

// MakeListPlaylistsEndpoint returns an endpoint calling the Create method on the provided PlaylistService
func MakeListPlaylistsEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		se, ok := request.(Search)
		if !ok {
			return nil, fmt.Errorf("illegal search parameter")
		}
		lists, numRows, err := s.List(ctx, &se)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, pagingResponse{numRows, lists}}, nil
	}
}

// MakeCreatePlaylistEndpoint returns an endpoint calling the Create method on the provided PlaylistService
func MakeCreatePlaylistEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		playlist, ok := request.(models.Playlist)
		if !ok {
			return nil, fmt.Errorf("illegal playlist parameter")
		}
		pl, err := s.Create(ctx, &playlist)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, pl}, nil
	}
}

// MakeGetPlaylistEndpoint returns an endpoint calling the Get method on the provided PlaylistService
func MakeGetPlaylistEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(uint)
		if !ok {
			return nil, fmt.Errorf("illegal playlist ID")
		}
		pl, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, pl}, nil
	}
}

// MakeUpdatePlaylistEndpoint returns an endpoint calling the Update method on the provided PlaylistService
func MakeUpdatePlaylistEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		playlist, ok := request.(models.Playlist)
		if !ok {
			return nil, fmt.Errorf("illegal playlist parameter")
		}
		err := s.Update(ctx, &playlist)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// MakeDeletePlaylistEndpoint returns an endpoint calling the Delete method on the provided PlaylistService
func MakeDeletePlaylistEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(uint)
		if !ok {
			return nil, fmt.Errorf("illegal playlist ID")
		}
		err := s.Delete(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// MakeAddPlaylistEntryEndpoint returns an endpoint calling the AddEntry method on the provided PlaylistService
func MakeAddPlaylistEntryEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(models.PlaylistEntry)
		if !ok {
			return nil, fmt.Errorf("illegal playlist entry request")
		}
		err := s.AddEntry(ctx, req.PlaylistID, &req)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// MakeListPlaylistEntriesEndpoint returns an endpoint calling the ListEntries method on the provided PlaylistService
func MakeListPlaylistEntriesEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(playlistEntryListRequest)
		if !ok {
			return nil, fmt.Errorf("illegal playlist list request")
		}
		list, numRows, err := s.ListEntries(ctx, req.PlaylistID, req.Offset, req.Limit)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, pagingResponse{numRows, list}}, nil
	}
}

// MakeGetMainPlaylistEndpoint returns an endpoint calling the GetMain method on the provided PlaylistService
func MakeGetMainPlaylistEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		pl, err := s.GetMain(ctx)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, pl}, nil
	}
}

// MakeListMainPlaylistEntriesEndpoint returns an endpoint calling the ListMainEntries method on the provided
// PlaylistService
func MakeListMainPlaylistEntriesEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		pag, ok := request.(Pagination)
		if !ok {
			return nil, fmt.Errorf("illegal pagination parameter")
		}
		list, numRows, err := s.ListMainEntries(ctx, pag.Offset, pag.Limit)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, pagingResponse{numRows, list}}, nil
	}
}

// MakeUpdateEntryEndpoint returns an endpoint calling the UpdateEntry method on the provided PlaylistService
func MakeUpdateEntryEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(models.PlaylistEntry)
		if !ok {
			return nil, fmt.Errorf("illegal playlist entry")
		}
		err := s.UpdateEntry(ctx, req)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// MakeDeleteEntryEndpoint returns an endpoint calling the DeleteEntry method on the provided PlaylistService
func MakeDeleteEntryEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(uint)
		if !ok {
			return nil, fmt.Errorf("illegal entry ID")
		}
		err := s.DeleteEntry(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// MakePlaceEntryBeforeEndpint returns an endpoint calling the PlaceEntryBefore method on the provided PlaylistService
func MakePlaceEntryBeforeEndpint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(reorderRequest)
		if !ok {
			return nil, fmt.Errorf("illegal reorder request")
		}
		err := s.PlaceEntryBefore(ctx, req.Entry, req.OtherEntry)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// MakeAddMainPlaylistEntryEndpoint returns an endpoint calling the AddMainEntry method on the provided PlaylistService
func MakeAddMainPlaylistEntryEndpoint(s PlaylistService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(models.PlaylistEntry)
		if !ok {
			return nil, fmt.Errorf("illegal playlist entry request")
		}
		err := s.AddMainEntry(ctx, &req)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

// -- Events -----------------------------------------------------------------------------------------------------------

// MakeEventEndpoints builds the endpoints needed to communicate with the Event Service
func MakeEventEndpoints(s EventService) EventEndpoints {
	return EventEndpoints{
		List:            EnsureUserLoggedIn(makeListEventsEndpoint(s)),
		Get:             EnsureUserLoggedIn(makeGetEventEndpoint(s)),
		Create:          EnsureUserLoggedIn(makeCreateEventEndpoint(s)),
		Update:          EnsureUserLoggedIn(makeUpdateEventEndpoint(s)),
		Delete:          EnsureUserLoggedIn(makeDeleteEventEndpoint(s)),
		SetCurrentEvent: EnsureUserLoggedIn(makeSetCurrentEventEndpoint(s)),
		CurrentEvent:    makeGetCurrentEventEndpoint(s),
	}
}

func makeListEventsEndpoint(s EventService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		se, ok := request.(Search)
		if !ok {
			return nil, fmt.Errorf("illegal search parameter")
		}
		list, numRows, err := s.List(ctx, &se)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, pagingResponse{numRows, list}}, nil
	}
}

func makeGetEventEndpoint(s EventService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(uint)
		if !ok {
			return nil, fmt.Errorf("illegal event ID")
		}
		ev, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, ev}, nil
	}
}

func makeCreateEventEndpoint(s EventService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		event, ok := request.(models.Event)
		if !ok {
			return nil, fmt.Errorf("illegal event parameter")
		}
		ev, err := s.Create(ctx, &event)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, ev}, nil
	}
}

func makeUpdateEventEndpoint(s EventService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		event, ok := request.(models.Event)
		if !ok {
			return nil, fmt.Errorf("illegal event parameter")
		}
		err := s.Update(ctx, &event)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

func makeDeleteEventEndpoint(s EventService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(uint)
		if !ok {
			return nil, fmt.Errorf("illegal event ID")
		}
		err := s.Delete(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

func makeSetCurrentEventEndpoint(s EventService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(uint)
		if !ok {
			return nil, fmt.Errorf("illegal event ID")
		}
		err := s.SetCurrentEvent(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

func makeGetCurrentEventEndpoint(s EventService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		ev, err := s.CurrentEvent(ctx)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, ev}, nil
	}
}

// -- Sessions ---------------------------------------------------------------------------------------------------------

// MakeSessionEndpoints builds the endpoints needed to communicate with the Session Service
func MakeSessionEndpoints(s SessionService) SessionEndpoints {
	return SessionEndpoints{
		Login:  makeLoginEndpoint(s),
		Logout: makeLogoutEndpoint(s),
		WhoAmI: makeWhoAmIEndpoint(s),
	}
}

func makeLoginEndpoint(s SessionService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		se, ok := request.(loginRequest)
		if !ok {
			return nil, fmt.Errorf("illegal login request")
		}
		si, err := s.Login(ctx, se.User, se.Pass)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, si}, nil
	}
}

func makeLogoutEndpoint(s SessionService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("illegal session token")
		}
		err := s.Logout(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, nil}, nil
	}
}

func makeWhoAmIEndpoint(s SessionService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		id, ok := request.(string)
		if !ok {
			return nil, fmt.Errorf("illegal session token")
		}
		si, err := s.WhoAmI(ctx, id)
		if err != nil {
			return nil, err
		}
		return basicResponse{true, si}, nil
	}
}
