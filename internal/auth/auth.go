package auth

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/emergingrobotics/goaquatemp/internal/api"
	"github.com/emergingrobotics/goaquatemp/internal/transport"
)

// APIVersion is the API version name (legacy, hitemp, modern).
type APIVersion string

// defaultTokenLifetime is how long a token is assumed valid before the
// client proactively logs in again.
const defaultTokenLifetime = 12 * time.Hour

// AuthError represents an authentication error.
type AuthError struct {
	Code      int
	Message   string
	Retryable bool
	// Rejected is true when the server answered and refused the login
	// (as opposed to a transport or parse failure).
	Rejected bool
	Err      error
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
	dialect    api.Dialect
	apiVersion APIVersion
	username   string
	password   string
}

// NewManager creates a new authentication manager.
func NewManager(http transport.HTTPClient, apiVersion string, username, password string) *Manager {
	return &Manager{
		store:      NewTokenStore(),
		http:       http,
		dialect:    api.DialectFor(apiVersion),
		apiVersion: APIVersion(apiVersion),
		username:   username,
		password:   password,
	}
}

// Authenticate performs authentication and stores the token.
func (m *Manager) Authenticate(ctx context.Context) (*Token, error) {
	req := m.dialect.LoginRequest(m.username, m.preparePassword())

	body, err := m.http.Post(ctx, m.dialect.Path(api.EndpointLogin), req, "")
	if err != nil {
		return nil, &AuthError{
			Code:      -1,
			Message:   "failed to send login request",
			Retryable: true,
			Err:       err,
		}
	}

	resp, err := m.dialect.ParseResponse(body)
	if err != nil {
		return nil, &AuthError{
			Code:    -1,
			Message: "failed to parse login response",
			Err:     err,
		}
	}

	if !resp.IsSuccess() {
		return nil, &AuthError{
			Code:     resp.ErrorCode,
			Message:  fmt.Sprintf("server rejected login: %s", resp.ErrorMessage),
			Rejected: true,
		}
	}

	result, err := m.dialect.ParseLogin(resp.Result)
	if err != nil {
		return nil, &AuthError{Code: -1, Message: "failed to parse login result", Err: err}
	}
	if result.Token == "" {
		return nil, &AuthError{Code: -1, Message: "no token in login response"}
	}

	// The API does not report token lifetime; tokens are invalidated
	// server-side (error -100), which the client handles by re-logging in.
	token := &Token{
		Value:     result.Token,
		UserID:    result.UserID,
		ExpiresAt: time.Now().Add(defaultTokenLifetime),
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

// preparePassword prepares the password for the API. Every API version
// expects the lowercase hex MD5 of the password (matches the reference
// Home Assistant integration).
func (m *Manager) preparePassword() string {
	return md5Hash(m.password)
}

// md5Hash returns the MD5 hash of a string as a lowercase hex string.
func md5Hash(s string) string {
	hash := md5.Sum([]byte(s))
	return strings.ToLower(hex.EncodeToString(hash[:]))
}

// FormatTokenHeader returns the value for the x-token header (the bare token).
func (m *Manager) FormatTokenHeader() string {
	token := m.GetTokenValue()
	if token == "" {
		return ""
	}
	return token
}
