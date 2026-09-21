package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ammiranda/otf_api/otf_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	w.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

func TestEnsureClient_ReturnsCachedClient(t *testing.T) {
	s := &MCPServer{
		client: otf_api.NewClient(),
	}
	c, err := s.ensureClient()
	require.NoError(t, err)
	assert.Same(t, s.client, c)
}

func TestEnsureClient_ReturnsErrorWhenNoCredentials(t *testing.T) {
	s := &MCPServer{}

	savedPrompt := promptCredentials
	promptCredentials = func() (string, string, error) {
		return "", "", errors.New("no terminal available")
	}
	defer func() { promptCredentials = savedPrompt }()

	savedLoad := loadConfig
	loadConfig = func() (otf_api.CLIConfig, error) {
		return otf_api.CLIConfig{}, nil
	}
	defer func() { loadConfig = savedLoad }()

	_, err := s.ensureClient()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials available")
}

func TestEnsureClient_RestoresSessionFromConfig(t *testing.T) {
	savedLoad := loadConfig
	loadConfig = func() (otf_api.CLIConfig, error) {
		return otf_api.CLIConfig{Token: "cached-token"}, nil
	}
	defer func() { loadConfig = savedLoad }()

	s := &MCPServer{}
	c, err := s.ensureClient()
	require.NoError(t, err)
	assert.Equal(t, "cached-token", c.Token)
}

func TestRestoreSession(t *testing.T) {
	tests := []struct {
		name      string
		config    otf_api.CLIConfig
		cfgErr    error
		wantToken string
		wantRefresh string
	}{
		{
			name:        "restores token and refresh",
			config:      otf_api.CLIConfig{Token: "tok", RefreshToken: "ref"},
			cfgErr:      nil,
			wantToken:   "tok",
			wantRefresh: "ref",
		},
		{
			name:   "config error does nothing",
			config: otf_api.CLIConfig{},
			cfgErr: errors.New("config missing"),
		},
		{
			name:   "empty token does nothing",
			config: otf_api.CLIConfig{Token: ""},
			cfgErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &MCPServer{}
			client := otf_api.NewClient()
			s.restoreSession(client, tt.config, tt.cfgErr)
			assert.Equal(t, tt.wantToken, client.Token)
			assert.Equal(t, tt.wantRefresh, client.RefreshToken)
		})
	}
}

func TestTryRefreshAuth(t *testing.T) {
	tests := []struct {
		name       string
		client     *otf_api.Client
		want       bool
		wantToken  string
		wantSaved  bool
	}{
		{
			name: "already authenticated",
			client: func() *otf_api.Client {
				c := otf_api.NewClient()
				c.Token = "valid"
				c.TokenExpiry = time.Now().Add(1 * time.Hour)
				return c
			}(),
			want: true,
		},
		{
			name: "no refresh token",
			client: func() *otf_api.Client {
				c := otf_api.NewClient()
				c.Token = ""
				return c
			}(),
			want: false,
		},
		{
			name: "refresh succeeds",
			client: func() *otf_api.Client {
				c := otf_api.NewClient()
				c.RefreshToken = "refresh-1"
				c.SetAuthenticator(&mockAuthenticator{
					refreshAuthFunc: func(ctx context.Context, token string) (*otf_api.AuthResult, error) {
						return &otf_api.AuthResult{Token: "new-tok", ExpiresIn: 3600 * time.Second}, nil
					},
				})
				return c
			}(),
			want:      true,
			wantToken: "new-tok",
			wantSaved: true,
		},
		{
			name: "refresh fails",
			client: func() *otf_api.Client {
				c := otf_api.NewClient()
				c.RefreshToken = "bad-refresh"
				c.SetAuthenticator(&mockAuthenticator{
					refreshAuthFunc: func(ctx context.Context, token string) (*otf_api.AuthResult, error) {
						return nil, errors.New("refresh failed")
					},
				})
				return c
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var savedConfig otf_api.CLIConfig
			savedSave := saveConfig
			saveConfig = func(cfg otf_api.CLIConfig) error {
				savedConfig = cfg
				return nil
			}
			defer func() { saveConfig = savedSave }()

			s := &MCPServer{ctx: context.Background()}
			config := otf_api.CLIConfig{Token: "old", RefreshToken: tt.client.RefreshToken}
			got := s.tryRefreshAuth(tt.client, &config)
			assert.Equal(t, tt.want, got)
			if tt.wantToken != "" {
				assert.Equal(t, tt.wantToken, tt.client.Token)
			}
			if tt.wantSaved {
				assert.Equal(t, tt.wantToken, savedConfig.Token)
			}
		})
	}
}

