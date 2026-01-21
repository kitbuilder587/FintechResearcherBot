package mock

import (
	"context"
	"strings"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/llm"
)

type Client struct {
	Response string
	Error    error
	Delay    time.Duration

	CallCount  int
	LastSystem string
	LastPrompt string
	AllCalls   []LLMCall
}

type LLMCall struct {
	System string
	Prompt string
}

func New() *Client {
	return &Client{
		Response: "This is a mock response with sources [S1] and [S2].",
	}
}

func (c *Client) WithResponse(response string) *Client {
	c.Response = response
	return c
}

func (c *Client) WithError(err error) *Client {
	c.Error = err
	return c
}

func (c *Client) WithDelay(delay time.Duration) *Client {
	c.Delay = delay
	return c
}

func (c *Client) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	c.CallCount++
	c.LastSystem = system
	c.LastPrompt = prompt
	c.AllCalls = append(c.AllCalls, LLMCall{System: system, Prompt: prompt})

	if c.Delay > 0 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(c.Delay):
		}
	}

	if c.Error != nil {
		return "", c.Error
	}

	return c.Response, nil
}

func (c *Client) Reset() {
	c.CallCount = 0
	c.LastSystem = ""
	c.LastPrompt = ""
	c.AllCalls = nil
}

func (c *Client) HasCriticCall() bool {
	for _, call := range c.AllCalls {
		if strings.Contains(call.System, "critical reviewer") {
			return true
		}
	}
	return false
}

var _ llm.Client = (*Client)(nil)
