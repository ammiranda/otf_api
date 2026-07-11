package otf_api

import (
	"crypto/sha256"
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
	origKey    func() ([]byte, error)
}

func (s *ConfigSuite) SetupTest() {
	s.configPath = filepath.Join(s.T().TempDir(), "config.json")
	s.origGet = KeyringGet
	s.origSet = KeyringSet
	s.origPath = GetConfigPath
	s.origKey = deriveKey
	GetConfigPath = func() (string, error) { return s.configPath, nil }
	KeyringGet = func(_, _ string) (string, error) { return "", errors.New("keyring unavailable") }
	KeyringSet = func(_, _, _ string) error { return errors.New("keyring unavailable") }
	deriveKey = func() ([]byte, error) { return nil, errors.New("encryption disabled") }
}

func (s *ConfigSuite) TearDownTest() {
	KeyringGet = s.origGet
	KeyringSet = s.origSet
	GetConfigPath = s.origPath
	deriveKey = s.origKey
}

func (s *ConfigSuite) withKeyring() {
	KeyringGet = func(_, _ string) (string, error) {
		return "", errors.New("keyring unavailable")
	}
	KeyringSet = func(_, _, _ string) error {
		return errors.New("keyring unavailable")
	}
}

func (s *ConfigSuite) withKeyringData(cfg CLIConfig) {
	data, _ := json.Marshal(cfg)
	KeyringGet = func(_, _ string) (string, error) {
		return string(data), nil
	}
	KeyringSet = func(_, _, _ string) error {
		return nil
	}
}

func (s *ConfigSuite) withEncryption() {
	deriveKey = func() ([]byte, error) {
		return []byte("0123456789abcdef0123456789abcdef"), nil
	}
}

func (s *ConfigSuite) withKeyringSet() {
	KeyringSet = func(_, _, _ string) error { return nil }
}

func (s *ConfigSuite) writeFile(cfg CLIConfig) {
	data, err := json.Marshal(cfg)
	s.Require().NoError(err)
	s.Require().NoError(os.WriteFile(s.configPath, data, 0600))
}

func (s *ConfigSuite) TestGetConfigPath() {
	GetConfigPath = s.origPath
	path, err := GetConfigPath()
	s.Require().NoError(err)
	s.Contains(path, "otf-cli")
	s.Contains(path, "config.json")
}

func (s *ConfigSuite) TestSaveAndLoadFromFile() {
	s.withEncryption()
	saved := CLIConfig{
		Token:              "tok",
		RefreshToken:       "ref",
		Timezone:           "America/Chicago",
		PreferredStudioIDs: []string{"studio-1", "studio-2"},
	}
	s.Require().NoError(saveToFile(saved))

	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Equal("tok", loaded.Token)
	s.Equal("ref", loaded.RefreshToken)
	s.Equal("America/Chicago", loaded.Timezone)
	s.Equal([]string{"studio-1", "studio-2"}, loaded.PreferredStudioIDs)
	s.NotEmpty(loaded.EncryptedToken)
	s.NotEmpty(loaded.EncryptedRefreshToken)
}

func (s *ConfigSuite) TestSaveToFile_StripsCredentials() {
	s.withEncryption()
	saved := CLIConfig{
		Token:    "tok",
		Username: "user@example.com",
		Password: "secret",
	}
	s.Require().NoError(saveToFile(saved))

	data, err := os.ReadFile(s.configPath)
	s.Require().NoError(err)

	var raw map[string]any
	s.Require().NoError(json.Unmarshal(data, &raw))
	s.Empty(raw["token"])
	s.Empty(raw["username"])
	s.Empty(raw["password"])
	s.NotEmpty(raw["encrypted_username"])
	s.NotEmpty(raw["encrypted_password"])

	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Equal("tok", loaded.Token)
	s.Equal("user@example.com", loaded.Username)
	s.Equal("secret", loaded.Password)
}

