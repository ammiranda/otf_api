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

func TestAuthenticate_Success(t *testing.T) {
	c := &Client{
		HTTPClient: &http.Client{},
		authenticator: &mockAuthenticator{
			authenticateFunc: func(ctx context.Context, creds map[string]string) (*AuthResult, error) {
				assert.Equal(t, "user@test.com", creds["username"])
				assert.Equal(t, "pass", creds["password"])
				return &AuthResult{
					Token:        "test-token",
					RefreshToken: "test-refresh",
					ExpiresIn:    3600 * time.Second,
				}, nil
			},
		},
	}

	err := c.Authenticate(context.Background(), "user@test.com", "pass")
	require.NoError(t, err)
	assert.Equal(t, "test-token", c.Token)
	assert.Equal(t, "test-refresh", c.RefreshToken)
	assert.False(t, c.TokenExpiry.IsZero(), "TokenExpiry should be set")
}

func TestAuthenticate_NoAuthNeeded(t *testing.T) {
	called := false
	c := &Client{
		Token:       "valid-token",
		TokenExpiry: time.Now().Add(1 * time.Hour),
		HTTPClient:  &http.Client{},
		authenticator: &mockAuthenticator{
			authenticateFunc: func(ctx context.Context, creds map[string]string) (*AuthResult, error) {
				called = true
				return &AuthResult{Token: "new-token"}, nil
			},
		},
	}

	err := c.Authenticate(context.Background(), "user@test.com", "pass")
	require.NoError(t, err)
	assert.False(t, called, "Authenticate should not have been called when auth is not needed")
	assert.Equal(t, "valid-token", c.Token)
}

func TestAuthenticate_Error(t *testing.T) {
	expectedErr := errors.New("auth failed")
	c := &Client{
		HTTPClient: &http.Client{},
		authenticator: &mockAuthenticator{
			authenticateFunc: func(ctx context.Context, creds map[string]string) (*AuthResult, error) {
				return nil, expectedErr
			},
		},
	}

	err := c.Authenticate(context.Background(), "user@test.com", "pass")
	assert.ErrorIs(t, err, expectedErr)
}

func TestRefreshAuth_Success(t *testing.T) {
	c := &Client{
		RefreshToken: "old-refresh",
		HTTPClient:   &http.Client{},
		authenticator: &mockAuthenticator{
			refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
				assert.Equal(t, "old-refresh", token)
				return &AuthResult{
					Token:     "new-token",
					ExpiresIn: 3600 * time.Second,
				}, nil
			},
		},
	}

	err := c.RefreshAuth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "new-token", c.Token)
	assert.Equal(t, "old-refresh", c.RefreshToken, "RefreshToken should not change when not returned by authenticator")
}

func TestRefreshAuth_SuccessWithNewRefreshToken(t *testing.T) {
	c := &Client{
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
	}

	err := c.RefreshAuth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "new-refresh", c.RefreshToken)
}

func TestRefreshAuth_NoRefreshToken(t *testing.T) {
	c := &Client{
		HTTPClient: &http.Client{},
		authenticator: &mockAuthenticator{
			refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
				t.Error("should not be called")
				return nil, nil
			},
		},
	}

	err := c.RefreshAuth(context.Background())
	require.Error(t, err)
}

func TestRefreshAuth_Error(t *testing.T) {
	expectedErr := errors.New("refresh failed")
	c := &Client{
		RefreshToken: "some-refresh",
		HTTPClient:   &http.Client{},
		authenticator: &mockAuthenticator{
			refreshAuthFunc: func(ctx context.Context, token string) (*AuthResult, error) {
				return nil, expectedErr
			},
		},
	}

	err := c.RefreshAuth(context.Background())
	assert.ErrorIs(t, err, expectedErr)
}

func TestNeedAuth_EmptyToken(t *testing.T) {
	c := &Client{Token: ""}
	assert.True(t, c.NeedAuth())
}

func TestNeedAuth_ExpiredToken(t *testing.T) {
	c := &Client{
		Token:       "expired",
		TokenExpiry: time.Now().Add(-1 * time.Hour),
	}
	assert.True(t, c.NeedAuth())
}

func TestNeedAuth_ValidToken(t *testing.T) {
	c := &Client{
		Token:       "valid",
		TokenExpiry: time.Now().Add(1 * time.Hour),
	}
	assert.False(t, c.NeedAuth())
}

func TestNeedAuth_TokenWithoutExpiry(t *testing.T) {
	c := &Client{Token: "some-token"}
	assert.False(t, c.NeedAuth())
}

func TestNeedAuth_AboutToExpire(t *testing.T) {
	c := &Client{
		Token:       "about-to-expire",
		TokenExpiry: time.Now().Add(1 * time.Minute),
	}
	assert.True(t, c.NeedAuth())
}

func makeJWT(exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
	return fmt.Sprintf("%s.%s.%s", header, payload, sig)
}

func TestSetToken_ValidJWT(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour).Unix()
	token := makeJWT(exp)

	c := &Client{HTTPClient: &http.Client{}}
	c.SetToken(token)

	assert.Equal(t, token, c.Token)
	assert.False(t, c.TokenExpiry.IsZero(), "TokenExpiry should be set from JWT exp claim")
}

func TestSetToken_InvalidJWT(t *testing.T) {
	c := &Client{HTTPClient: &http.Client{}}
	c.SetToken("not-a-jwt")

	assert.Equal(t, "not-a-jwt", c.Token)
	assert.True(t, c.TokenExpiry.IsZero(), "TokenExpiry should be zero for unparseable JWT")
}

func TestSetToken_InitializesTransport(t *testing.T) {
	c := &Client{}
	c.SetToken("test-token")

	require.NotNil(t, c.HTTPClient, "HTTPClient should be initialized")
	assert.NotNil(t, c.HTTPClient.Transport, "Transport should be set up with auth middleware")
}

func TestSetToken_DoesNotReplaceExistingTransport(t *testing.T) {
	c := &Client{
		HTTPClient: &http.Client{},
	}
	c.HTTPClient.Transport = Chain(nil, AddHeader("X-Custom", "value"))
	c.SetToken("test-token")

	assert.NotNil(t, c.HTTPClient.Transport, "existing transport should not be replaced with nil")
}

func TestParseTokenExpiry_Valid(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour).Unix()
	token := makeJWT(exp)

	parsed, err := parseTokenExpiry(token)
	require.NoError(t, err)
	assert.Equal(t, exp, parsed.Unix())
}

func TestParseTokenExpiry_InvalidFormat(t *testing.T) {
	_, err := parseTokenExpiry("not-a-jwt")
	require.Error(t, err)
}

func TestParseTokenExpiry_InvalidBase64(t *testing.T) {
	token := "header.!!!invalid-base64!!.sig"
	_, err := parseTokenExpiry(token)
	require.Error(t, err)
}

func TestParseTokenExpiry_InvalidJSON(t *testing.T) {
	token := fmt.Sprintf("%s.%s.%s",
		base64.RawURLEncoding.EncodeToString([]byte("header")),
		base64.RawURLEncoding.EncodeToString([]byte("not-json")),
		base64.RawURLEncoding.EncodeToString([]byte("sig")),
	)
	_, err := parseTokenExpiry(token)
	require.Error(t, err)
}

func TestSetAuthenticator(t *testing.T) {
	c := &Client{}
	mock := &mockAuthenticator{}
	c.SetAuthenticator(mock)

	assert.Equal(t, mock, c.authenticator)
}
