package otf_api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	PreferredStudioIDs    []string `json:"preferred_studio_ids,omitempty"`
	Timezone              string   `json:"timezone,omitempty"`
	Token                 string   `json:"token,omitempty"`
	RefreshToken          string   `json:"refresh_token,omitempty"`
	Username              string   `json:"username,omitempty"`
	Password              string   `json:"password,omitempty"`
	EncryptedToken        string   `json:"encrypted_token,omitempty"`
	EncryptedRefreshToken string   `json:"encrypted_refresh_token,omitempty"`
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

	key, keyErr := deriveKey()
	if keyErr == nil {
		if config.EncryptedToken != "" {
			if dec, err := decrypt(config.EncryptedToken, key); err == nil {
				config.Token = dec
			}
		}
		if config.EncryptedRefreshToken != "" {
			if dec, err := decrypt(config.EncryptedRefreshToken, key); err == nil {
				config.RefreshToken = dec
			}
		}
	}

	return config, nil
}

func saveToFile(config CLIConfig) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	config.Username = ""
	config.Password = ""

	key, keyErr := deriveKey()
	if keyErr == nil {
		if config.Token != "" {
			if enc, err := encrypt(config.Token, key); err == nil {
				config.EncryptedToken = enc
				config.Token = ""
			}
		}
		if config.RefreshToken != "" {
			if enc, err := encrypt(config.RefreshToken, key); err == nil {
				config.EncryptedRefreshToken = enc
				config.RefreshToken = ""
			}
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

var deriveKey = func() ([]byte, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	h := sha256.Sum256([]byte(dir + ":otf-file-key-v1"))
	return h[:], nil
}

func encrypt(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(encoded string, key []byte) (string, error) {
	if encoded == "" {
		return "", nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
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
