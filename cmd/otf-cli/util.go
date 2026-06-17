package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ammiranda/otf_api/otf_api"
)

func setupClient(ctx context.Context) *otf_api.Client {
	apiClient := otf_api.NewClient()

	config, cfgErr := loadConfig()

	if cfgErr == nil && config.Token != "" {
		apiClient.SetToken(config.Token)
		apiClient.RefreshToken = config.RefreshToken
	}

	if apiClient.NeedAuth() {
		if apiClient.RefreshToken != "" {
			if err := apiClient.RefreshAuth(ctx); err == nil {
				config.Token = apiClient.Token
				config.RefreshToken = apiClient.RefreshToken
				if saveErr := saveConfig(config); saveErr != nil {
					log.Printf("Warning: could not cache refreshed token: %v", saveErr)
				}
				return apiClient
			}
		}

		username, password := credsFromConfig(config)
		if username == "" || password == "" {
			if err := survey.AskOne(&survey.Input{Message: "OTF Username:"}, &username, survey.WithValidator(survey.Required)); err != nil {
				log.Fatalf("Error reading username: %v", err)
			}
			if err := survey.AskOne(&survey.Password{Message: "OTF Password:"}, &password, survey.WithValidator(survey.Required)); err != nil {
				log.Fatalf("Error reading password: %v", err)
			}
		}

		if err := apiClient.Authenticate(ctx, username, password); err != nil {
			log.Fatalf("Error authenticating: %v", err)
		}

		config.Username = username
		config.Password = password
	}

	config.Token = apiClient.Token
	config.RefreshToken = apiClient.RefreshToken
	if saveErr := saveConfig(config); saveErr != nil {
		log.Printf("Warning: could not cache credentials: %v", saveErr)
	}

	return apiClient
}

func credsFromConfig(config otf_api.CLIConfig) (string, string) {
	if config.Username != "" && config.Password != "" {
		return config.Username, config.Password
	}
	return "", ""
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

func formatTime(t time.Time, config otf_api.CLIConfig) string {
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
