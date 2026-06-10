package otf_api

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func saveKeychainVars() func() {
	origAvailable := keychainAvailable
	origGet := keychainGet
	origSet := keychainSet
	return func() {
		keychainAvailable = origAvailable
		keychainGet = origGet
		keychainSet = origSet
	}
}

func saveConfigPath() func() {
	orig := GetConfigPath
	return func() { GetConfigPath = orig }
}

func tempConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.json")
}

func TestGetConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	require.NoError(t, err)
	assert.Contains(t, path, "otf-cli")
	assert.Contains(t, path, "config.json")
}

func TestSaveAndLoadFromFile(t *testing.T) {
	defer saveConfigPath()()

	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }

	saved := CLIConfig{
		Token:              "tok",
		RefreshToken:       "ref",
		Timezone:           "America/Chicago",
		PreferredStudioIDs: []string{"studio-1", "studio-2"},
	}
	err := saveToFile(saved)
	require.NoError(t, err)

	loaded, err := loadFromFile()
	require.NoError(t, err)
	assert.Equal(t, saved, loaded)
}

func TestLoadFromFile_NotExist(t *testing.T) {
	defer saveConfigPath()()

	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }

	config, err := loadFromFile()
	require.NoError(t, err)
	assert.Empty(t, config.Token)
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	defer saveConfigPath()()

	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }

	require.NoError(t, os.WriteFile(configPath, []byte("not json"), 0600))

	_, err := loadFromFile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing")
}

func TestLoadFromFile_GetConfigPathError(t *testing.T) {
	defer saveConfigPath()()

	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }

	_, err := loadFromFile()
	require.Error(t, err)
}

func TestSaveToFile_GetConfigPathError(t *testing.T) {
	defer saveConfigPath()()

	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }

	err := saveToFile(CLIConfig{Token: "tok"})
	require.Error(t, err)
}

func TestLoadConfig_NoKeychain_FileExists(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return false }
	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }

	saved := CLIConfig{Token: "file-token", RefreshToken: "file-refresh"}
	data, _ := json.Marshal(saved)
	require.NoError(t, os.WriteFile(configPath, data, 0600))

	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "file-token", config.Token)
	assert.Equal(t, "file-refresh", config.RefreshToken)
}

func TestLoadConfig_NoKeychain_FileNotExist(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return false }
	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }

	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Empty(t, config.Token)
}

func TestLoadConfig_NoKeychain_FileError(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return false }
	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }

	_, err := LoadConfig()
	require.Error(t, err)
}

func TestLoadConfig_KeychainOverridesFile(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return true }
	keychainGet = func(k string) (string, error) {
		switch k {
		case "token":
			return "kc-token", nil
		case "refresh_token":
			return "kc-refresh", nil
		case "timezone":
			return "America/Chicago", nil
		case "preferred_studio_ids":
			return `["studio-a"]`, nil
		}
		return "", nil
	}

	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }
	saved := CLIConfig{Token: "file-token"}
	data, _ := json.Marshal(saved)
	require.NoError(t, os.WriteFile(configPath, data, 0600))

	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "kc-token", config.Token)
	assert.Equal(t, "kc-refresh", config.RefreshToken)
	assert.Equal(t, "America/Chicago", config.Timezone)
	assert.Equal(t, []string{"studio-a"}, config.PreferredStudioIDs)
}

func TestLoadConfig_KeychainFails_FallsBackToFile(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return true }
	keychainGet = func(k string) (string, error) {
		return "", errors.New("keychain error")
	}

	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }
	saved := CLIConfig{Token: "file-token", Timezone: "America/Chicago"}
	data, _ := json.Marshal(saved)
	require.NoError(t, os.WriteFile(configPath, data, 0600))

	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "file-token", config.Token)
	assert.Equal(t, "America/Chicago", config.Timezone)
}

func TestLoadConfig_KeychainPartialFailure_FallsBackToFile(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return true }
	callCount := 0
	keychainGet = func(k string) (string, error) {
		callCount++
		if k == "token" {
			return "kc-token", nil
		}
		return "", errors.New("keychain error")
	}

	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }
	saved := CLIConfig{Token: "file-token", Timezone: "America/Chicago"}
	data, _ := json.Marshal(saved)
	require.NoError(t, os.WriteFile(configPath, data, 0600))

	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "file-token", config.Token)
	assert.Equal(t, "America/Chicago", config.Timezone)
}

