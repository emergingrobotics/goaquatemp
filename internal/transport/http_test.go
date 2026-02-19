package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      HTTPError
		expected string
	}{
		{
			name:     "bad request",
			err:      HTTPError{StatusCode: 400, Message: "invalid input"},
			expected: "http error 400: invalid input",
		},
		{
			name:     "not found",
			err:      HTTPError{StatusCode: 404, Message: "resource not found"},
			expected: "http error 404: resource not found",
		},
		{
			name:     "internal server error",
			err:      HTTPError{StatusCode: 500, Message: "server error"},
			expected: "http error 500: server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("HTTPError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("https://api.example.com", 30*time.Second, 3, true)

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.baseURL != "https://api.example.com" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://api.example.com")
	}

	if client.retryCount != 3 {
		t.Errorf("retryCount = %d, want %d", client.retryCount, 3)
	}

	if client.debug != true {
		t.Errorf("debug = %v, want %v", client.debug, true)
	}
}

func TestClient_SetBaseURL(t *testing.T) {
	client := NewClient("https://old.example.com", 30*time.Second, 0, false)
	client.SetBaseURL("https://new.example.com")

	if client.baseURL != "https://new.example.com" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://new.example.com")
	}
}

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want %q", r.Method, http.MethodGet)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		if r.Header.Get("User-Agent") != "AquaTemp-Go/1.0" {
			t.Errorf("User-Agent = %q, want %q", r.Header.Get("User-Agent"), "AquaTemp-Go/1.0")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, 0, false)
	resp, err := client.Get(context.Background(), "/test", "")

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if string(resp) != `{"status":"ok"}` {
		t.Errorf("Get() response = %q, want %q", string(resp), `{"status":"ok"}`)
	}
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want %q", r.Method, http.MethodPost)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}

		if body["key"] != "value" {
			t.Errorf("body[key] = %q, want %q", body["key"], "value")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result": "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, 0, false)
	resp, err := client.Post(context.Background(), "/test", map[string]string{"key": "value"}, "")

	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if result["result"] != "success" {
		t.Errorf("result = %q, want %q", result["result"], "success")
	}
}

func TestClient_PostWithToken(t *testing.T) {
	expectedToken := "test-token-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("x-token")
		if token != expectedToken {
			t.Errorf("x-token = %q, want %q", token, expectedToken)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, 0, false)
	_, err := client.Post(context.Background(), "/test", nil, expectedToken)

	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
}

func TestClient_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, 0, false)
	_, err := client.Get(context.Background(), "/test", "")

	if err == nil {
		t.Fatal("Get() expected error, got nil")
	}

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *HTTPError", err)
	}

	if httpErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want %d", httpErr.StatusCode, 400)
	}
}

func TestClient_ServerErrorRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, 3, false)
	resp, err := client.Get(context.Background(), "/test", "")

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if attempts != 3 {
		t.Errorf("attempts = %d, want %d", attempts, 3)
	}

	if string(resp) != `{"status":"ok"}` {
		t.Errorf("response = %q, want %q", string(resp), `{"status":"ok"}`)
	}
}

func TestClient_ServerErrorExhaustedRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, 2, false)
	_, err := client.Get(context.Background(), "/test", "")

	if err == nil {
		t.Fatal("Get() expected error after exhausted retries, got nil")
	}

	// 1 initial attempt + 2 retries = 3 total attempts
	if attempts != 3 {
		t.Errorf("attempts = %d, want %d", attempts, 3)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, 0, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Get(ctx, "/test", "")

	if err == nil {
		t.Fatal("Get() expected error with cancelled context, got nil")
	}
}
