package otf_api

import (
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

func NewClient() *Client {
	baseIOURL := os.Getenv("OTF_API_IO_BASE_URL")
	if baseIOURL == "" {
		baseIOURL = "https://api.orangetheory.io/v1/"
	}
	baseCOURL := os.Getenv("OTF_API_CO_BASE_URL")
	if baseCOURL == "" {
		baseCOURL = "https://api.orangetheory.co/mobile/v1/"
	}
	authURL := os.Getenv("OTF_AUTH_URL")
	if authURL == "" {
		authURL = "https://cognito-idp.us-east-1.amazonaws.com/"
	}
	clientID := os.Getenv("OTF_CLIENT_ID")
	if clientID == "" {
		clientID = DefaultClientID
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
	return c
}
