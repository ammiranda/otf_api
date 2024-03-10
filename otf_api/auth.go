package otf_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
