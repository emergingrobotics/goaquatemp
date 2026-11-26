package auth

import (
	"testing"
	"time"
)

func TestTokenIsValid(t *testing.T) {
	tests := []struct {
		name  string
		token *Token
		want  bool
	}{
		{
			name:  "nil token",
			token: nil,
			want:  false,
		},
		{
			name:  "empty value",
			token: &Token{Value: "", ExpiresAt: time.Now().Add(time.Hour)},
			want:  false,
		},
		{
			name:  "expired",
			token: &Token{Value: "test", ExpiresAt: time.Now().Add(-time.Hour)},
			want:  false,
		},
		{
			name:  "expiring soon (within 60s buffer)",
			token: &Token{Value: "test", ExpiresAt: time.Now().Add(30 * time.Second)},
			want:  false,
		},
		{
			name:  "valid",
			token: &Token{Value: "test", ExpiresAt: time.Now().Add(time.Hour)},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.token.IsValid()
			if got != tt.want {
				t.Errorf("Token.IsValid() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestTokenStore(t *testing.T) {
	store := NewTokenStore()

	// Initially empty
	if store.IsValid() {
		t.Error("expected IsValid() = false for empty store")
	}

	if store.Get() != nil {
		t.Error("expected Get() = nil for empty store")
	}

	// Set a token
	token := &Token{
		Value:     "test-token",
		UserID:    "user123",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	store.Set(token)

	if !store.IsValid() {
		t.Error("expected IsValid() = true after Set()")
	}

	got := store.Get()
	if got == nil || got.Value != "test-token" {
		t.Error("expected Get() to return the set token")
	}

	// Clear the token
	store.Clear()

	if store.IsValid() {
		t.Error("expected IsValid() = false after Clear()")
	}
}

func TestMd5Hash(t *testing.T) {
	got := md5Hash("password")
	want := "5f4dcc3b5aa765d61d8327deb882cf99"

	if got != want {
		t.Errorf("md5Hash(\"password\") = %s; want %s", got, want)
	}
}

func TestPreparePasswordAlwaysMD5(t *testing.T) {
	for _, v := range []string{"modern", "hitemp", "legacy"} {
		m := &Manager{apiVersion: APIVersion(v), password: "testpassword"}
		if got, want := m.preparePassword(), md5Hash("testpassword"); got != want {
			t.Errorf("%s: preparePassword() = %q; want %q", v, got, want)
		}
	}
}

func TestFormatTokenHeader(t *testing.T) {
	store := NewTokenStore()
	m := &Manager{store: store}

	if got := m.FormatTokenHeader(); got != "" {
		t.Errorf("FormatTokenHeader() with no token = %q; want empty", got)
	}

	store.Set(&Token{
		Value:     "abc123",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	got := m.FormatTokenHeader()
	want := "abc123"
	if got != want {
		t.Errorf("FormatTokenHeader() = %q; want %q", got, want)
	}
}

func TestIsAuthenticated(t *testing.T) {
	store := NewTokenStore()
	m := &Manager{store: store}

	if m.IsAuthenticated() {
		t.Error("expected IsAuthenticated() = false with no token")
	}

	store.Set(&Token{
		Value:     "token",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	if !m.IsAuthenticated() {
		t.Error("expected IsAuthenticated() = true with valid token")
	}
}

func TestClear(t *testing.T) {
	store := NewTokenStore()
	m := &Manager{store: store}

	store.Set(&Token{
		Value:     "token",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	m.Clear()

	if m.IsAuthenticated() {
		t.Error("expected IsAuthenticated() = false after Clear()")
	}
}

func TestGetTokenValue(t *testing.T) {
	store := NewTokenStore()
	m := &Manager{store: store}

	if got := m.GetTokenValue(); got != "" {
		t.Errorf("GetTokenValue() with no token = %q; want empty", got)
	}

	store.Set(&Token{
		Value:     "mytoken",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	if got := m.GetTokenValue(); got != "mytoken" {
		t.Errorf("GetTokenValue() = %q; want %q", got, "mytoken")
	}
}
