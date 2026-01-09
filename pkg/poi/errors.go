package poi

import "errors"

var (
	// ErrPOINotFound indicates the requested POI was not found in cache or store.
	ErrPOINotFound = errors.New("poi not found")
	// ErrStoreFailure indicates a failure in the underlying storage.
	ErrStoreFailure = errors.New("poi store failure")
)
