//go:build darwin

package otf_api

import (
	"fmt"
	"os/exec"
)

var keychainDel = func(key string) error {
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
