package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configFileName = "config.json"
	cliDirName     = "otf-cli"
)

type IPLocation struct {
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	City    string  `json:"city"`
	Region  string  `json:"regionName"`
	Country string  `json:"country"`
}

type CLIConfig struct {
	PreferredStudioIDs []string `json:"preferred_studio_ids,omitempty"`
	Timezone           string   `json:"timezone,omitempty"`
	Token              string   `json:"token,omitempty"`
	RefreshToken       string   `json:"refresh_token,omitempty"`
}

func getConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	cliConfigDir := filepath.Join(configDir, cliDirName)
	if err := os.MkdirAll(cliConfigDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create cli config directory %s: %w", cliConfigDir, err)
	}
	return filepath.Join(cliConfigDir, configFileName), nil
}

func loadConfig() (CLIConfig, error) {
	var config CLIConfig
	configFilePath, err := getConfigPath()
	if err != nil {
		return config, err
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, fmt.Errorf("failed to read config file %s: %w", configFilePath, err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config data from %s: %w", configFilePath, err)
	}
	return config, nil
}

func saveConfig(config CLIConfig) error {
	configFilePath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	if err := os.WriteFile(configFilePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
	}
	return nil
}
