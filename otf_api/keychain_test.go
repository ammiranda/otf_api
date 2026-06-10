//go:build darwin

package otf_api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeychainIntegration_StoreAndRetrieve(t *testing.T) {
	const testKey = "_test_otf_api_token"
	want := "test-secret-value"

	err := keychainSet(testKey, want)
	require.NoError(t, err)
	t.Cleanup(func() { cleanupKeychain(t, testKey) })

	got, err := keychainGet(testKey)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestKeychainIntegration_Overwrite(t *testing.T) {
	const testKey = "_test_otf_api_overwrite"

	err := keychainSet(testKey, "first")
	require.NoError(t, err)
	t.Cleanup(func() { cleanupKeychain(t, testKey) })

	err = keychainSet(testKey, "second")
	require.NoError(t, err)

	got, err := keychainGet(testKey)
	require.NoError(t, err)
	assert.Equal(t, "second", got)
}

func TestKeychainIntegration_MultipleKeys(t *testing.T) {
	const (
		tokenKey = "_test_otf_api_token"
		refKey   = "_test_otf_api_refresh"
	)

	err := keychainSet(tokenKey, "token-val")
	require.NoError(t, err)
	t.Cleanup(func() { cleanupKeychain(t, tokenKey) })

	err = keychainSet(refKey, "refresh-val")
	require.NoError(t, err)
	t.Cleanup(func() { cleanupKeychain(t, refKey) })

	token, err := keychainGet(tokenKey)
	require.NoError(t, err)
	assert.Equal(t, "token-val", token)

	refresh, err := keychainGet(refKey)
	require.NoError(t, err)
	assert.Equal(t, "refresh-val", refresh)
}

func TestKeychainIntegration_MissingKey(t *testing.T) {
	_, err := keychainGet("_test_otf_api_nonexistent")
	require.Error(t, err)
}

func cleanupKeychain(t testing.TB, key string) {
	t.Helper()
	if err := keychainDel(key); err != nil {
		t.Logf("cleanup keychain %q: %v", key, err)
	}
}
