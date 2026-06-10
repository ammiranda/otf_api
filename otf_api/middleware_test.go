package otf_api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MiddlewareSuite struct {
	suite.Suite
	server *httptest.Server
}

func (s *MiddlewareSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
		s.server = nil
	}
}

func (s *MiddlewareSuite) newServer(handler http.HandlerFunc) {
	if s.server != nil {
		s.server.Close()
	}
	s.server = httptest.NewServer(http.HandlerFunc(handler))
}

func (s *MiddlewareSuite) client() *Client {
	return &Client{
		HTTPClient: &http.Client{},
	}
}

func (s *MiddlewareSuite) clientWithToken(token string) *Client {
	c := s.client()
	c.Token = token
	c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))
	return c
}

func (s *MiddlewareSuite) TestSetsHeaders() {
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(s.T(), "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(s.T(), "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	})

	c := s.clientWithToken("test-token")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, s.server.URL, nil)
	_, err := c.HTTPClient.Do(req)
	s.NoError(err)
}

func (s *MiddlewareSuite) TestReadsTokenDynamically() {
	var capturedToken string
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})

	c := s.clientWithToken("initial-token")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, s.server.URL, nil)
	_, err := c.HTTPClient.Do(req)
	s.NoError(err)
	s.Equal("Bearer initial-token", capturedToken)

	c.Token = "updated-token"

	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, s.server.URL, nil)
	_, err = c.HTTPClient.Do(req2)
	s.NoError(err)
	s.Equal("Bearer updated-token", capturedToken)
}

func (s *MiddlewareSuite) TestRetriesOn401WithRefresh() {
	requestCount := 0
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	c := &Client{
		RefreshToken: "valid-refresh",
		TokenExpiry:  time.Now().Add(-1 * time.Hour),
		HTTPClient:   &http.Client{},
	}
	c.authenticator = &mockAuthenticator{
		refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
			c.Token = "refreshed-token"
			c.TokenExpiry = time.Now().Add(1 * time.Hour)
			return &AuthResult{Token: "refreshed-token", ExpiresIn: 3600 * time.Second}, nil
		},
	}
	c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, s.server.URL, nil)
	res, err := c.HTTPClient.Do(req)
	s.NoError(err)
	s.Equal(2, requestCount)
	s.Equal(http.StatusOK, res.StatusCode)
	s.Equal("refreshed-token", c.Token)
}

func (s *MiddlewareSuite) TestDoesNotRetryOn401WithoutRefreshToken() {
	requestCount := 0
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusUnauthorized)
	})

	c := s.clientWithToken("expired-token")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, s.server.URL, nil)
	res, err := c.HTTPClient.Do(req)
	s.NoError(err)
	s.Equal(1, requestCount)
	s.Equal(http.StatusUnauthorized, res.StatusCode)
}

func (s *MiddlewareSuite) TestFailedRefreshStillReturnsError() {
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

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

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, s.server.URL, nil)
	_, err := c.HTTPClient.Do(req)
	s.Error(err)
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

func TestMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareSuite))
}
