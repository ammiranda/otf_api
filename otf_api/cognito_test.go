package otf_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type CognitoSuite struct {
	suite.Suite
	server   *httptest.Server
	clientID string
}

func (s *CognitoSuite) SetupTest() {
	s.clientID = "test-client-id"
}

func (s *CognitoSuite) TearDownTest() {
	s.closeServer()
}

func (s *CognitoSuite) closeServer() {
	if s.server != nil {
		s.server.Close()
		s.server = nil
	}
}

func (s *CognitoSuite) newServer(handler http.HandlerFunc) {
	s.closeServer()
	// wrap to assert common Cognito headers
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodPost, r.Method)
		s.Equal("application/x-amz-json-1.1", r.Header.Get("Content-Type"))
		handler(w, r)
	})
	s.server = httptest.NewServer(wrapped)
}

func (s *CognitoSuite) auth() Authenticator {
	return NewCognitoAuthenticator(s.server.URL, s.clientID)
}

func (s *CognitoSuite) TestAuthenticate_Success() {
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("AWSCognitoIdentityProviderService.InitiateAuth", r.Header.Get("X-Amz-Target"))

		var req map[string]any
		s.Require().NoError(json.NewDecoder(r.Body).Decode(&req))
		s.Equal("USER_PASSWORD_AUTH", req["AuthFlow"])
		s.Equal(s.clientID, req["ClientId"])

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		s.Require().NoError(json.NewEncoder(w).Encode(cognitoInitiateAuthResponse{
			AuthenticationResult: cognitoAuthResult{
				IDToken:      "test-id-token",
				RefreshToken: "test-refresh-token",
				ExpiresIn:    3600,
			},
		}))
	})

	result, err := s.auth().Authenticate(context.Background(), map[string]string{
		"username": "user@test.com",
		"password": "pass123",
	})
	s.Require().NoError(err)
	s.Equal("test-id-token", result.Token)
	s.Equal("test-refresh-token", result.RefreshToken)
	s.Equal(3600*time.Second, result.ExpiresIn)
}

func (s *CognitoSuite) TestAuthenticate_MissingToken() {
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		s.Require().NoError(json.NewEncoder(w).Encode(cognitoInitiateAuthResponse{
			AuthenticationResult: cognitoAuthResult{},
		}))
	})

	_, err := s.auth().Authenticate(context.Background(), map[string]string{
		"username": "user@test.com",
		"password": "pass123",
	})
	s.Error(err)
}

func (s *CognitoSuite) TestAuthenticate_HTTPError() {
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		s.Require().NoError(json.NewEncoder(w).Encode(map[string]string{
			"__type":  "NotAuthorizedException",
			"message": "Incorrect username or password",
		}))
	})

	_, err := s.auth().Authenticate(context.Background(), map[string]string{
		"username": "user@test.com",
		"password": "wrong",
	})
	s.Error(err)
}

func (s *CognitoSuite) TestAuthenticate_CredentialsArePassed() {
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		s.Require().NoError(json.NewDecoder(r.Body).Decode(&req))
		params := req["AuthParameters"].(map[string]any)
		s.Equal("custom-user", params["USERNAME"])
		s.Equal("custom-pass", params["PASSWORD"])

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		s.Require().NoError(json.NewEncoder(w).Encode(cognitoInitiateAuthResponse{
			AuthenticationResult: cognitoAuthResult{IDToken: "token", ExpiresIn: 3600},
		}))
	})

	_, err := s.auth().Authenticate(context.Background(), map[string]string{
		"username": "custom-user",
		"password": "custom-pass",
	})
	s.NoError(err)
}

func (s *CognitoSuite) TestRefreshAuth_Success() {
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("AWSCognitoIdentityProviderService.InitiateAuth", r.Header.Get("X-Amz-Target"))

		var req map[string]any
		s.Require().NoError(json.NewDecoder(r.Body).Decode(&req))
		s.Equal("REFRESH_TOKEN_AUTH", req["AuthFlow"])
		params := req["AuthParameters"].(map[string]any)
		s.Equal("test-refresh-token", params["REFRESH_TOKEN"])

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		s.Require().NoError(json.NewEncoder(w).Encode(cognitoInitiateAuthResponse{
			AuthenticationResult: cognitoAuthResult{IDToken: "new-id-token", ExpiresIn: 3600},
		}))
	})

	result, err := s.auth().RefreshAuth(context.Background(), "test-refresh-token")
	s.Require().NoError(err)
	s.Equal("new-id-token", result.Token)
	s.Empty(result.RefreshToken)
}

func (s *CognitoSuite) TestRefreshAuth_Error() {
	s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		s.Require().NoError(json.NewEncoder(w).Encode(map[string]string{
			"__type":  "InvalidParameterException",
			"message": "Refresh Token has been revoked",
		}))
	})

	_, err := s.auth().RefreshAuth(context.Background(), "expired-refresh-token")
	s.Error(err)
}

func TestCognitoSuite(t *testing.T) {
	suite.Run(t, new(CognitoSuite))
}
