package wikidata

import "errors"

var (
	// ErrNetwork indicates a failure in network communication.
	ErrNetwork = errors.New("wikidata network error")
	// ErrParse indicates a failure to parse the response.
	ErrParse = errors.New("wikidata parse error")
	// ErrInvalidQuery indicates the generated SPARQL query was invalid.
	ErrInvalidQuery = errors.New("wikidata invalid query")
	// ErrNotFound indicates the requested entity was not found.
	ErrNotFound = errors.New("wikidata entity not found")
)
