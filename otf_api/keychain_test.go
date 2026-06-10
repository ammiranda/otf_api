//go:build darwin

package otf_api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type KeychainSuite struct {
	suite.Suite
	createdKeys []string
}

func (s *KeychainSuite) TearDownTest() {
	for _, k := range s.createdKeys {
		if err := keychainDel(k); err != nil {
			s.T().Logf("cleanup keychain %q: %v", k, err)
		}
	}
	s.createdKeys = nil
}

func (s *KeychainSuite) key(key string) string {
	return fmt.Sprintf("_test_otf_api_%s", key)
}

func (s *KeychainSuite) track(key string) {
	s.createdKeys = append(s.createdKeys, s.key(key))
}

func (s *KeychainSuite) set(key, value string) {
	s.Require().NoError(keychainSet(s.key(key), value))
}

func (s *KeychainSuite) get(key string) string {
	val, err := keychainGet(s.key(key))
	s.Require().NoError(err)
	return val
}

func (s *KeychainSuite) TestStoreAndRetrieve() {
	tests := []struct {
		key   string
		value string
	}{
		{"token", "test-secret-value"},
		{"refresh", "another-secret"},
		{"timezone", "America/Chicago"},
		{"studio_ids", `["a","b"]`},
	}

	for _, tt := range tests {
		s.Run(tt.key, func() {
			s.track(tt.key)
			s.set(tt.key, tt.value)
			got := s.get(tt.key)
			s.Assert().Equal(tt.value, got)
		})
	}
}

func (s *KeychainSuite) TestOverwrite() {
	s.track("overwrite")
	s.set("overwrite", "first")
	s.set("overwrite", "second")
	got := s.get("overwrite")
	s.Assert().Equal("second", got)
}

func (s *KeychainSuite) TestMultipleKeys() {
	s.track("token")
	s.track("refresh")

	s.set("token", "token-val")
	s.set("refresh", "refresh-val")

	s.Assert().Equal("token-val", s.get("token"))
	s.Assert().Equal("refresh-val", s.get("refresh"))
}

func (s *KeychainSuite) TestMissingKey() {
	_, err := keychainGet(s.key("nonexistent"))
	s.Require().Error(err)
}

func TestKeychainIntegration(t *testing.T) {
	suite.Run(t, new(KeychainSuite))
}