func TestReauthWithStoredCredentials(t *testing.T) {
	tests := []struct {
		name      string
		config    otf_api.CLIConfig
		envUser   string
		envPass   string
		authErr   error
		wantErr   bool
		errMsg    string
		wantToken string
	}{
		{
			name:      "from config",
			config:    otf_api.CLIConfig{Username: "u", Password: "p"},
			wantToken: "auth-tok",
		},
		{
			name:      "from env vars",
			config:    otf_api.CLIConfig{},
			envUser:   "eu",
			envPass:   "ep",
			wantToken: "auth-tok",
		},
		{
			name:    "no credentials",
			config:  otf_api.CLIConfig{},
			wantErr: true,
			errMsg:  "no stored credentials available",
		},
		{
			name:    "auth fails",
			config:  otf_api.CLIConfig{Username: "u", Password: "p"},
			authErr: errors.New("bad creds"),
			wantErr: true,
			errMsg:  "re-authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			savedLoad := loadConfig
			loadConfig = func() (otf_api.CLIConfig, error) {
				return tt.config, nil
			}
			defer func() { loadConfig = savedLoad }()

			if tt.envUser != "" {
				t.Setenv("OTF_USERNAME", tt.envUser)
				t.Setenv("OTF_PASSWORD", tt.envPass)
			}

			client := otf_api.NewClient()
			client.SetAuthenticator(&mockAuthenticator{
				authenticateFunc: func(ctx context.Context, credentials map[string]string) (*otf_api.AuthResult, error) {
					if tt.authErr != nil {
						return nil, tt.authErr
					}
					return &otf_api.AuthResult{Token: "auth-tok", RefreshToken: "auth-ref", ExpiresIn: 3600 * time.Second}, nil
				},
			})

			s := &MCPServer{ctx: context.Background()}
			err := s.reauthWithStoredCredentials(context.Background(), client)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, client.Token)
		})
	}
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name       string
		config     otf_api.CLIConfig
		promptUser string
		promptPass string
		promptErr  error
		authErr    error
		wantErr    bool
		errMsg     string
		wantToken  string
	}{
		{
			name:       "from config",
			config:     otf_api.CLIConfig{Username: "u", Password: "p"},
			wantToken:  "auth-tok",
		},
		{
			name:       "from prompt",
			config:     otf_api.CLIConfig{},
			promptUser: "pu",
			promptPass: "pp",
			wantToken:  "auth-tok",
		},
		{
			name:      "prompt fails",
			config:    otf_api.CLIConfig{},
			promptErr: errors.New("no tty"),
			wantErr:   true,
			errMsg:    "no credentials available",
		},
		{
			name:      "auth fails",
			config:    otf_api.CLIConfig{Username: "u", Password: "p"},
			authErr:   errors.New("bad creds"),
			wantErr:   true,
			errMsg:    "authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			savedPrompt := promptCredentials
			promptCredentials = func() (string, string, error) {
				return tt.promptUser, tt.promptPass, tt.promptErr
			}
			defer func() { promptCredentials = savedPrompt }()

			client := otf_api.NewClient()
			client.SetAuthenticator(&mockAuthenticator{
				authenticateFunc: func(ctx context.Context, credentials map[string]string) (*otf_api.AuthResult, error) {
					if tt.authErr != nil {
						return nil, tt.authErr
					}
					return &otf_api.AuthResult{Token: "auth-tok", RefreshToken: "auth-ref", ExpiresIn: 3600 * time.Second}, nil
				},
			})

			s := &MCPServer{ctx: context.Background()}
			config := tt.config
			err := s.authenticate(client, &config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, client.Token)
		})
	}
}

func TestCredsFromConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  otf_api.CLIConfig
		wantU string
		wantP string
	}{
		{"both set", otf_api.CLIConfig{Username: "u", Password: "p"}, "u", "p"},
		{"missing password", otf_api.CLIConfig{Username: "u"}, "", ""},
		{"missing username", otf_api.CLIConfig{Password: "p"}, "", ""},
		{"empty", otf_api.CLIConfig{}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, p := credsFromConfig(tt.cfg)
			assert.Equal(t, tt.wantU, u)
			assert.Equal(t, tt.wantP, p)
		})
	}
}

