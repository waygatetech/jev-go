// Package jev is a client for the Jev semantic judgment API
// (https://api.typesafe.ai).
package jev

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

const (
	DefaultBaseURL     = "https://api.typesafe.ai"
	defaultMaxAttempts = 3
)

var defaultRetryBaseDelay = 200 * time.Millisecond

// Question is a single semantic judgment Jev is asked to make.
type Question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

// Answer holds the result of one Question. Only the fields matching
// Type are populated; the rest are left as zero values.
type Answer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
}

// Request is a call to the /v1/systemone endpoint. State is caller-defined
// and marshaled as-is; Jev does not prescribe its shape.
type Request struct {
	Model     string              `json:"model"`
	State     any                 `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Response is the decoded reply from /v1/systemone, plus the HTTP status
// it arrived with.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	HTTPStatus int `json:"-"`
}

// Client calls the Jev API, retrying rate-limit and overload responses
// with exponential backoff.
type Client struct {
	BaseURL        string
	APIKey         string
	HTTPClient     *http.Client
	MaxAttempts    int
	RetryBaseDelay time.Duration
}

// NewClient returns a Client with default BaseURL, timeout, and retry
// settings for the given API key.
func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:        DefaultBaseURL,
		APIKey:         apiKey,
		HTTPClient:     &http.Client{Timeout: 10 * time.Second},
		MaxAttempts:    defaultMaxAttempts,
		RetryBaseDelay: defaultRetryBaseDelay,
	}
}

// Evaluate sends req to /v1/systemone, retrying on HTTP 429 and 529.
// The returned Response's HTTPStatus is set whenever a response was
// received, even when err is non-nil.
func (c *Client) Evaluate(ctx context.Context, req Request) (Response, error) {
	var response Response
	body, err := json.Marshal(req)
	if err != nil {
		return response, fmt.Errorf("encode Jev request: %w", err)
	}

	maxAttempts := c.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	retryDelay := c.RetryBaseDelay
	if retryDelay <= 0 {
		retryDelay = defaultRetryBaseDelay
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			strings.TrimRight(c.BaseURL, "/")+"/v1/systemone", bytes.NewReader(body))
		if err != nil {
			return response, fmt.Errorf("create Jev request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(httpReq)
		if err != nil {
			return response, fmt.Errorf("Jev request failed: %w", err)
		}
		response.HTTPStatus = resp.StatusCode
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return response, fmt.Errorf("read Jev response: %w", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 529 {
			if attempt+1 < maxAttempts {
				timer := time.NewTimer(retryDelay * time.Duration(1<<attempt))
				select {
				case <-ctx.Done():
					timer.Stop()
					return response, fmt.Errorf("Jev retry canceled: %w", ctx.Err())
				case <-timer.C:
				}
				continue
			}
			return response, fmt.Errorf("Jev HTTP %d after %d attempts", resp.StatusCode, maxAttempts)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return response, fmt.Errorf("Jev HTTP %d", resp.StatusCode)
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return response, fmt.Errorf("decode Jev response: %w", err)
		}
		response.HTTPStatus = resp.StatusCode
		return response, nil
	}
	return response, fmt.Errorf("Jev request exhausted retries")
}
