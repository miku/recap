// Package llm is a thin client for OpenAI-compatible /chat/completions
// endpoints. It is deliberately small: one method, one request shape.
// Streaming, function calling, and structured output can be added later.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

type Option func(*Client)

func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// New returns a client targeting endpoint (the OpenAI-compatible base URL,
// e.g. "http://localhost:11434/v1"). apiKey may be empty for local servers.
func New(endpoint, apiKey string, opts ...Option) *Client {
	c := &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		http:     &http.Client{Timeout: 5 * time.Minute},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a single user message and returns the assistant's reply.
func (c *Client) Complete(ctx context.Context, model, userPrompt string) (string, error) {
	return c.Chat(ctx, model, []Message{{Role: "user", Content: userPrompt}})
}

// Chat sends a full message list. Use this when a system prompt or
// multi-turn context is needed.
func (c *Client) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: model, Messages: messages})
	if err != nil {
		return "", err
	}
	url := c.endpoint + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("%s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return cr.Choices[0].Message.Content, nil
}
