package otf_api

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
	configPath string
	origKC     func() bool
	origGet    func(string) (string, error)
	origSet    func(string, string) error
	origPath   func() (string, error)
}

func (s *ConfigSuite) SetupTest() {
	s.configPath = filepath.Join(s.T().TempDir(), "config.json")
	s.origKC = keychainAvailable
	s.origGet = keychainGet
	s.origSet = keychainSet
	s.origPath = GetConfigPath
	GetConfigPath = func() (string, error) { return s.configPath, nil }
}

func (s *ConfigSuite) TearDownTest() {
	keychainAvailable = s.origKC
	keychainGet = s.origGet
	keychainSet = s.origSet
	GetConfigPath = s.origPath
}

func (s *ConfigSuite) writeFile(cfg CLIConfig) {
	data, err := json.Marshal(cfg)
	s.Require().NoError(err)
	s.Require().NoError(os.WriteFile(s.configPath, data, 0600))
}

func (s *ConfigSuite) assertFile(cfg CLIConfig) {
	data, err := os.ReadFile(s.configPath)
	s.Require().NoError(err)
	var got CLIConfig
	s.Require().NoError(json.Unmarshal(data, &got))
	s.Equal(cfg, got)
}

func (s *ConfigSuite) noKeychain()   { keychainAvailable = func() bool { return false } }
func (s *ConfigSuite) withKeychain() { keychainAvailable = func() bool { return true } }

func (s *ConfigSuite) TestGetConfigPath() {
	// use real GetConfigPath
	GetConfigPath = s.origPath
	path, err := GetConfigPath()
	s.Require().NoError(err)
	s.Contains(path, "otf-cli")
	s.Contains(path, "config.json")
}

func (s *ConfigSuite) TestSaveAndLoadFromFile() {
	saved := CLIConfig{
		Token:              "tok",
		RefreshToken:       "ref",
		Timezone:           "America/Chicago",
		PreferredStudioIDs: []string{"studio-1", "studio-2"},
	}
	s.Require().NoError(saveToFile(saved))

	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Equal(saved, loaded)
}

func (s *ConfigSuite) TestLoadFromFile_NotExist() {
	cfg, err := loadFromFile()
	s.Require().NoError(err)
	s.Empty(cfg.Token)
}

func (s *ConfigSuite) TestLoadFromFile_ReadError() {
	s.Require().NoError(os.MkdirAll(s.configPath, 0755))
	_, err := loadFromFile()
	s.Require().Error(err)
	s.Contains(err.Error(), "reading")
}

func (s *ConfigSuite) TestLoadFromFile_InvalidJSON() {
	s.Require().NoError(os.WriteFile(s.configPath, []byte("not json"), 0600))
	_, err := loadFromFile()
	s.Require().Error(err)
	s.Contains(err.Error(), "parsing")
}

func (s *ConfigSuite) TestLoadFromFile_PathError() {
	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }
	_, err := loadFromFile()
	s.Error(err)
}

func (s *ConfigSuite) TestSaveToFile_PathError() {
	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }
	err := saveToFile(CLIConfig{Token: "tok"})
	s.Error(err)
}

func (s *ConfigSuite) TestLoadConfig_NoKeychain_FileExists() {
	s.noKeychain()
	s.writeFile(CLIConfig{Token: "t", RefreshToken: "r"})
	got, err := LoadConfig()
	s.Require().NoError(err)
	s.Equal("t", got.Token)
	s.Equal("r", got.RefreshToken)
}

func (s *ConfigSuite) TestLoadConfig_NoKeychain_FileNotExist() {
	s.noKeychain()
	got, err := LoadConfig()
	s.Require().NoError(err)
	s.Empty(got.Token)
}

func (s *ConfigSuite) TestLoadConfig_NoKeychain_FileError() {
	s.noKeychain()
	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }
	_, err := LoadConfig()
	s.Error(err)
}

func (s *ConfigSuite) TestLoadConfig_KeychainOverridesFile() {
	s.withKeychain()
	s.writeFile(CLIConfig{Token: "file-token"})
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

	got, err := LoadConfig()
	s.Require().NoError(err)
	s.Equal("kc-token", got.Token)
	s.Equal("kc-refresh", got.RefreshToken)
	s.Equal("America/Chicago", got.Timezone)
	s.Equal([]string{"studio-a"}, got.PreferredStudioIDs)
}

func (s *ConfigSuite) TestLoadConfig_KeychainFails_FallsBackToFile() {
	tests := []struct {
		name    string
		keyGet  func(string) (string, error)
	}{
		{"all fail", func(string) (string, error) { return "", errors.New("fail") }},
		{"partial fail", func(k string) (string, error) {
			if k == "token" { return "kc-token", nil }
			return "", errors.New("fail")
		}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.withKeychain()
			keychainGet = tt.keyGet
			s.writeFile(CLIConfig{Token: "file-token", Timezone: "America/Chicago"})

			got, err := LoadConfig()
			s.Require().NoError(err)
			s.Equal("file-token", got.Token)
			s.Equal("America/Chicago", got.Timezone)
		})
	}
}

