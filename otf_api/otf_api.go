package otf_api

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// DefaultClientID is the Cognito App Client ID extracted from the
// OrangeTheory iOS app. Used for authentication if OTF_CLIENT_ID
// is not set.
const DefaultClientID = "65knvqta6p37efc2l3eh26pl5o"

type Client struct {
	BaseIOURL    string
	BaseCOURL    string
	AuthURL      string
	Token        string
	RefreshToken string
	TokenExpiry  time.Time
	HTTPClient   *http.Client
	MemberID     string

	authenticator Authenticator
}

// NewClient constructor that creates and returns a new instance
// of the OTF API client with a Cognito authenticator by default.
func NewClient() (*Client, error) {
	baseIOURL := os.Getenv("OTF_API_IO_BASE_URL")
	baseCOURL := os.Getenv("OTF_API_CO_BASE_URL")
	authURL := os.Getenv("OTF_AUTH_URL")
	clientID := os.Getenv("OTF_CLIENT_ID")
	if clientID == "" {
		clientID = DefaultClientID
	}

	if baseIOURL == "" || baseCOURL == "" || authURL == "" {
		return nil, fmt.Errorf("missing required env vars: OTF_API_IO_BASE_URL=%q OTF_API_CO_BASE_URL=%q OTF_AUTH_URL=%q", baseIOURL, baseCOURL, authURL)
	}

	c := &Client{
		BaseIOURL: baseIOURL,
		BaseCOURL: baseCOURL,
		AuthURL:   authURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		authenticator: NewCognitoAuthenticator(authURL, clientID),
	}
	c.HTTPClient.Transport = Chain(nil, AuthMiddleware(c))
	return c, nil
}
