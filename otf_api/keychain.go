package otf_api

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const keychainService = "otf-cli"

var keychainAvailable = func() bool {
	return runtime.GOOS == "darwin"
}

var keychainSet = func(key, value string) error {
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

var keychainGet = func(key string) (string, error) {
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

var keychainDel = func(key string) error {
	if !keychainAvailable() {
		return fmt.Errorf("keychain not available on %s", runtime.GOOS)
	}
	cmd := exec.Command("security", "delete-generic-password",
		"-a", keychainService,
		"-s", key,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("security delete-generic-password failed: %w\n%s", err, out)
	}
	return nil
}


