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
	origGet    func(string, string) (string, error)
	origSet    func(string, string, string) error
	origPath   func() (string, error)
}

func (s *ConfigSuite) SetupTest() {
	s.configPath = filepath.Join(s.T().TempDir(), "config.json")
	s.origGet = keyringGet
	s.origSet = keyringSet
	s.origPath = GetConfigPath
	GetConfigPath = func() (string, error) { return s.configPath, nil }
	keyringGet = func(_, _ string) (string, error) { return "", errors.New("keyring unavailable") }
	keyringSet = func(_, _, _ string) error { return errors.New("keyring unavailable") }
}

func (s *ConfigSuite) TearDownTest() {
	keyringGet = s.origGet
	keyringSet = s.origSet
	GetConfigPath = s.origPath
}

func (s *ConfigSuite) withKeyring() {
	keyringGet = func(_, _ string) (string, error) {
		return "", errors.New("keyring unavailable")
	}
	keyringSet = func(_, _, _ string) error {
		return errors.New("keyring unavailable")
	}
}

func (s *ConfigSuite) withKeyringData(cfg CLIConfig) {
	data, _ := json.Marshal(cfg)
	keyringGet = func(_, _ string) (string, error) {
		return string(data), nil
	}
	keyringSet = func(_, _, _ string) error {
		return nil
	}
}

func (s *ConfigSuite) withKeyringSet() {
	keyringSet = func(_, _, _ string) error { return nil }
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

func (s *ConfigSuite) TestGetConfigPath() {
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

func (s *ConfigSuite) TestLoadConfig_KeyringSuccess() {
	s.withKeyringData(CLIConfig{Token: "kr-token", RefreshToken: "kr-refresh"})
	s.writeFile(CLIConfig{Token: "file-token"})

	got, err := LoadConfig()
	s.Require().NoError(err)
	s.Equal("kr-token", got.Token)
	s.Equal("kr-refresh", got.RefreshToken)
}

func (s *ConfigSuite) TestLoadConfig_KeyringFails_FallsBackToFile() {
	s.withKeyring()
	s.writeFile(CLIConfig{Token: "file-token", Timezone: "America/Chicago"})

	got, err := LoadConfig()
	s.Require().NoError(err)
	s.Equal("file-token", got.Token)
	s.Equal("America/Chicago", got.Timezone)
}

func (s *ConfigSuite) TestLoadConfig_BothFail() {
	s.withKeyring()
	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }

	_, err := LoadConfig()
	s.Error(err)
	s.Contains(err.Error(), "keyring")
	s.Contains(err.Error(), "file")
}

func (s *ConfigSuite) TestSaveConfig_KeyringSuccess() {
	s.withKeyringSet()
	cfg := CLIConfig{Token: "kr-token"}
	s.Require().NoError(SaveConfig(cfg))
}

func (s *ConfigSuite) TestSaveConfig_KeyringFails_FallsBackToFile() {
	s.withKeyring()
	cfg := CLIConfig{Token: "file-token"}
	s.Require().NoError(SaveConfig(cfg))
	s.assertFile(cfg)
}

func (s *ConfigSuite) TestLoadFromKeyring() {
	tests := []struct {
		name    string
		data    string
		getErr  error
		want    CLIConfig
		wantErr bool
	}{
		{
			name: "valid config",
			data: `{"token":"t","refresh_token":"r","timezone":"z","preferred_studio_ids":["a","b"]}`,
			want: CLIConfig{Token: "t", RefreshToken: "r", Timezone: "z", PreferredStudioIDs: []string{"a", "b"}},
		},
		{
			name:    "get error",
			getErr:  errors.New("not found"),
			wantErr: true,
		},
		{
			name:    "invalid json",
			data:    "not json",
			wantErr: true,
		},
		{
			name: "empty config",
			data: `{}`,
			want: CLIConfig{},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			keyringGet = func(_, _ string) (string, error) {
				return tt.data, tt.getErr
			}
			got, err := loadFromKeyring()
			if tt.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(tt.want, got)
			}
		})
	}
}

func (s *ConfigSuite) TestSaveToKeyring() {
	keyringSet = func(_, _, value string) error {
		var got CLIConfig
		s.Require().NoError(json.Unmarshal([]byte(value), &got))
		s.Equal("t", got.Token)
		s.Equal("r", got.RefreshToken)
		return nil
	}
	cfg := CLIConfig{Token: "t", RefreshToken: "r"}
	s.Require().NoError(saveToKeyring(cfg))
}

func (s *ConfigSuite) TestSaveToKeyring_MarshalError() {
	keyringSet = func(_, _, _ string) error { return nil }
	err := saveToKeyring(CLIConfig{})
	s.NoError(err)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
