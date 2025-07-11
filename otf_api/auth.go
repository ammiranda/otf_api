package otf_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Credentials struct {
	Username string `json:"USERNAME"`
	Password string `json:"PASSWORD"`
}

type AuthenticateRequest struct {
	AuthParameters Credentials `json:"AuthParameters"`
	AuthFlow       string      `json:"AuthFlow"`
	ClientID       string      `json:"ClientId"`
}

type IDToken struct {
	IDToken string `json:"IdToken"`
}

type AuthenticateResponse struct {
	AuthenticationResult IDToken `json:"AuthenticationResult"`
}

// Authenticate sends an authentication request to the OTF API which
// returns a JWT token when successful. The token will be set on
// the client instance use in multiple requests.
func (c *Client) Authenticate(
	ctx context.Context,
	username string,
	password string,
) (err error) {
	if c.NeedAuth() {
		reqBody := AuthenticateRequest{
			AuthParameters: Credentials{
				Username: username,
				Password: password,
			},
			AuthFlow: "USER_PASSWORD_AUTH",
			ClientID: getEnvVar("OTF_CLIENT_ID"),
		}

		jsonBody, marshalErr := json.Marshal(reqBody)
		if marshalErr != nil {
			err = fmt.Errorf("failed marshaling request body: %w", marshalErr)
			return
		}

		req, reqErr := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			c.AuthURL,
			bytes.NewBuffer(jsonBody))
		if reqErr != nil {
			err = fmt.Errorf("error preparing request: %w", reqErr)
			return
		}

		req.Header = http.Header{
			"Content-Type": {
				"application/x-amz-json-1.1",
			},
			"X-Amz-Target": {
				"AWSCognitoIdentityProviderService.InitiateAuth",
			},
		}

		res, httpErr := c.HTTPClient.Do(req)
		if httpErr != nil {
			err = fmt.Errorf("error authenticating: %w", httpErr)
			return
		}
		defer func() {
			if closeErr := res.Body.Close(); closeErr != nil {
				if err == nil {
					err = fmt.Errorf("error closing response body: %w", closeErr)
				} else {
					log.Printf("Failed to close response body for Authenticate (original error: %v): %v", err, closeErr)
				}
			}
		}()

		parsedResp := AuthenticateResponse{}
		decodeErr := json.NewDecoder(res.Body).Decode(&parsedResp)
		if decodeErr != nil {
			err = fmt.Errorf("error parsing response: %w", decodeErr)
			return
		}

		token := parsedResp.AuthenticationResult.IDToken
		c.Token = token
		c.HTTPClient.Transport = Chain(
			nil,
			AddHeader(http.CanonicalHeaderKey("authorization"), fmt.Sprintf("Bearer %s", token)),
			AddHeader(http.CanonicalHeaderKey("content-type"), "application/json"),
		)
	}

	return
}

// NeedAuth
func (c *Client) NeedAuth() bool {
	return c.Token == ""
}
