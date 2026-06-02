package otf_api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

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

func getEnvVar(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(err)
	}

	return os.Getenv(key)
}

// NewClient constructor that creates and returns a new instance
// of the OTF API client with a Cognito authenticator by default.
func NewClient() (*Client, error) {
	baseIOURL := getEnvVar("OTF_API_IO_BASE_URL")
	baseCOURL := getEnvVar("OTF_API_CO_BASE_URL")
	authURL := getEnvVar("OTF_AUTH_URL")
	clientID := getEnvVar("OTF_CLIENT_ID")

	if baseIOURL == "" || baseCOURL == "" || authURL == "" {
		return nil, fmt.Errorf("base urls not configured correctly")
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
