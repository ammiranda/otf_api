package otf_api

import (
	"encoding/json"
	"errors"
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

func GetConfigPath() (string, error) {
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
	config, fileErr := loadFromFile()
	hasKeychain := keychainAvailable()

	if fileErr != nil && !hasKeychain {
		return config, fileErr
	}

	if hasKeychain {
		kcConfig, err := loadFromKeychain()
		if err == nil {
			config = kcConfig
		}
	}

	return config, nil
}

func loadFromKeychain() (CLIConfig, error) {
	var config CLIConfig
	var errs []error

	if token, err := keychainGet("token"); err != nil {
		errs = append(errs, err)
	} else {
		config.Token = token
	}

	if refresh, err := keychainGet("refresh_token"); err != nil {
		errs = append(errs, err)
	} else {
		config.RefreshToken = refresh
	}

	if tz, err := keychainGet("timezone"); err != nil {
		errs = append(errs, err)
	} else {
		config.Timezone = tz
	}

	if raw, err := keychainGet("preferred_studio_ids"); err != nil {
		errs = append(errs, err)
	} else if raw != "" {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			errs = append(errs, err)
		} else {
			config.PreferredStudioIDs = ids
		}
	}

	if len(errs) > 0 {
		return config, fmt.Errorf("keychain: %w", errors.Join(errs...))
	}
	return config, nil
}

func SaveConfig(config CLIConfig) error {
	if keychainAvailable() {
		if err := storeInKeychain(config); err == nil {
			return nil
		}
	}

	return saveToFile(config)
}

func storeInKeychain(config CLIConfig) error {
	if config.Token != "" {
		if err := keychainSet("token", config.Token); err != nil {
			return fmt.Errorf("keychain token: %w", err)
		}
	}
	if config.RefreshToken != "" {
		if err := keychainSet("refresh_token", config.RefreshToken); err != nil {
			return fmt.Errorf("keychain refresh_token: %w", err)
		}
	}
	if config.Timezone != "" {
		if err := keychainSet("timezone", config.Timezone); err != nil {
			return fmt.Errorf("keychain timezone: %w", err)
		}
	}
	if len(config.PreferredStudioIDs) > 0 {
		idsJSON, err := json.Marshal(config.PreferredStudioIDs)
		if err != nil {
			return fmt.Errorf("marshaling studio IDs: %w", err)
		}
		if err := keychainSet("preferred_studio_ids", string(idsJSON)); err != nil {
			return fmt.Errorf("keychain preferred_studio_ids: %w", err)
		}
	}
	return nil
}