func TestHandleToolCall_ReturnsErrorWhenNoAuth(t *testing.T) {
	savedPrompt := promptCredentials
	promptCredentials = func() (string, string, error) {
		return "", "", errors.New("no terminal available")
	}
	defer func() { promptCredentials = savedPrompt }()

	savedLoad := loadConfig
	loadConfig = func() (otf_api.CLIConfig, error) {
		return otf_api.CLIConfig{}, nil
	}
	defer func() { loadConfig = savedLoad }()

	out := captureStdout(t, func() {
		s := &MCPServer{}
		params := json.RawMessage(`{"name":"get_schedules","arguments":{}}`)
		s.handleToolCall(json.RawMessage(`1`), params)
	})

	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, errCodeAuthRequired, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Authentication required")
}

func TestHandleInitialize(t *testing.T) {
	out := captureStdout(t, func() {
		s := &MCPServer{}
		s.handleInitialize("init-1", nil)
	})

	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, "init-1", resp.ID)
	require.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2024-11-05", result["protocolVersion"])
}

func TestHandleToolsList(t *testing.T) {
	out := captureStdout(t, func() {
		s := &MCPServer{}
		s.handleToolsList("tools-1")
	})

	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, "tools-1", resp.ID)
	require.NotNil(t, resp.Result)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	tools, ok := result["tools"].([]any)
	require.True(t, ok)
	assert.Len(t, tools, 5)
}

func TestHandleToolCall_UnknownTool(t *testing.T) {
	client := otf_api.NewClient()
	client.Token = "test-token"
	client.TokenExpiry = time.Now().Add(1 * time.Hour)

	out := captureStdout(t, func() {
		s := &MCPServer{client: client}
		params := json.RawMessage(`{"name":"unknown_tool","arguments":{}}`)
		s.handleToolCall("id-1", params)
	})

	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, errCodeMethodNotFound, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Unknown tool")
}

func TestGetSchedules(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		serverResp []byte
		statusCode int
		wantText   string
		wantErr    bool
	}{
		{
			name:       "success with studio ids",
			args:       `{"studio_ids":"studio-1,studio-2"}`,
			serverResp: mustJSON(otf_api.StudioScheduleResponse{Items: []otf_api.StudioClass{{ID: "c1", Name: "Orange 60"}}}),
			statusCode: http.StatusOK,
			wantText:   "Orange 60",
		},
		{
			name:     "no studios configured",
			args:     `{}`,
			wantText: "No studio IDs provided",
			wantErr:  true,
		},
		{
			name:       "server error",
			args:       `{"studio_ids":"studio-1"}`,
			serverResp: []byte(`not json`),
			statusCode: http.StatusInternalServerError,
			wantText:   "Error fetching schedules",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handler http.HandlerFunc
			if tt.serverResp != nil {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
					w.Write(tt.serverResp)
				}))
				defer server.Close()
				handler = func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
					w.Write(tt.serverResp)
				}
				_ = handler
			}

			savedLoad := loadConfig
			loadConfig = func() (otf_api.CLIConfig, error) {
				return otf_api.CLIConfig{}, nil
			}
			defer func() { loadConfig = savedLoad }()

			client := otf_api.NewClient()
			if tt.serverResp != nil {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
					w.Write(tt.serverResp)
				}))
				defer server.Close()
				client.BaseIOURL = server.URL + "/"
			}
			client.SetToken("test-token")
			client.TokenExpiry = time.Now().Add(1 * time.Hour)

			s := &MCPServer{ctx: context.Background()}
			result := s.getSchedules(client, json.RawMessage(tt.args))
			assert.Equal(t, tt.wantErr, result.IsError)
			require.Len(t, result.Content, 1)
			assert.Contains(t, result.Content[0].Text, tt.wantText)
		})
	}
}

func TestListBookings(t *testing.T) {
	tests := []struct {
		name       string
		serverResp []byte
		statusCode int
		wantText   string
		wantErr    bool
	}{
		{
			name:       "success",
			serverResp: mustJSON(otf_api.BookingResponse{Items: []otf_api.BookingRequest{{ID: "b1", ServiceName: "Orange 60"}}}),
			statusCode: http.StatusOK,
			wantText:   "Orange 60",
		},
		{
			name:       "server error",
			serverResp: []byte(`{"error":"fail"}`),
			statusCode: http.StatusInternalServerError,
			wantText:   "Error fetching bookings",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write(tt.serverResp)
			}))
			defer server.Close()

			client := otf_api.NewClient()
			client.BaseIOURL = server.URL + "/"
			client.SetToken("test-token")
			client.TokenExpiry = time.Now().Add(1 * time.Hour)

			s := &MCPServer{ctx: context.Background()}
			result := s.listBookings(client)
			assert.Equal(t, tt.wantErr, result.IsError)
			require.Len(t, result.Content, 1)
			assert.Contains(t, result.Content[0].Text, tt.wantText)
		})
	}
}

