package otf_api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuthenticator struct {
	authenticateFunc func(ctx context.Context, credentials map[string]string) (*AuthResult, error)
	refreshAuthFunc  func(ctx context.Context, refreshToken string) (*AuthResult, error)
}

var _ Authenticator = (*mockAuthenticator)(nil)

func (m *mockAuthenticator) Authenticate(ctx context.Context, credentials map[string]string) (*AuthResult, error) {
	return m.authenticateFunc(ctx, credentials)
}

func (m *mockAuthenticator) RefreshAuth(ctx context.Context, refreshToken string) (*AuthResult, error) {
	return m.refreshAuthFunc(ctx, refreshToken)
}

func TestNeedAuth(t *testing.T) {
	tests := []struct {
		name string
		c    *Client
		want bool
	}{
		{"empty token", &Client{Token: ""}, true},
		{"expired token", &Client{Token: "e", TokenExpiry: time.Now().Add(-1 * time.Hour)}, true},
		{"valid token", &Client{Token: "v", TokenExpiry: time.Now().Add(1 * time.Hour)}, false},
		{"no expiry", &Client{Token: "s"}, false},
		{"about to expire", &Client{Token: "a", TokenExpiry: time.Now().Add(1 * time.Minute)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.c.NeedAuth())
		})
	}
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name    string
		client  *Client
		creds   func(c *Client) (string, string)
		wantTok string
		wantRef string
		err     bool
	}{
		{
			"success",
			&Client{
				HTTPClient: &http.Client{},
				authenticator: &mockAuthenticator{
					authenticateFunc: func(ctx context.Context, creds map[string]string) (*AuthResult, error) {
						return &AuthResult{
							Token:        "test-token",
							RefreshToken: "test-refresh",
							ExpiresIn:    3600 * time.Second,
						}, nil
					},
				},
			},
			func(c *Client) (string, string) { return "user@test.com", "pass" },
			"test-token",
			"test-refresh",
			false,
		},
		{
			"no auth needed",
			&Client{
				Token:       "valid-token",
				TokenExpiry: time.Now().Add(1 * time.Hour),
				HTTPClient:  &http.Client{},
				authenticator: &mockAuthenticator{
					authenticateFunc: func(ctx context.Context, creds map[string]string) (*AuthResult, error) {
						t.Error("should not be called")
						return &AuthResult{Token: "new-token"}, nil
					},
				},
			},
			func(c *Client) (string, string) { return "user@test.com", "pass" },
			"valid-token",
			"",
			false,
		},
		{
			"error",
			&Client{
				HTTPClient: &http.Client{},
				authenticator: &mockAuthenticator{
					authenticateFunc: func(ctx context.Context, creds map[string]string) (*AuthResult, error) {
						return nil, errors.New("auth failed")
					},
				},
			},
			func(c *Client) (string, string) { return "user@test.com", "pass" },
			"",
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, p := tt.creds(tt.client)
			err := tt.client.Authenticate(context.Background(), u, p)
			if tt.err {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantTok, tt.client.Token)
			assert.Equal(t, tt.wantRef, tt.client.RefreshToken)
		})
	}
}

func TestRefreshAuth(t *testing.T) {
	tests := []struct {
		name       string
		client     *Client
		wantTok    string
		wantRef    string
		err        bool
	}{
		{
			"success",
			&Client{
				RefreshToken: "old-refresh",
				HTTPClient:   &http.Client{},
				authenticator: &mockAuthenticator{
					refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
						return &AuthResult{Token: "new-token", ExpiresIn: 3600 * time.Second}, nil
					},
				},
			},
			"new-token",
			"old-refresh",
			false,
		},
		{
			"success with new refresh token",
			&Client{
				RefreshToken: "old-refresh",
				HTTPClient:   &http.Client{},
				authenticator: &mockAuthenticator{
					refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
						return &AuthResult{
							Token:        "new-token",
							RefreshToken: "new-refresh",
							ExpiresIn:    3600 * time.Second,
						}, nil
					},
				},
			},
			"new-token",
			"new-refresh",
			false,
		},
		{
			"no refresh token",
			&Client{
				HTTPClient: &http.Client{},
				authenticator: &mockAuthenticator{
					refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
						t.Error("should not be called")
						return nil, nil
					},
				},
			},
			"",
			"",
			true,
		},
		{
			"error",
			&Client{
				RefreshToken: "some-refresh",
				HTTPClient:   &http.Client{},
				authenticator: &mockAuthenticator{
					refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
						return nil, errors.New("refresh failed")
					},
				},
			},
			"",
			"some-refresh",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.RefreshAuth(context.Background())
			if tt.err {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantTok, tt.client.Token)
			assert.Equal(t, tt.wantRef, tt.client.RefreshToken)
		})
	}
}

func TestSetToken(t *testing.T) {
	tests := []struct {
		name       string
		client     *Client
		token      string
		wantExpiry bool
		wantErr    bool
	}{
		{
			"valid JWT",
			&Client{HTTPClient: &http.Client{}},
			makeJWT(time.Now().Add(1 * time.Hour).Unix()),
			true,
			false,
		},
		{
			"invalid JWT",
			&Client{HTTPClient: &http.Client{}},
			"not-a-jwt",
			false,
			false,
		},
		{
			"initializes transport",
			&Client{},
			"test-token",
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.client.SetToken(tt.token)
			assert.Equal(t, tt.token, tt.client.Token)
			if tt.wantExpiry {
				assert.False(t, tt.client.TokenExpiry.IsZero(), "TokenExpiry should be set")
			} else if tt.name != "initializes transport" {
				assert.True(t, tt.client.TokenExpiry.IsZero(), "TokenExpiry should be zero")
			}
			if tt.name == "initializes transport" {
				require.NotNil(t, tt.client.HTTPClient, "HTTPClient should be initialized")
				assert.NotNil(t, tt.client.HTTPClient.Transport, "Transport should be set up")
			}
		})
	}
}

func TestSetToken_DoesNotReplaceExistingTransport(t *testing.T) {
	c := &Client{HTTPClient: &http.Client{}}
	c.HTTPClient.Transport = Chain(nil, AddHeader("X-Custom", "value"))
	c.SetToken("test-token")
	assert.NotNil(t, c.HTTPClient.Transport, "existing transport should not be replaced with nil")
}

func TestParseTokenExpiry(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour).Unix()

	tests := []struct {
		name  string
		token string
		err   bool
	}{
		{"valid", makeJWT(exp), false},
		{"invalid format", "not-a-jwt", true},
		{"invalid base64", "header.!!!invalid-base64!!.sig", true},
		{"invalid json", fmt.Sprintf("%s.%s.%s",
			base64.RawURLEncoding.EncodeToString([]byte("header")),
			base64.RawURLEncoding.EncodeToString([]byte("not-json")),
			base64.RawURLEncoding.EncodeToString([]byte("sig")),
		), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseTokenExpiry(tt.token)
			if tt.err {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, exp, parsed.Unix())
		})
	}
}

func makeJWT(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
	return fmt.Sprintf("%s.%s.%s", header, payload, sig)
}

func TestSetAuthenticator(t *testing.T) {
	c := &Client{}
	mock := &mockAuthenticator{}
	c.SetAuthenticator(mock)
	assert.Equal(t, mock, c.authenticator)
}
