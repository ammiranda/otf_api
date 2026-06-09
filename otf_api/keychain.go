package otf_api

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const keychainService = "otf-cli"

func keychainAvailable() bool {
	return runtime.GOOS == "darwin"
}

func keychainSet(key, value string) error {
	if !keychainAvailable() {
		return fmt.Errorf("keychain not available on %s", runtime.GOOS)
	}
	cmd := exec.Command("security", "add-generic-password",
		"-a", keychainService,
		"-s", key,
		"-w", value,
		"-U",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("security add-generic-password failed: %w\n%s", err, out)
	}
	return nil
}

func keychainGet(key string) (string, error) {
	if !keychainAvailable() {
		return "", fmt.Errorf("keychain not available on %s", runtime.GOOS)
	}
	cmd := exec.Command("security", "find-generic-password",
		"-a", keychainService,
		"-s", key,
		"-w",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("security find-generic-password failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}


