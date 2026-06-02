package otf_api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_SetsHeaders(t *testing.T) {
	c := &Client{
		Token:      "test-token",
		HTTPClient: &http.Client{},
	}
	c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	_, err := c.HTTPClient.Do(req)
	require.NoError(t, err)
}

func TestAuthMiddleware_ReadsTokenDynamically(t *testing.T) {
	c := &Client{
		Token:      "initial-token",
		HTTPClient: &http.Client{},
	}
	c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))

	var capturedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	_, err := c.HTTPClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, "Bearer initial-token", capturedToken)

	c.Token = "updated-token"

	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	_, err = c.HTTPClient.Do(req2)
	require.NoError(t, err)
	assert.Equal(t, "Bearer updated-token", capturedToken)
}

func TestAuthMiddleware_RetriesOn401WithRefresh(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &Client{
		RefreshToken: "valid-refresh",
		TokenExpiry:  time.Now().Add(-1 * time.Hour),
		HTTPClient:   &http.Client{},
	}
	c.authenticator = &mockAuthenticator{
		refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
			c.Token = "refreshed-token"
			c.TokenExpiry = time.Now().Add(1 * time.Hour)
			return &AuthResult{
				Token:     "refreshed-token",
				ExpiresIn: 3600 * time.Second,
			}, nil
		},
	}
	c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	res, err := c.HTTPClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 2, requestCount)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "refreshed-token", c.Token)
}

func TestAuthMiddleware_DoesNotRetryOn401WithoutRefreshToken(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := &Client{
		Token:      "expired-token",
		HTTPClient: &http.Client{},
	}
	c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	res, err := c.HTTPClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

func TestAuthMiddleware_FailedRefreshStillReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := &Client{
		RefreshToken: "bad-refresh",
		HTTPClient:   &http.Client{},
	}
	c.authenticator = &mockAuthenticator{
		refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
			return nil, http.ErrAbortHandler
		},
	}
	c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	_, err := c.HTTPClient.Do(req)
	require.Error(t, err)
}

func TestAddHeader(t *testing.T) {
	c := &http.Client{}
	c.Transport = Chain(nil, AddHeader("X-Custom", "custom-value"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	_, err := c.Do(req)
	require.NoError(t, err)
}
