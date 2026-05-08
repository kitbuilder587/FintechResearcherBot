package llm

import (
	"context"
	"errors"
)

var (
	ErrAuthFailed    = errors.New("authentication failed")
	ErrRequestFailed = errors.New("request failed")
	ErrEmptyResponse = errors.New("empty response")
	ErrRateLimit     = errors.New("rate limit exceeded")
)

type Client interface {
	CompleteWithSystem(ctx context.Context, system, prompt string) (string, error)
}

type UsageClient interface {
	CompleteWithSystemUsage(ctx context.Context, system, prompt string) (string, Usage, error)
}
