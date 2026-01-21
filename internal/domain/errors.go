package domain

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInternal     = errors.New("internal error")
)

var (
	ErrInvalidURL         = errors.New("invalid url")
	ErrDuplicateSource    = errors.New("source already exists")
	ErrSourceNotFound     = errors.New("source not found")
	ErrSourceLimitReached = errors.New("source limit reached")
)

var (
	ErrUserNotFound = errors.New("user not found")
)

var (
	ErrEmptyQuery       = errors.New("empty query")
	ErrQueryTooLong     = errors.New("query too long")
	ErrNoSources        = errors.New("no sources available")
	ErrAllSourcesFailed = errors.New("all sources failed")
	ErrLLMFailed        = errors.New("llm request failed")
	ErrNoResults        = errors.New("no results found")
)

var (
	ErrInvalidMaxRetries  = errors.New("max retries must be non-negative")
	ErrMaxRetriesExceeded = errors.New("max retries cannot exceed 10")
)

var (
	ErrEmptyContent      = errors.New("empty content")
	ErrEmptyEntityName   = errors.New("empty entity name")
	ErrInvalidEntityType = errors.New("invalid entity type")
	ErrEmptyQuestion     = errors.New("empty question")
)
