package otf_api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
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

func generateAWSSignature(accessKey, secretKey, region, service, method, path string, headers map[string]string, payload []byte) string {
	// Get the current timestamp
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	// Create the canonical request
	canonicalHeaders := make([]string, 0)
	for k, v := range headers {
		canonicalHeaders = append(canonicalHeaders, fmt.Sprintf("%s:%s", strings.ToLower(k), v))
	}
	sort.Strings(canonicalHeaders)

	payloadHash := sha256.Sum256(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		path,
		"", // canonical query string
		strings.Join(canonicalHeaders, "\n")+"\n",
		strings.Join([]string{"content-type", "host", "x-amz-date"}, ";"),
		hex.EncodeToString(payloadHash[:]),
	)

	// Create the string to sign
	scope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	canonicalRequestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		scope,
		hex.EncodeToString(canonicalRequestHash[:]),
	)

	// Calculate the signature
	kDate := hmac.New(sha256.New, []byte("AWS4"+secretKey))
	kDate.Write([]byte(dateStamp))
	kRegion := hmac.New(sha256.New, kDate.Sum(nil))
	kRegion.Write([]byte(region))
	kService := hmac.New(sha256.New, kRegion.Sum(nil))
	kService.Write([]byte(service))
	kSigning := hmac.New(sha256.New, kService.Sum(nil))
	kSigning.Write([]byte("aws4_request"))
	kSigning.Write([]byte(stringToSign))
	signature := hex.EncodeToString(kSigning.Sum(nil))

	return signature
}