func TestLoadConfig_FileErrorWithKeychain_FallsBackToKeychain(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return true }
	keychainGet = func(k string) (string, error) {
		switch k {
		case "token":
			return "kc-token", nil
		case "refresh_token":
			return "kc-refresh", nil
		case "timezone":
			return "", nil
		}
		return "", nil
	}

	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }

	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "kc-token", config.Token)
	assert.Equal(t, "kc-refresh", config.RefreshToken)
}

func TestSaveConfig_NoKeychain(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return false }
	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }

	cfg := CLIConfig{Token: "saved-token"}
	err := SaveConfig(cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var loaded CLIConfig
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Equal(t, "saved-token", loaded.Token)
}

func TestSaveConfig_KeychainSuccess(t *testing.T) {
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return true }
	var stored = map[string]string{}
	keychainSet = func(k, v string) error {
		stored[k] = v
		return nil
	}

	cfg := CLIConfig{
		Token:        "t",
		RefreshToken: "r",
		Timezone:     "z",
	}
	err := SaveConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, "t", stored["token"])
	assert.Equal(t, "r", stored["refresh_token"])
	assert.Equal(t, "z", stored["timezone"])
}

func TestSaveConfig_KeychainFails_FallsBackToFile(t *testing.T) {
	defer saveConfigPath()()
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return true }
	keychainSet = func(k, v string) error {
		return errors.New("keychain set failed")
	}

	configPath := tempConfigPath(t)
	GetConfigPath = func() (string, error) { return configPath, nil }

	cfg := CLIConfig{Token: "file-token"}
	err := SaveConfig(cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var loaded CLIConfig
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Equal(t, "file-token", loaded.Token)
}

func TestStoreInKeychain_EmptyFields(t *testing.T) {
	defer saveKeychainVars()()

	keychainSet = func(k, v string) error {
		t.Errorf("unexpected call to keychainSet(%q, %q)", k, v)
		return nil
	}

	err := storeInKeychain(CLIConfig{})
	require.NoError(t, err)
}

func TestStoreInKeychain_PreferredStudioIDs(t *testing.T) {
	defer saveKeychainVars()()

	var capturedKey, capturedValue string
	keychainSet = func(k, v string) error {
		capturedKey = k
		capturedValue = v
		return nil
	}

	err := storeInKeychain(CLIConfig{PreferredStudioIDs: []string{"s1", "s2"}})
	require.NoError(t, err)
	assert.Equal(t, "preferred_studio_ids", capturedKey)
	assert.Equal(t, `["s1","s2"]`, capturedValue)
}

func TestLoadFromKeychain_AllSuccess(t *testing.T) {
	defer saveKeychainVars()()

	keychainGet = func(k string) (string, error) {
		switch k {
		case "token":
			return "t", nil
		case "refresh_token":
			return "r", nil
		case "timezone":
			return "z", nil
		case "preferred_studio_ids":
			return `["a","b"]`, nil
		}
		return "", nil
	}

	config, err := loadFromKeychain()
	require.NoError(t, err)
	assert.Equal(t, "t", config.Token)
	assert.Equal(t, "r", config.RefreshToken)
	assert.Equal(t, "z", config.Timezone)
	assert.Equal(t, []string{"a", "b"}, config.PreferredStudioIDs)
}

func TestLoadFromKeychain_AllFail(t *testing.T) {
	defer saveKeychainVars()()

	keychainGet = func(k string) (string, error) {
		return "", errors.New("fail")
	}

	_, err := loadFromKeychain()
	require.Error(t, err)
}

func TestLoadFromKeychain_InvalidStudioIDsJSON(t *testing.T) {
	defer saveKeychainVars()()

	keychainGet = func(k string) (string, error) {
		if k == "preferred_studio_ids" {
			return "invalid json", nil
		}
		return "", errors.New("fail")
	}

	_, err := loadFromKeychain()
	require.Error(t, err)
}

func TestLoadFromKeychain_EmptyStudioIDs(t *testing.T) {
	defer saveKeychainVars()()

	keychainGet = func(k string) (string, error) {
		if k == "preferred_studio_ids" {
			return "", nil
		}
		return "val", nil
	}

	config, err := loadFromKeychain()
	require.NoError(t, err)
	assert.Nil(t, config.PreferredStudioIDs)
}

func TestSaveConfig_KeychainSetError(t *testing.T) {
	defer saveKeychainVars()()

	keychainAvailable = func() bool { return true }
	keychainSet = func(k, v string) error {
		return errors.New("keychain error")
	}

	cfg := CLIConfig{Token: "t"}
	err := storeInKeychain(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keychain token")
}
