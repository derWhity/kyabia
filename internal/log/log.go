package log

const (
	// FldFile is the name of the log field for storing file name information
	FldFile = "file"
	// FldPath is the name of the log field for storing path name information
	FldPath = "path"
	// FldTransport is the name of the log field for storing a transport name
	FldTransport = "transport"
	// FldSession is the name of the log field for storing the session ID
	FldSession = "session"
	// FldUser is the name of the log field for storing the ID of the currently active user
	FldUser = "user"
	// FldVideo is the name of the log field for storing a video hash (ID)
	FldVideo = "video"
	// FldVersion is the version number of the application
	FldVersion = "ver"
	// FldIP is the IP address used in the log entry
	FldIP = "ip"
	// FldID is the ID of an entity used in the log entry
	FldID = "id"
	// FldSearch is a search term used in a serach
	FldSearch = "search"
	// FldOffset is the requested offset value in a search
	FldOffset = "offset"
	// FldLimit is the requested result limit in a search
	FldLimit = "limit"
)
