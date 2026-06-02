package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ammiranda/otf-api"
)

func setupClient(ctx context.Context) *otf_api.Client {
	apiClient, err := otf_api.NewClient()
	if err != nil {
		log.Fatalf("Error creating API client: %v", err)
	}

	config, cfgErr := loadConfig()
	tryToken := cfgErr == nil && config.Token != ""

	if tryToken {
		apiClient.SetToken(config.Token)
		apiClient.RefreshToken = config.RefreshToken
		if !apiClient.NeedAuth() {
			return apiClient
		}
	}

	username := os.Getenv("OTF_USERNAME")
	password := os.Getenv("OTF_PASSWORD")
	if username == "" || password == "" {
		log.Fatal("OTF_USERNAME and OTF_PASSWORD environment variables must be set (or cache a token by running a command interactively first)")
	}

	if err := apiClient.Authenticate(ctx, username, password); err != nil {
		log.Fatalf("Error authenticating: %v", err)
	}

	config.Token = apiClient.Token
	config.RefreshToken = apiClient.RefreshToken
	if saveErr := saveConfig(config); saveErr != nil {
		log.Printf("Warning: could not cache token: %v", saveErr)
	}

	return apiClient
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Fatalf("error encoding JSON: %v", err)
	}
}

func findClassByID(classes []otf_api.StudioClass, id string) (otf_api.StudioClass, bool) {
	for _, c := range classes {
		if c.ID == id {
			return c, true
		}
	}
	return otf_api.StudioClass{}, false
}

func padOrTruncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		return string(runes[:width])
	} else if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
}

func formatTime(t time.Time, config CLIConfig) string {
	if config.Timezone == "" {
		return t.Format("3:04 PM MST")
	}

	loc, err := time.LoadLocation(config.Timezone)
	if err != nil {
		log.Printf("Warning: Invalid timezone %s, using local timezone: %v", config.Timezone, err)
		return t.Format("3:04 PM MST")
	}

	return t.In(loc).Format("3:04 PM MST")
}