func (s *ConfigSuite) TestSaveToFile_EncryptsTokens() {
	s.withEncryption()
	saved := CLIConfig{Token: "my-token", RefreshToken: "my-refresh"}
	s.Require().NoError(saveToFile(saved))

	data, err := os.ReadFile(s.configPath)
	s.Require().NoError(err)

	var raw map[string]any
	s.Require().NoError(json.Unmarshal(data, &raw))
	s.Empty(raw["token"])
	s.Empty(raw["refresh_token"])
	s.NotEmpty(raw["encrypted_token"])
	s.NotEmpty(raw["encrypted_refresh_token"])
}

func (s *ConfigSuite) TestSaveAndLoadFromFile_WithEncryption() {
	s.withEncryption()
	saved := CLIConfig{Token: "my-token", RefreshToken: "my-refresh", Timezone: "UTC"}
	s.Require().NoError(saveToFile(saved))

	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Equal("my-token", loaded.Token)
	s.Equal("my-refresh", loaded.RefreshToken)
	s.Equal("UTC", loaded.Timezone)
	s.NotEmpty(loaded.EncryptedToken)
	s.NotEmpty(loaded.EncryptedRefreshToken)
}

func (s *ConfigSuite) TestLoadFromFile_BackwardCompatPlaintextTokens() {
	s.writeFile(CLIConfig{Token: "plain-token", RefreshToken: "plain-refresh", Timezone: "EST"})

	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Equal("plain-token", loaded.Token)
	s.Equal("plain-refresh", loaded.RefreshToken)
	s.Equal("EST", loaded.Timezone)
}

