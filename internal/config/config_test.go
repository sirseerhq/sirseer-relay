// Copyright 2025 SirSeer, LLC
//
// Licensed under the Business Source License 1.1 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mariadb.com/bsl11
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test GitHub defaults
	if cfg.GitHub.APIEndpoint != "https://api.github.com" {
		t.Errorf("APIEndpoint = %s, want https://api.github.com", cfg.GitHub.APIEndpoint)
	}
	if cfg.GitHub.GraphQLEndpoint != "https://api.github.com/graphql" {
		t.Errorf("GraphQLEndpoint = %s, want https://api.github.com/graphql", cfg.GitHub.GraphQLEndpoint)
	}
	if cfg.GitHub.TokenEnv != "GITHUB_TOKEN" {
		t.Errorf("TokenEnv = %s, want GITHUB_TOKEN", cfg.GitHub.TokenEnv)
	}

	// Test defaults
	if cfg.Defaults.BatchSize != 50 {
		t.Errorf("BatchSize = %d, want 50", cfg.Defaults.BatchSize)
	}
	if cfg.Defaults.OutputFormat != "ndjson" {
		t.Errorf("OutputFormat = %s, want ndjson", cfg.Defaults.OutputFormat)
	}
	if cfg.Defaults.StateDir != "~/.sirseer/state" {
		t.Errorf("StateDir = %s, want ~/.sirseer/state", cfg.Defaults.StateDir)
	}

	// Test rate limit defaults
	if !cfg.RateLimit.AutoWait {
		t.Error("AutoWait = false, want true")
	}
	if !cfg.RateLimit.ShowProgress {
		t.Error("ShowProgress = false, want true")
	}
}

func TestLoadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write test config
	configContent := `
github:
  api_endpoint: https://github.enterprise.com/api/v3
  graphql_endpoint: https://github.enterprise.com/api/graphql
  token_env: GITHUB_ENTERPRISE_TOKEN

defaults:
  batch_size: 25
  output_format: json
  state_dir: /custom/state

repositories:
  "org/repo":
    batch_size: 10

rate_limit:
  auto_wait: false
  show_progress: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify GitHub settings
	if cfg.GitHub.APIEndpoint != "https://github.enterprise.com/api/v3" {
		t.Errorf("APIEndpoint = %s, want https://github.enterprise.com/api/v3", cfg.GitHub.APIEndpoint)
	}
	if cfg.GitHub.TokenEnv != "GITHUB_ENTERPRISE_TOKEN" {
		t.Errorf("TokenEnv = %s, want GITHUB_ENTERPRISE_TOKEN", cfg.GitHub.TokenEnv)
	}

	// Verify defaults
	if cfg.Defaults.BatchSize != 25 {
		t.Errorf("BatchSize = %d, want 25", cfg.Defaults.BatchSize)
	}
	if cfg.Defaults.OutputFormat != "json" {
		t.Errorf("OutputFormat = %s, want json", cfg.Defaults.OutputFormat)
	}

	// Verify repository overrides
	if repoConfig, ok := cfg.Repositories["org/repo"]; !ok {
		t.Error("Repository org/repo not found")
	} else if repoConfig.BatchSize != 10 {
		t.Errorf("Repository BatchSize = %d, want 10", repoConfig.BatchSize)
	}

	// Verify rate limit settings
	if cfg.RateLimit.AutoWait {
		t.Error("AutoWait = true, want false")
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("GITHUB_API_ENDPOINT", "https://custom.api.com")
	os.Setenv("GITHUB_GRAPHQL_ENDPOINT", "https://custom.graphql.com")
	os.Setenv("SIRSEER_BATCH_SIZE", "75")
	os.Setenv("SIRSEER_STATE_DIR", "/env/state")
	os.Setenv("SIRSEER_RATE_LIMIT_AUTO_WAIT", "false")

	defer func() {
		os.Unsetenv("GITHUB_API_ENDPOINT")
		os.Unsetenv("GITHUB_GRAPHQL_ENDPOINT")
		os.Unsetenv("SIRSEER_BATCH_SIZE")
		os.Unsetenv("SIRSEER_STATE_DIR")
		os.Unsetenv("SIRSEER_RATE_LIMIT_AUTO_WAIT")
	}()

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify environment overrides
	if cfg.GitHub.APIEndpoint != "https://custom.api.com" {
		t.Errorf("APIEndpoint = %s, want https://custom.api.com", cfg.GitHub.APIEndpoint)
	}
	if cfg.GitHub.GraphQLEndpoint != "https://custom.graphql.com" {
		t.Errorf("GraphQLEndpoint = %s, want https://custom.graphql.com", cfg.GitHub.GraphQLEndpoint)
	}
	if cfg.Defaults.BatchSize != 75 {
		t.Errorf("BatchSize = %d, want 75", cfg.Defaults.BatchSize)
	}
	if cfg.Defaults.StateDir != "/env/state" {
		t.Errorf("StateDir = %s, want /env/state", cfg.Defaults.StateDir)
	}
	if cfg.RateLimit.AutoWait {
		t.Error("AutoWait = true, want false")
	}
}

func TestGetBatchSize(t *testing.T) {
	cfg := &Config{
		Defaults: DefaultsConfig{
			BatchSize: 50,
		},
		Repositories: map[string]RepoConfig{
			"org/repo1": {BatchSize: 25},
			"org/repo2": {BatchSize: 0}, // No override
		},
	}

	tests := []struct {
		repo string
		want int
	}{
		{"org/repo1", 25}, // Has override
		{"org/repo2", 50}, // No override (0 means use default)
		{"org/repo3", 50}, // Not in map
	}

	for _, tt := range tests {
		if got := cfg.GetBatchSize(tt.repo); got != tt.want {
			t.Errorf("GetBatchSize(%s) = %d, want %d", tt.repo, got, tt.want)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr string
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: "",
		},
		{
			name: "negative batch size",
			config: &Config{
				Defaults: DefaultsConfig{BatchSize: -1},
				GitHub:   GitHubConfig{APIEndpoint: "http://api", GraphQLEndpoint: "http://graphql"},
			},
			wantErr: "batch size must be positive",
		},
		{
			name: "batch size too large",
			config: &Config{
				Defaults: DefaultsConfig{BatchSize: 150},
				GitHub:   GitHubConfig{APIEndpoint: "http://api", GraphQLEndpoint: "http://graphql"},
			},
			wantErr: "exceeds GitHub API limit of 100",
		},
		{
			name: "empty API endpoint",
			config: &Config{
				Defaults: DefaultsConfig{BatchSize: 50},
				GitHub:   GitHubConfig{APIEndpoint: "", GraphQLEndpoint: "http://graphql"},
			},
			wantErr: "GitHub API endpoint cannot be empty",
		},
		{
			name: "empty GraphQL endpoint",
			config: &Config{
				Defaults: DefaultsConfig{BatchSize: 50},
				GitHub:   GitHubConfig{APIEndpoint: "http://api", GraphQLEndpoint: ""},
			},
			wantErr: "GitHub GraphQL endpoint cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() error = nil, want %s", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %v, want containing %s", err, tt.wantErr)
				}
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/test", filepath.Join(home, "test")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		if got := expandPath(tt.input); got != tt.want {
			t.Errorf("expandPath(%s) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
		{"1", true},
		{"on", true},
		{"ON", true},
		{"false", false},
		{"FALSE", false},
		{"no", false},
		{"0", false},
		{"off", false},
		{"", false},
		{"random", false},
	}

	for _, tt := range tests {
		if got := parseBool(tt.input); got != tt.want {
			t.Errorf("parseBool(%s) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"50", 50, false},
		{"1", 1, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		got, err := parsePositiveInt(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePositiveInt(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parsePositiveInt(%s) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
