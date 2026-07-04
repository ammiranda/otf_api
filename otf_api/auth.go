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

// ErrCodeAuthRequired is the JSON-RPC error code used by MCP to signal
// that authentication is needed before the request can be served.
const ErrCodeAuthRequired = -32001

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
func (c *Client) RefreshAuth(ctx context.Context) error {
	if c.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	result, err := c.authenticator.RefreshAuth(ctx, c.RefreshToken)
	if err != nil {
		return err
	}

	c.setAuthResult(result)
	return nil
}

// SetAuthenticator replaces the authenticator used by the client.
func (c *Client) SetAuthenticator(a Authenticator) {
	c.authenticator = a
}

func (c *Client) setAuthResult(result *AuthResult) {
	c.Token = result.Token
	c.TokenExpiry = time.Now().Add(result.ExpiresIn)
	if result.RefreshToken != "" {
		c.RefreshToken = result.RefreshToken
	}
}

// NeedAuth returns true if the client needs to authenticate. It checks
// whether the token is empty or expired (with a 5-minute buffer).
func (c *Client) NeedAuth() bool {
	if c.Token == "" {
		return true
	}
	if !c.TokenExpiry.IsZero() && time.Now().After(c.TokenExpiry.Add(-5*time.Minute)) {
		return true
	}
	return false
}

// SetToken directly sets the JWT on the client and configures the
// auth middleware transport if it hasn't been set up yet.
func (c *Client) SetToken(token string) {
	c.Token = token
	if exp, err := parseTokenExpiry(token); err == nil {
		c.TokenExpiry = exp
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if c.HTTPClient.Transport == nil {
		c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))
	}
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