func (s *ConfigSuite) TestLoadFromFile_EncryptionFailureFallsBackToEmpty() {
	s.withEncryption()

	saved := CLIConfig{Token: "good-token"}
	s.Require().NoError(saveToFile(saved))

	deriveKey = func() ([]byte, error) { return []byte("different-key-wont-match!!!!"), nil }
	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Empty(loaded.Token)
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

func (s *ConfigSuite) TestSaveAndLoad_AllowIPLocation_True() {
	allow := true
	saved := CLIConfig{AllowIPLocation: &allow}
	s.Require().NoError(saveToFile(saved))

	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Require().NotNil(loaded.AllowIPLocation)
	s.True(*loaded.AllowIPLocation)
}

func (s *ConfigSuite) TestSaveAndLoad_AllowIPLocation_False() {
	allow := false
	saved := CLIConfig{AllowIPLocation: &allow}
	s.Require().NoError(saveToFile(saved))

	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Require().NotNil(loaded.AllowIPLocation)
	s.False(*loaded.AllowIPLocation)
}

func (s *ConfigSuite) TestLoad_AllowIPLocation_Unset() {
	s.writeFile(CLIConfig{Timezone: "UTC"})

	loaded, err := loadFromFile()
	s.Require().NoError(err)
	s.Nil(loaded.AllowIPLocation)
}

func (s *ConfigSuite) TestSaveConfig_KeyringSuccess() {
	s.withKeyringSet()
	cfg := CLIConfig{Token: "kr-token"}
	s.Require().NoError(SaveConfig(cfg))
}

func (s *ConfigSuite) TestSaveConfig_KeyringFails_FallsBackToFile() {
	s.withKeyring()
	s.withEncryption()
	cfg := CLIConfig{Token: "file-token"}
	s.Require().NoError(SaveConfig(cfg))

	loaded, err := LoadConfig()
	s.Require().NoError(err)
	s.Equal("file-token", loaded.Token)
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
			KeyringGet = func(_, _ string) (string, error) {
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
	KeyringSet = func(_, _, value string) error {
		var got CLIConfig
		s.Require().NoError(json.Unmarshal([]byte(value), &got))
		s.Equal("t", got.Token)
		s.Equal("r", got.RefreshToken)
		s.Equal("u", got.Username)
		s.Equal("p", got.Password)
		return nil
	}
	cfg := CLIConfig{Token: "t", RefreshToken: "r", Username: "u", Password: "p"}
	s.Require().NoError(saveToKeyring(cfg))
}

func (s *ConfigSuite) TestSaveToKeyring_MarshalError() {
	KeyringSet = func(_, _, _ string) error { return nil }
	err := saveToKeyring(CLIConfig{})
	s.NoError(err)
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	origKey := deriveKey
	defer func() { deriveKey = origKey }()

	key := []byte("0123456789abcdef0123456789abcdef")
	original := "my-sensitive-token"

	enc, err := encrypt(original, key)
	if err != nil {
		t.Fatal(err)
	}
	if enc == "" || enc == original {
		t.Fatalf("encrypted output should not be empty or equal to input")
	}

	dec, err := decrypt(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if dec != original {
		t.Fatalf("round-trip failed: got %q, want %q", dec, original)
	}
}

func TestEncryptDecrypt_Empty(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")

	enc, err := encrypt("", key)
	if err != nil {
		t.Fatal(err)
	}
	if enc != "" {
		t.Fatalf("encrypt of empty should return empty, got %q", enc)
	}

	dec, err := decrypt("", key)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "" {
		t.Fatalf("decrypt of empty should return empty, got %q", dec)
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	_, err := decrypt("not-valid-base64!!!", key)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := []byte("0123456789abcdef0123456789abcdef")
	key2 := []byte("ffffffffffffffffffffffffffffffff")
	original := "hello-world"

	enc, err := encrypt(original, key1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = decrypt(enc, key2)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")

	enc, err := encrypt("my-token", key)
	if err != nil {
		t.Fatal(err)
	}

	tampered := enc[:len(enc)-1] + "X"
	_, err = decrypt(tampered, key)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestDeriveKey_PersistsAcrossCalls(t *testing.T) {
	origPath := GetConfigPath
	defer func() { GetConfigPath = origPath }()

	path := filepath.Join(t.TempDir(), "config.json")
	GetConfigPath = func() (string, error) { return path, nil }

	k1, err := deriveKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(k1))
	}

	k2, err := deriveKey()
	if err != nil {
		t.Fatal(err)
	}
	if string(k1) != string(k2) {
		t.Fatal("deriveKey should return the same persisted key across calls")
	}
}

func TestDeriveKey_RandomPerInstall(t *testing.T) {
	origPath := GetConfigPath
	defer func() { GetConfigPath = origPath }()

	path1 := filepath.Join(t.TempDir(), "config.json")
	path2 := filepath.Join(t.TempDir(), "config.json")

	GetConfigPath = func() (string, error) { return path1, nil }
	k1, err := deriveKey()
	if err != nil {
		t.Fatal(err)
	}

	GetConfigPath = func() (string, error) { return path2, nil }
	k2, err := deriveKey()
	if err != nil {
		t.Fatal(err)
	}

	if string(k1) == string(k2) {
		t.Fatal("deriveKey should generate an independent random key for each install")
	}
}

func TestDeriveKey_KeyFileNotDerivableFromPathAlone(t *testing.T) {
	origPath := GetConfigPath
	defer func() { GetConfigPath = origPath }()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	GetConfigPath = func() (string, error) { return path, nil }

	key, err := deriveKey()
	if err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256([]byte(dir + ":otf-file-key-v1"))
	if string(key) == string(h[:]) {
		t.Fatal("derived key must not match the legacy static-salt hash of the config path")
	}

	keyPath := filepath.Join(dir, keyFileName)
	if _, statErr := os.Stat(keyPath); statErr != nil {
		t.Fatalf("expected key file to be persisted at %s: %v", keyPath, statErr)
	}
}

func TestDeriveKey_Error(t *testing.T) {
	origPath := GetConfigPath
	defer func() { GetConfigPath = origPath }()

	GetConfigPath = func() (string, error) { return "", errors.New("no config dir") }
	_, err := deriveKey()
	if err == nil {
		t.Fatal("expected error when GetConfigPath fails")
	}
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
