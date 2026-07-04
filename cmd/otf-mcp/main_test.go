package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/ammiranda/otf_api/otf_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureClient_ReturnsCachedClient(t *testing.T) {
	s := &MCPServer{
		client: otf_api.NewClient(),
	}
	c, err := s.ensureClient()
	require.NoError(t, err)
	assert.Same(t, s.client, c)
}

func TestEnsureClient_ReturnsErrorWhenNoCredentials(t *testing.T) {
	s := &MCPServer{}

	savedPrompt := promptCredentials
	promptCredentials = func() (string, string, error) {
		return "", "", errors.New("no terminal available")
	}
	defer func() { promptCredentials = savedPrompt }()

	savedLoad := loadConfig
	loadConfig = func() (otf_api.CLIConfig, error) {
		return otf_api.CLIConfig{}, nil
	}
	defer func() { loadConfig = savedLoad }()

	_, err := s.ensureClient()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials available")
}

func TestHandleToolCall_ReturnsErrorWhenNoAuth(t *testing.T) {
	savedPrompt := promptCredentials
	promptCredentials = func() (string, string, error) {
		return "", "", errors.New("no terminal available")
	}
	defer func() { promptCredentials = savedPrompt }()

	savedLoad := loadConfig
	loadConfig = func() (otf_api.CLIConfig, error) {
		return otf_api.CLIConfig{}, nil
	}
	defer func() { loadConfig = savedLoad }()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	old := os.Stdout
	os.Stdout = w

	s := &MCPServer{}
	params := json.RawMessage(`{"name":"get_schedules","arguments":{}}`)
	s.handleToolCall(json.RawMessage(`1`), params)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	var resp JSONRPCResponse
	err = json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, err)

	require.NotNil(t, resp.Error)
	assert.Equal(t, otf_api.ErrCodeAuthRequired, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Authentication required")
}
