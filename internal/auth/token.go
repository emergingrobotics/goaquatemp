package auth

import (
	"sync"
	"time"
)

// Token represents an authentication token.
type Token struct {
	Value     string
	UserID    string
	ExpiresAt time.Time
}

// IsValid returns true if the token is set and not expired.
func (t *Token) IsValid() bool {
	if t == nil || t.Value == "" {
		return false
	}
	// Add a buffer of 60 seconds before actual expiry
	return time.Now().Add(60 * time.Second).Before(t.ExpiresAt)
}

// IsExpired returns true if the token has expired.
func (t *Token) IsExpired() bool {
	if t == nil {
		return true
	}
	return time.Now().After(t.ExpiresAt)
}

// TokenStore provides thread-safe storage for authentication tokens.
type TokenStore struct {
	mu    sync.RWMutex
	token *Token
}

// NewTokenStore creates a new TokenStore.
func NewTokenStore() *TokenStore {
	return &TokenStore{}
}

// Set stores a token.
func (s *TokenStore) Set(token *Token) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
}

// Get returns the current token.
func (s *TokenStore) Get() *Token {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token
}

// Clear removes the stored token.
func (s *TokenStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = nil
}

// IsValid returns true if the stored token is valid.
func (s *TokenStore) IsValid() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.token.IsValid()
}