func TestCancelBooking(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		statusCode int
		wantText   string
		wantErr    bool
	}{
		{
			name:       "success",
			args:       `{"booking_id":"book-1"}`,
			statusCode: http.StatusNoContent,
			wantText:   "Successfully canceled booking book-1",
		},
		{
			name:     "missing booking_id",
			args:     `{}`,
			wantText: "booking_id is required",
			wantErr:  true,
		},
		{
			name:       "server error",
			args:       `{"booking_id":"book-1"}`,
			statusCode: http.StatusInternalServerError,
			wantText:   "Error canceling booking",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := otf_api.NewClient()
			client.BaseIOURL = server.URL + "/"
			client.SetToken("test-token")
			client.TokenExpiry = time.Now().Add(1 * time.Hour)

			s := &MCPServer{ctx: context.Background()}
			result := s.cancelBooking(client, json.RawMessage(tt.args))
			assert.Equal(t, tt.wantErr, result.IsError)
			require.Len(t, result.Content, 1)
			assert.Contains(t, result.Content[0].Text, tt.wantText)
		})
	}
}

func TestBookClass(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		statusCode int
		wantText   string
		wantErr    bool
	}{
		{
			name:       "success",
			args:       `{"class_id":"class-1"}`,
			statusCode: http.StatusCreated,
			wantText:   "Successfully booked class class-1",
		},
		{
			name:       "waitlist",
			args:       `{"class_id":"class-1","waitlist":true}`,
			statusCode: http.StatusCreated,
			wantText:   "Successfully added to waitlist",
		},
		{
			name:     "missing class_id",
			args:     `{}`,
			wantText: "class_id is required",
			wantErr:  true,
		},
		{
			name:       "server error",
			args:       `{"class_id":"class-1"}`,
			statusCode: http.StatusBadRequest,
			wantText:   "Error booking class",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := otf_api.NewClient()
			client.BaseIOURL = server.URL + "/"
			client.SetToken("test-token")
			client.TokenExpiry = time.Now().Add(1 * time.Hour)

			s := &MCPServer{ctx: context.Background()}
			result := s.bookClass(client, json.RawMessage(tt.args))
			assert.Equal(t, tt.wantErr, result.IsError)
			require.Len(t, result.Content, 1)
			assert.Contains(t, result.Content[0].Text, tt.wantText)
		})
	}
}

func TestSearchStudios(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		lat        float64
		long       float64
		serverResp []byte
		statusCode int
		wantText   string
		wantErr    bool
	}{
		{
			name:       "success with lat long",
			args:       `{"lat":40.7128,"long":-74.006}`,
			serverResp: mustJSON(otf_api.ListStudiosResponse{Data: otf_api.Studios{Data: []otf_api.Studio{{StudioUUID: "s1", StudioName: "Downtown"}}}}),
			statusCode: http.StatusOK,
			wantText:   "Downtown",
		},
		{
			name:     "no location no consent",
			args:     `{}`,
			wantText: "Location detection from your IP requires your consent",
			wantErr:  true,
		},
		{
			name:       "server error",
			args:       `{"lat":40.7128,"long":-74.006}`,
			serverResp: []byte(`not json`),
			statusCode: http.StatusInternalServerError,
			wantText:   "Error searching studios",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write(tt.serverResp)
			}))
			defer server.Close()

			client := otf_api.NewClient()
			client.BaseCOURL = server.URL + "/"
			client.SetToken("test-token")
			client.TokenExpiry = time.Now().Add(1 * time.Hour)

			s := &MCPServer{ctx: context.Background()}
			result := s.searchStudios(client, json.RawMessage(tt.args))
			assert.Equal(t, tt.wantErr, result.IsError)
			require.Len(t, result.Content, 1)
			assert.Contains(t, result.Content[0].Text, tt.wantText)
		})
	}
}

func TestSearchStudios_IPLocation(t *testing.T) {
	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(IPLocation{Lat: 40.0, Lon: -74.0, City: "NYC", Region: "NY", Country: "US"})
	}))
	defer ipServer.Close()

	savedURL := ipAPIURL
	ipAPIURL = ipServer.URL + "/"
	defer func() { ipAPIURL = savedURL }()

	savedLoad := loadConfig
	loadConfig = func() (otf_api.CLIConfig, error) {
		return otf_api.CLIConfig{}, nil
	}
	defer func() { loadConfig = savedLoad }()

	studioServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(otf_api.ListStudiosResponse{Data: otf_api.Studios{Data: []otf_api.Studio{{StudioUUID: "s1", StudioName: "Downtown"}}}})
	}))
	defer studioServer.Close()

	client := otf_api.NewClient()
	client.BaseCOURL = studioServer.URL + "/"
	client.SetToken("test-token")
	client.TokenExpiry = time.Now().Add(1 * time.Hour)

	s := &MCPServer{ctx: context.Background()}
	result := s.searchStudios(client, json.RawMessage(`{"allow_ip_location":true}`))
	assert.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "Downtown")
	assert.Contains(t, result.Content[0].Text, "NYC")
}

