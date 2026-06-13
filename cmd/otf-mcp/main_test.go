package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ammiranda/otf_api/otf_api"
	"github.com/stretchr/testify/require"
)

func TestDetectLocation_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"lat": 30.25, "lon": -97.75, "city": "Austin", "regionName": "Texas", "country": "United States"}`)
	}))
	defer ts.Close()

	origURL := ipAPIURL
	ipAPIURL = ts.URL
	defer func() { ipAPIURL = origURL }()

	loc, err := detectLocation()
	require.NoError(t, err)
	require.Equal(t, 30.25, loc.Lat)
	require.Equal(t, -97.75, loc.Lon)
	require.Equal(t, "Austin", loc.City)
	require.Equal(t, "Texas", loc.Region)
	require.Equal(t, "United States", loc.Country)
}

func TestDetectLocation_HTTPError(t *testing.T) {
	origURL := ipAPIURL
	ipAPIURL = "http://127.0.0.1:1/"
	defer func() { ipAPIURL = origURL }()

	_, err := detectLocation()
	require.Error(t, err)
	require.Contains(t, err.Error(), "IP geolocation request failed")
}

func TestDetectLocation_NoCoordinates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"lat": 0, "lon": 0, "city": "", "regionName": "", "country": ""}`)
	}))
	defer ts.Close()

	origURL := ipAPIURL
	ipAPIURL = ts.URL
	defer func() { ipAPIURL = origURL }()

	_, err := detectLocation()
	require.Error(t, err)
	require.Contains(t, err.Error(), "IP geolocation returned no coordinates")
}

func TestDetectLocation_BadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `not json`)
	}))
	defer ts.Close()

	origURL := ipAPIURL
	ipAPIURL = ts.URL
	defer func() { ipAPIURL = origURL }()

	_, err := detectLocation()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse IP geolocation response")
}

type searchStudiosTestCase struct {
	name        string
	args        map[string]any
	configAllow *bool
	wantErr     bool
	errContains string
}

func TestSearchStudios_Consent(t *testing.T) {
	otfAPITS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{
			"data": {
				"studios": [],
				"pagination": {"pageIndex": 1, "pageSize": 200, "totalCount": 0, "totalPages": 1}
			}
		}`)
	}))
	defer otfAPITS.Close()

	client := otf_api.NewClient()
	client.BaseCOURL = otfAPITS.URL + "/"
	client.HTTPClient = otfAPITS.Client()

	tests := []searchStudiosTestCase{
		{
			name: "explicit lat/long no consent needed",
			args: map[string]any{"lat": 30.25, "long": -97.75},
		},
		{
			name:    "no lat/long no consent param, no config consent",
			args:    map[string]any{},
			wantErr: true,
		},
		{
			name: "allow_ip_location true in params",
			args: map[string]any{"allow_ip_location": true},
		},
		{
			name:        "allow_ip_location false in params",
			args:        map[string]any{"allow_ip_location": false},
			wantErr:     true,
			errContains: "requires your consent",
		},
		{
			name:        "no lat/long no param, config consent true",
			args:        map[string]any{},
			configAllow: boolPtr(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir := t.TempDir()
			configPath := filepath.Join(configDir, "config.json")

			origPath := otf_api.GetConfigPath
			origKeyringGet := otf_api.KeyringGet
			origKeyringSet := otf_api.KeyringSet
			otf_api.GetConfigPath = func() (string, error) { return configPath, nil }
			otf_api.KeyringGet = func(_, _ string) (string, error) { return "", fmt.Errorf("keyring unavailable") }
			otf_api.KeyringSet = func(_, _, _ string) error { return fmt.Errorf("keyring unavailable") }
			defer func() {
				otf_api.GetConfigPath = origPath
				otf_api.KeyringGet = origKeyringGet
				otf_api.KeyringSet = origKeyringSet
			}()

			if tt.configAllow != nil {
				cfg := otf_api.CLIConfig{AllowIPLocation: tt.configAllow}
				require.NoError(t, otf_api.SaveConfig(cfg))
			}

			origIPURL := ipAPIURL
			ipAPITS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"lat": 30.25, "lon": -97.75, "city": "Austin", "regionName": "Texas", "country": "United States"}`)
			}))
			ipAPIURL = ipAPITS.URL
			defer func() {
				ipAPITS.Close()
				ipAPIURL = origIPURL
			}()

			server := &MCPServer{ctx: context.Background()}
			rawArgs, _ := json.Marshal(tt.args)
			result := server.searchStudios(client, rawArgs)

			if tt.wantErr {
				require.True(t, result.IsError, "expected error but got success")
				if tt.errContains != "" {
					require.Contains(t, result.Content[0].Text, tt.errContains)
				}
			} else {
				require.False(t, result.IsError, "unexpected error: %v", result.Content)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
