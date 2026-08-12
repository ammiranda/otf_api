package otf_api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Authenticator handles authentication with the OTF API provider.
// Implementations handle provider-specific protocols (Cognito, OAuth2, etc.).
type Authenticator interface {
	// Authenticate performs initial authentication with the given
	// credentials. The expected key/value pairs in the map are
	// implementation-specific (e.g., "username", "password").
	Authenticate(ctx context.Context, credentials map[string]string) (*AuthResult, error)

	// RefreshAuth uses a refresh token to obtain a new token.
	RefreshAuth(ctx context.Context, refreshToken string) (*AuthResult, error)
}

// AuthResult contains the tokens and metadata returned by an Authenticator.
type AuthResult struct {
	Token        string
	RefreshToken string
	ExpiresIn    time.Duration
}

type refreshCall struct {
	done chan struct{}
	err  error
}

// Authenticate performs initial authentication using username and password.
func (c *Client) Authenticate(
	ctx context.Context,
	username string,
	password string,
) error {
	if !c.NeedAuth() {
		return nil
	}

	result, err := c.authenticator.Authenticate(ctx, map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return err
	}

	c.setAuthResult(result)
	return nil
}

// RefreshAuth uses the stored refresh token to obtain a new ID token.
// Concurrent callers share a single refresh; only one call is made to the
// authenticator and the rest wait for its result.
func (c *Client) RefreshAuth(ctx context.Context) error {
	call, refreshToken := c.startRefresh()
	if refreshToken == "" {
		if call == nil {
			return fmt.Errorf("no refresh token available")
		}
		select {
		case <-call.done:
			return call.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	result, err := c.authenticator.RefreshAuth(ctx, refreshToken)
	c.endRefresh(call, result, err)
	return err
}

func (c *Client) startRefresh() (*refreshCall, string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.refreshing != nil {
		return c.refreshing, ""
	}
	if c.RefreshToken == "" {
		return nil, ""
	}
	call := &refreshCall{done: make(chan struct{})}
	c.refreshing = call
	return call, c.RefreshToken
}

func (c *Client) endRefresh(call *refreshCall, result *AuthResult, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshing = nil
	call.err = err
	if err == nil {
		c.setAuthResultLocked(result)
	}
	close(call.done)
}

// SetAuthenticator replaces the authenticator used by the client.
func (c *Client) SetAuthenticator(a Authenticator) {
	c.authenticator = a
}

func (c *Client) setAuthResult(result *AuthResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setAuthResultLocked(result)
}

func (c *Client) setAuthResultLocked(result *AuthResult) {
	c.Token = result.Token
	c.TokenExpiry = time.Now().Add(result.ExpiresIn)
	if exp, err := parseTokenExpiry(result.Token); err == nil {
		c.TokenExpiry = exp
	}
	if result.RefreshToken != "" {
		c.RefreshToken = result.RefreshToken
	}
}

// NeedAuth returns true if the client needs to authenticate. It checks
// whether the token is empty or expired (with a 5-minute buffer).
func (c *Client) NeedAuth() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Token == "" {
		return true
	}
	return !c.TokenExpiry.IsZero() && time.Now().After(c.TokenExpiry.Add(-5*time.Minute))
}

// SetToken directly sets the JWT on the client and configures the
// auth middleware transport if it hasn't been set up yet.
func (c *Client) SetToken(token string) {
	c.setToken(token)

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if c.HTTPClient.Transport == nil {
		c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))
	}
}

func (c *Client) setToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Token = token
	if exp, err := parseTokenExpiry(token); err == nil {
		c.TokenExpiry = exp
	}
}

func (c *Client) TokenValue() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Token
}

func (c *Client) HasRefreshToken() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.RefreshToken != ""
}

// parseTokenExpiry extracts the exp claim from a JWT without
// verifying the signature.
func parseTokenExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return time.Unix(claims.Exp, 0), nil
}
