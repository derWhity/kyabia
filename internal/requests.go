package internal

// -- Request data -----------------------------------------------------------------------------------------------------

// Pagination describes a request that uses paging data to retrieve only a subset of the full result
type Pagination struct {
	// Position in the resultset to start the returned result at
	Offset uint
	// Number of items to return
	Limit uint
}

// Search describes a typical search request with a search term and pagination information
type Search struct {
	Pagination
	// The string to search for
	Search string
}
