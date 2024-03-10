package otf_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
)

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

func (c *Client) Authenticate(
	ctx context.Context,
	username string,
	password string,
) error {
	if c.NeedAuth() {
		reqBody := AuthenticateRequest{
			AuthParameters: Credentials{
				Username: username,
				Password: password,
			},
			AuthFlow: "USER_PASSWORD_AUTH",
			ClientID: getEnvVar("OTF_CLIENT_ID"),
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed marshaling request body: %w", err)
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			c.AuthURL,
			bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("error preparing request: %w", err)
		}

		req.Header = http.Header{
			"Content-Type": {
				"application/x-amz-json-1.1",
			},
			"X-Amz-Target": {
				"AWSCognitoIdentityProviderService.InitiateAuth",
			},
		}

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("error authenticating: %w", err)
		}
		defer res.Body.Close()

		parsedResp := AuthenticateResponse{}
		err = json.NewDecoder(res.Body).Decode(&parsedResp)
		if err != nil {
			return fmt.Errorf("error parsing response: %w", err)
		}

		c.Token = parsedResp.AuthenticationResult.IDToken
	}

	return nil
}

// NeedAuth
func (c *Client) NeedAuth() bool {
	return c.Token == ""
}

// type ListStudiosResponse struct {
// 	Data
// 	Studios []Studio `json:""`
// }

func (c *Client) ListStudios(
	ctx context.Context,
	lat float64,
	long float64,
	distance float64,
) error {
	params := url.Values{
		"latitude": {
			"30.259373217464326",
		},
		"longitude": {
			"-97.70429793893",
		},
		"distance": {
			"50.0",
		},
	}
	// params.Add("latitude", "30.259373217464326")
	// params.Add("longitude", "-97.70429793893")
	// params.Add("distance", "30.0")

	url := c.BaseCOURL + "studios?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header = http.Header{
		"Content-Type": {
			"application/json",
		},
		"Authorization": {
			c.Token,
		},
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))

	return nil
}

func (c *Client) GetStudiosSchedule(
	ctx context.Context,
	studioIDs []string,
) (StudioScheduleResponse, error) {
	params := url.Values{
		"studio_ids": studioIDs,
	}

	url := c.BaseIOURL + "classes?" + params.Encode()
	fmt.Println(url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return StudioScheduleResponse{}, err
	}

	req.Header = http.Header{
		"Content-Type": {
			"application/json",
		},
		"Authorization": {
			c.Token,
		},
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return StudioScheduleResponse{}, err
	}
	defer res.Body.Close()

	parsedResp := StudioScheduleResponse{}
	err = json.NewDecoder(res.Body).Decode(&parsedResp)
	if err != nil {
		return StudioScheduleResponse{}, fmt.Errorf("error parsing response: %w", err)
	}

	return parsedResp, nil
}