func (s *ConfigSuite) TestLoadConfig_FileErrorWithKeychain_FallsBackToKeychain() {
	s.withKeychain()
	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }
	keychainGet = func(k string) (string, error) {
		switch k {
		case "token":
			return "kc-token", nil
		case "refresh_token":
			return "kc-refresh", nil
		}
		return "", nil
	}

	got, err := LoadConfig()
	s.Require().NoError(err)
	s.Equal("kc-token", got.Token)
	s.Equal("kc-refresh", got.RefreshToken)
}

func (s *ConfigSuite) TestSaveConfig_NoKeychain() {
	s.noKeychain()
	cfg := CLIConfig{Token: "saved-token"}
	s.Require().NoError(SaveConfig(cfg))
	s.assertFile(cfg)
}

func (s *ConfigSuite) TestSaveConfig_KeychainSuccess() {
	s.withKeychain()
	var stored map[string]string
	keychainSet = func(k, v string) error {
		if stored == nil {
			stored = map[string]string{}
		}
		stored[k] = v
		return nil
	}

	cfg := CLIConfig{Token: "t", RefreshToken: "r", Timezone: "z"}
	s.Require().NoError(SaveConfig(cfg))
	s.Equal("t", stored["token"])
	s.Equal("r", stored["refresh_token"])
	s.Equal("z", stored["timezone"])
}

func (s *ConfigSuite) TestSaveConfig_KeychainFails_FallsBackToFile() {
	s.withKeychain()
	keychainSet = func(string, string) error { return errors.New("fail") }

	cfg := CLIConfig{Token: "file-token"}
	s.Require().NoError(SaveConfig(cfg))
	s.assertFile(cfg)
}

func (s *ConfigSuite) TestStoreInKeychain_EmptyFields() {
	keychainSet = func(k, v string) error {
		s.T().Errorf("unexpected call keychainSet(%q, %q)", k, v)
		return nil
	}
	s.Require().NoError(storeInKeychain(CLIConfig{}))
}

func (s *ConfigSuite) TestStoreInKeychain_PreferredStudioIDs() {
	var capturedKey, capturedValue string
	keychainSet = func(k, v string) error {
		capturedKey, capturedValue = k, v
		return nil
	}
	s.Require().NoError(storeInKeychain(CLIConfig{PreferredStudioIDs: []string{"s1", "s2"}}))
	s.Equal("preferred_studio_ids", capturedKey)
	s.Equal(`["s1","s2"]`, capturedValue)
}

func (s *ConfigSuite) TestStoreInKeychain_TokenError() {
	keychainSet = func(k, v string) error { return errors.New("keychain error") }
	err := storeInKeychain(CLIConfig{Token: "t"})
	s.Error(err)
	s.Contains(err.Error(), "keychain token")
}

func (s *ConfigSuite) TestStoreInKeychain_RefreshTokenError() {
	keychainSet = func(k, v string) error {
		if k == "refresh_token" {
			return errors.New("refresh error")
		}
		return nil
	}
	err := storeInKeychain(CLIConfig{Token: "t", RefreshToken: "r"})
	s.Error(err)
	s.Contains(err.Error(), "keychain refresh_token")
}

func (s *ConfigSuite) TestStoreInKeychain_TimezoneError() {
	keychainSet = func(k, v string) error {
		if k == "timezone" {
			return errors.New("tz error")
		}
		return nil
	}
	err := storeInKeychain(CLIConfig{Token: "t", RefreshToken: "r", Timezone: "z"})
	s.Error(err)
	s.Contains(err.Error(), "keychain timezone")
}

func (s *ConfigSuite) TestStoreInKeychain_StudioIDsError() {
	keychainSet = func(k, v string) error {
		if k == "preferred_studio_ids" {
			return errors.New("ids error")
		}
		return nil
	}
	err := storeInKeychain(CLIConfig{Token: "t", RefreshToken: "r", Timezone: "z", PreferredStudioIDs: []string{"s1"}})
	s.Error(err)
	s.Contains(err.Error(), "keychain preferred_studio_ids")
}

func (s *ConfigSuite) TestLoadFromKeychain() {
	tests := []struct {
		name string
		get  func(string) (string, error)
		want CLIConfig
		err  bool
	}{
		{
			"all success",
			func(k string) (string, error) {
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
			},
			CLIConfig{Token: "t", RefreshToken: "r", Timezone: "z", PreferredStudioIDs: []string{"a", "b"}},
			false,
		},
		{
			"all fail",
			func(string) (string, error) { return "", errors.New("fail") },
			CLIConfig{},
			true,
		},
		{
			"invalid studio ids json",
			func(k string) (string, error) {
				if k == "preferred_studio_ids" {
					return "invalid", nil
				}
				return "", errors.New("fail")
			},
			CLIConfig{},
			true,
		},
		{
			"empty studio ids",
			func(k string) (string, error) {
				if k == "preferred_studio_ids" {
					return "", nil
				}
				return "val", nil
			},
			CLIConfig{Token: "val", RefreshToken: "val", Timezone: "val"},
			false,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			keychainGet = tt.get
			got, err := loadFromKeychain()
			if tt.err {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tt.want, got)
			}
		})
	}
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
