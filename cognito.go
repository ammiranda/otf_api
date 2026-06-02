package otf_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type cognitoAuthenticator struct {
	authURL  string
	clientID string
}

// NewCognitoAuthenticator creates an Authenticator that uses AWS Cognito's
// InitiateAuth API with USER_PASSWORD_AUTH and REFRESH_TOKEN_AUTH flows.
func NewCognitoAuthenticator(authURL, clientID string) Authenticator {
	return &cognitoAuthenticator{
		authURL:  authURL,
		clientID: clientID,
	}
}

type cognitoCredentials struct {
	Username string `json:"USERNAME"`
	Password string `json:"PASSWORD"`
}

type cognitoAuthRequest struct {
	AuthParameters cognitoCredentials `json:"AuthParameters"`
	AuthFlow       string             `json:"AuthFlow"`
	ClientID       string             `json:"ClientId"`
}

type cognitoRefreshRequest struct {
	AuthParameters map[string]string `json:"AuthParameters"`
	AuthFlow       string            `json:"AuthFlow"`
	ClientID       string            `json:"ClientId"`
}

type cognitoAuthResult struct {
	IDToken      string `json:"IdToken"`
	RefreshToken string `json:"RefreshToken"`
	AccessToken  string `json:"AccessToken"`
	ExpiresIn    int    `json:"ExpiresIn"`
}

type cognitoInitiateAuthResponse struct {
	AuthenticationResult cognitoAuthResult `json:"AuthenticationResult"`
}

func (a *cognitoAuthenticator) Authenticate(ctx context.Context, credentials map[string]string) (*AuthResult, error) {
	reqBody := cognitoAuthRequest{
		AuthParameters: cognitoCredentials{
			Username: credentials["username"],
			Password: credentials["password"],
		},
		AuthFlow: "USER_PASSWORD_AUTH",
		ClientID: a.clientID,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.authURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error preparing request: %w", err)
	}

	req.Header = http.Header{
		"Content-Type": {"application/x-amz-json-1.1"},
		"X-Amz-Target": {"AWSCognitoIdentityProviderService.InitiateAuth"},
	}

	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("error authenticating: %w", err)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	parsedResp := cognitoInitiateAuthResponse{}
	if err := json.NewDecoder(res.Body).Decode(&parsedResp); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if parsedResp.AuthenticationResult.IDToken == "" {
		return nil, fmt.Errorf("authentication response did not contain an ID token")
	}

	result := &AuthResult{
		Token:        parsedResp.AuthenticationResult.IDToken,
		RefreshToken: parsedResp.AuthenticationResult.RefreshToken,
	}
	if parsedResp.AuthenticationResult.ExpiresIn > 0 {
		result.ExpiresIn = time.Duration(parsedResp.AuthenticationResult.ExpiresIn) * time.Second
	}

	return result, nil
}

func (a *cognitoAuthenticator) RefreshAuth(ctx context.Context, refreshToken string) (*AuthResult, error) {
	reqBody := cognitoRefreshRequest{
		AuthParameters: map[string]string{
			"REFRESH_TOKEN": refreshToken,
		},
		AuthFlow: "REFRESH_TOKEN_AUTH",
		ClientID: a.clientID,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling refresh request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.authURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error preparing refresh request: %w", err)
	}

	req.Header = http.Header{
		"Content-Type": {"application/x-amz-json-1.1"},
		"X-Amz-Target": {"AWSCognitoIdentityProviderService.InitiateAuth"},
	}

	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("error refreshing auth: %w", err)
	}
	defer res.Body.Close()

	parsedResp := cognitoInitiateAuthResponse{}
	if err := json.NewDecoder(res.Body).Decode(&parsedResp); err != nil {
		return nil, fmt.Errorf("error parsing refresh response: %w", err)
	}

	if parsedResp.AuthenticationResult.IDToken == "" {
		return nil, fmt.Errorf("refresh response did not contain an ID token")
	}

	result := &AuthResult{
		Token: parsedResp.AuthenticationResult.IDToken,
	}
	if parsedResp.AuthenticationResult.ExpiresIn > 0 {
		result.ExpiresIn = time.Duration(parsedResp.AuthenticationResult.ExpiresIn) * time.Second
	}

	return result, nil
}
