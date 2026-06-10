package otf_api

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

type CLIConfig struct {
	PreferredStudioIDs []string `json:"preferred_studio_ids,omitempty"`
	Timezone           string   `json:"timezone,omitempty"`
	Token              string   `json:"token,omitempty"`
	RefreshToken       string   `json:"refresh_token,omitempty"`
}

var GetConfigPath = func() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	cliConfigDir := filepath.Join(configDir, cliDirName)
	if err := os.MkdirAll(cliConfigDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create config directory %s: %w", cliConfigDir, err)
	}
	return filepath.Join(cliConfigDir, configFileName), nil
}

func loadFromFile() (CLIConfig, error) {
	var config CLIConfig
	path, err := GetConfigPath()
	if err != nil {
		return config, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("parsing %s: %w", path, err)
	}
	return config, nil
}

func saveToFile(config CLIConfig) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func LoadConfig() (CLIConfig, error) {
	config, kcErr := loadFromKeyring()
	if kcErr == nil {
		return config, nil
	}

	config, fileErr := loadFromFile()
	if fileErr != nil {
		return config, fmt.Errorf("keyring: %w; file: %w", kcErr, fileErr)
	}
	return config, nil
}

func loadFromKeyring() (CLIConfig, error) {
	var config CLIConfig
	data, err := keyringGet(keyringService, keyringUser)
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return config, fmt.Errorf("unmarshaling keyring config: %w", err)
	}
	return config, nil
}

func SaveConfig(config CLIConfig) error {
	if err := saveToKeyring(config); err == nil {
		return nil
	}
	return saveToFile(config)
}

func saveToKeyring(config CLIConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return keyringSet(keyringService, keyringUser, string(data))
}