func TestSearchStudios_IPLocationDenied(t *testing.T) {
	savedLoad := loadConfig
	loadConfig = func() (otf_api.CLIConfig, error) {
		return otf_api.CLIConfig{}, nil
	}
	defer func() { loadConfig = savedLoad }()

	client := otf_api.NewClient()
	client.SetToken("test-token")
	client.TokenExpiry = time.Now().Add(1 * time.Hour)

	s := &MCPServer{ctx: context.Background()}
	result := s.searchStudios(client, json.RawMessage(`{}`))
	assert.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "consent")
}

func TestWriteResult(t *testing.T) {
	out := captureStdout(t, func() {
		s := &MCPServer{}
		s.writeResult("req-1", map[string]string{"key": "value"})
	})

	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "req-1", resp.ID)
	assert.Nil(t, resp.Error)
}

func TestWriteError(t *testing.T) {
	out := captureStdout(t, func() {
		s := &MCPServer{}
		s.writeError("req-2", errCodeMethodNotFound, "not found")
	})

	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "req-2", resp.ID)
	require.NotNil(t, resp.Error)
	assert.Equal(t, errCodeMethodNotFound, resp.Error.Code)
	assert.Equal(t, "not found", resp.Error.Message)
}

func TestRun(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":3,"method":"unknown"}`,
		`not valid json`,
		``,
	}, "\n")

	r, w, err := os.Pipe()
	require.NoError(t, err)

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	os.Stdin = r

	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = outW

	done := make(chan struct{})
	go func() {
		w.Write([]byte(input))
		w.Close()
		s := &MCPServer{}
		s.Run()
		outW.Close()
		close(done)
	}()

	<-done
	os.Stdin = oldStdin
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, outR)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 4)

	var resp JSONRPCResponse
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &resp))
	assert.Equal(t, float64(1), resp.ID)
	assert.NotNil(t, resp.Result)

	require.NoError(t, json.Unmarshal([]byte(lines[1]), &resp))
	assert.Equal(t, float64(2), resp.ID)
	assert.NotNil(t, resp.Result)

	require.NoError(t, json.Unmarshal([]byte(lines[2]), &resp))
	assert.Equal(t, float64(3), resp.ID)
	assert.NotNil(t, resp.Error)

	require.NoError(t, json.Unmarshal([]byte(lines[3]), &resp))
	assert.Nil(t, resp.ID)
	assert.NotNil(t, resp.Error)
}

func TestGetSchedules_WithPreferredStudios(t *testing.T) {
	savedLoad := loadConfig
	loadConfig = func() (otf_api.CLIConfig, error) {
		return otf_api.CLIConfig{PreferredStudioIDs: []string{"pref-1", "pref-2"}}, nil
	}
	defer func() { loadConfig = savedLoad }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := r.URL.Query()[otf_api.StudioIDsQueryParamKey]
		assert.Equal(t, []string{"pref-1", "pref-2"}, ids)
		json.NewEncoder(w).Encode(otf_api.StudioScheduleResponse{Items: []otf_api.StudioClass{{ID: "c1", Name: "Orange 60"}}})
	}))
	defer server.Close()

	client := otf_api.NewClient()
	client.BaseIOURL = server.URL + "/"
	client.SetToken("test-token")
	client.TokenExpiry = time.Now().Add(1 * time.Hour)

	s := &MCPServer{ctx: context.Background()}
	result := s.getSchedules(client, json.RawMessage(`{}`))
	assert.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "Orange 60")
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

type mockAuthenticator struct {
	authenticateFunc func(ctx context.Context, credentials map[string]string) (*otf_api.AuthResult, error)
	refreshAuthFunc  func(ctx context.Context, refreshToken string) (*otf_api.AuthResult, error)
}

func (m *mockAuthenticator) Authenticate(ctx context.Context, credentials map[string]string) (*otf_api.AuthResult, error) {
	return m.authenticateFunc(ctx, credentials)
}

func (m *mockAuthenticator) RefreshAuth(ctx context.Context, refreshToken string) (*otf_api.AuthResult, error) {
	return m.refreshAuthFunc(ctx, refreshToken)
}
