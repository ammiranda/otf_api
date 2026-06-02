package otf_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCognitoAuthenticator_Authenticate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-amz-json-1.1", r.Header.Get("Content-Type"))
		assert.Equal(t, "AWSCognitoIdentityProviderService.InitiateAuth", r.Header.Get("X-Amz-Target"))

		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "USER_PASSWORD_AUTH", req["AuthFlow"])
		assert.Equal(t, "test-client-id", req["ClientId"])

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		json.NewEncoder(w).Encode(cognitoInitiateAuthResponse{
			AuthenticationResult: cognitoAuthResult{
				IDToken:      "test-id-token",
				RefreshToken: "test-refresh-token",
				ExpiresIn:    3600,
			},
		})
	}))
	defer server.Close()

	auth := NewCognitoAuthenticator(server.URL, "test-client-id")
	result, err := auth.Authenticate(context.Background(), map[string]string{
		"username": "user@test.com",
		"password": "pass123",
	})
	require.NoError(t, err)
	assert.Equal(t, "test-id-token", result.Token)
	assert.Equal(t, "test-refresh-token", result.RefreshToken)
	assert.Equal(t, 3600*time.Second, result.ExpiresIn)
}

func TestCognitoAuthenticator_Authenticate_MissingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		json.NewEncoder(w).Encode(cognitoInitiateAuthResponse{
			AuthenticationResult: cognitoAuthResult{},
		})
	}))
	defer server.Close()

	auth := NewCognitoAuthenticator(server.URL, "test-client-id")
	_, err := auth.Authenticate(context.Background(), map[string]string{
		"username": "user@test.com",
		"password": "pass123",
	})
	require.Error(t, err)
}

func TestCognitoAuthenticator_Authenticate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"__type":  "NotAuthorizedException",
			"message": "Incorrect username or password",
		})
	}))
	defer server.Close()

	auth := NewCognitoAuthenticator(server.URL, "test-client-id")
	_, err := auth.Authenticate(context.Background(), map[string]string{
		"username": "user@test.com",
		"password": "wrong",
	})
	require.Error(t, err)
}

func TestCognitoAuthenticator_RefreshAuth_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "REFRESH_TOKEN_AUTH", req["AuthFlow"])

		params := req["AuthParameters"].(map[string]any)
		assert.Equal(t, "test-refresh-token", params["REFRESH_TOKEN"])

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		json.NewEncoder(w).Encode(cognitoInitiateAuthResponse{
			AuthenticationResult: cognitoAuthResult{
				IDToken:   "new-id-token",
				ExpiresIn: 3600,
			},
		})
	}))
	defer server.Close()

	auth := NewCognitoAuthenticator(server.URL, "test-client-id")
	result, err := auth.RefreshAuth(context.Background(), "test-refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "new-id-token", result.Token)
	assert.Empty(t, result.RefreshToken)
}

func TestCognitoAuthenticator_RefreshAuth_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"__type":  "InvalidParameterException",
			"message": "Refresh Token has been revoked",
		})
	}))
	defer server.Close()

	auth := NewCognitoAuthenticator(server.URL, "test-client-id")
	_, err := auth.RefreshAuth(context.Background(), "expired-refresh-token")
	require.Error(t, err)
}

func TestCognitoAuthenticator_CredentialsArePassed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		params := req["AuthParameters"].(map[string]any)
		assert.Equal(t, "custom-user", params["USERNAME"])
		assert.Equal(t, "custom-pass", params["PASSWORD"])

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		json.NewEncoder(w).Encode(cognitoInitiateAuthResponse{
			AuthenticationResult: cognitoAuthResult{
				IDToken:   "token",
				ExpiresIn: 3600,
			},
		})
	}))
	defer server.Close()

	auth := NewCognitoAuthenticator(server.URL, "test-client-id")
	_, err := auth.Authenticate(context.Background(), map[string]string{
		"username": "custom-user",
		"password": "custom-pass",
	})
	require.NoError(t, err)
}
