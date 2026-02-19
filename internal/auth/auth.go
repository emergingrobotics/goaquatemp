package auth

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/user/goaquatemp/internal/api"
	"github.com/user/goaquatemp/internal/transport"
)

// APIVersion represents the API version for password handling.
type APIVersion string

const (
	APIModern APIVersion = "modern"
)

// AuthError represents an authentication error.
type AuthError struct {
	Code      int
	Message   string
	Retryable bool
	Err       error
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("auth error %d: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("auth error %d: %s", e.Code, e.Message)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// AuthManager defines the interface for authentication operations.
type AuthManager interface {
	Authenticate(ctx context.Context) (*Token, error)
	RefreshToken(ctx context.Context) (*Token, error)
	GetToken() *Token
	GetTokenValue() string
	IsAuthenticated() bool
	Clear()
	EnsureAuthenticated(ctx context.Context) error
	FormatTokenHeader() string
}

var _ AuthManager = (*Manager)(nil)

// Manager handles authentication with the Aqua Temp API.
type Manager struct {
	store      *TokenStore
	http       transport.HTTPClient
	apiVersion APIVersion
	username   string
	password   string
}

// NewManager creates a new authentication manager.
func NewManager(http transport.HTTPClient, apiVersion string, username, password string) *Manager {
	return &Manager{
		store:      NewTokenStore(),
		http:       http,
		apiVersion: APIVersion(apiVersion),
		username:   username,
		password:   password,
	}
}

// Authenticate performs authentication and stores the token.
func (m *Manager) Authenticate(ctx context.Context) (*Token, error) {
	password := m.preparePassword()

	req := api.NewLoginRequest(m.username, password)

	body, err := m.http.Post(ctx, api.EndpointLogin, req, "")
	if err != nil {
		return nil, &AuthError{
			Code:      -1,
			Message:   "failed to send login request",
			Retryable: true,
			Err:       err,
		}
	}

	var resp api.LoginResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &AuthError{
			Code:      -1,
			Message:   "failed to parse login response",
			Retryable: false,
			Err:       err,
		}
	}

	if !resp.IsSuccess() {
		retryable := resp.ErrorCode >= 500
		return nil, &AuthError{
			Code:      resp.ErrorCode,
			Message:   resp.ErrorMessage,
			Retryable: retryable,
		}
	}

	if resp.ObjectResult == nil || resp.ObjectResult.Token == "" {
		return nil, &AuthError{
			Code:    -1,
			Message: "no token in response",
		}
	}

	expiresIn := resp.ObjectResult.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // Default to 1 hour
	}

	token := &Token{
		Value:     resp.ObjectResult.Token,
		UserID:    resp.ObjectResult.UserID,
		ExpiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}

	m.store.Set(token)
	return token, nil
}

// RefreshToken refreshes the authentication token.
func (m *Manager) RefreshToken(ctx context.Context) (*Token, error) {
	// The Aqua Temp API doesn't have a refresh endpoint,
	// so we just re-authenticate.
	return m.Authenticate(ctx)
}

// GetToken returns the current token.
func (m *Manager) GetToken() *Token {
	return m.store.Get()
}

// GetTokenValue returns the current token value string.
func (m *Manager) GetTokenValue() string {
	token := m.store.Get()
	if token == nil {
		return ""
	}
	return token.Value
}

// IsAuthenticated returns true if we have a valid token.
func (m *Manager) IsAuthenticated() bool {
	return m.store.IsValid()
}

// Clear removes the stored token.
func (m *Manager) Clear() {
	m.store.Clear()
}

// EnsureAuthenticated ensures we have a valid token, refreshing if needed.
func (m *Manager) EnsureAuthenticated(ctx context.Context) error {
	if m.IsAuthenticated() {
		return nil
	}

	_, err := m.RefreshToken(ctx)
	return err
}

// preparePassword prepares the password for the API.
// Modern API (v1.5.9+) requires MD5 hashed password.
func (m *Manager) preparePassword() string {
	if m.apiVersion == APIModern {
		return md5Hash(m.password)
	}
	return m.password
}

// md5Hash returns the MD5 hash of a string as a lowercase hex string.
func md5Hash(s string) string {
	hash := md5.Sum([]byte(s))
	return strings.ToLower(hex.EncodeToString(hash[:]))
}

// FormatTokenHeader returns the token formatted for the x-token header.
func (m *Manager) FormatTokenHeader() string {
	token := m.GetTokenValue()
	if token == "" {
		return ""
	}
	return fmt.Sprintf("bearer %s", token)
}
