package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// HTTPError represents an HTTP-level error.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http error %d: %s", e.StatusCode, e.Message)
}

// HTTPClient defines the interface for making HTTP requests to the API.
type HTTPClient interface {
	Post(ctx context.Context, endpoint string, body any, token string) ([]byte, error)
	Get(ctx context.Context, endpoint string, token string) ([]byte, error)
	SetBaseURL(baseURL string)
}

var _ HTTPClient = (*Client)(nil)

// Client wraps the HTTP client for API requests.
type Client struct {
	baseURL    string
	httpClient *http.Client
	debug      bool
	retryCount int
}

// NewClient creates a new HTTP transport client.
func NewClient(baseURL string, timeout time.Duration, retryCount int, debug bool) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		debug:      debug,
		retryCount: retryCount,
	}
}

// Post sends a POST request to the API.
func (c *Client) Post(ctx context.Context, endpoint string, body any, token string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, endpoint, body, token)
}

// Get sends a GET request to the API.
func (c *Client) Get(ctx context.Context, endpoint string, token string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, endpoint, nil, token)
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, body any, token string) ([]byte, error) {
	url := c.baseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)

		if c.debug {
			log.Printf("[DEBUG] Request: %s %s\n%s", method, url, string(jsonBody))
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}

			// Re-create body reader for retry
			if body != nil {
				jsonBody, _ := json.Marshal(body)
				bodyReader = bytes.NewReader(jsonBody)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "AquaTemp-Go/1.0")

		if token != "" {
			req.Header.Set("x-token", token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if c.debug {
				log.Printf("[DEBUG] Request failed (attempt %d): %v", attempt+1, err)
			}
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if c.debug {
			log.Printf("[DEBUG] Response (%d): %s", resp.StatusCode, string(respBody))
		}

		if resp.StatusCode >= 500 {
			lastErr = &HTTPError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("server error: %s", http.StatusText(resp.StatusCode)),
			}
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, &HTTPError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("client error: %s", http.StatusText(resp.StatusCode)),
			}
		}

		return respBody, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.retryCount+1, lastErr)
	}

	return nil, fmt.Errorf("request failed")
}

// SetBaseURL updates the base URL.
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}
