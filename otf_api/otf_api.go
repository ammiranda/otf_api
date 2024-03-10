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
	BaseIOURL  string
	BaseCOURL  string
	AuthURL    string
	Token      string
	HTTPClient *http.Client
}

func getEnvVar(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(err)
	}

	return os.Getenv(key)
}

// NewClient
func NewClient() (*Client, error) {
	baseIOURL := getEnvVar("OTF_API_IO_BASE_URL")
	baseCOURL := getEnvVar("OTF_API_CO_BASE_URL")
	authURL := getEnvVar("OTF_AUTH_URL")

	if baseIOURL == "" || baseCOURL == "" || authURL == "" {
		return nil, fmt.Errorf("base urls not configured correctly")
	}

	return &Client{
		BaseIOURL: baseIOURL,
		BaseCOURL: baseCOURL,
		AuthURL:   authURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}
