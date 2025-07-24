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

// Package config types define the configuration structures used throughout
// sirseer-relay. These types represent settings that can be loaded from
// YAML configuration files, environment variables, or command-line flags.
package config

// Config represents the complete configuration for sirseer-relay.
// It consolidates settings from various sources and provides a unified
// interface for accessing configuration values throughout the application.
type Config struct {
	GitHub       GitHubConfig          `yaml:"github"`
	Defaults     DefaultsConfig        `yaml:"defaults"`
	Repositories map[string]RepoConfig `yaml:"repositories"`
	RateLimit    RateLimitConfig       `yaml:"rate_limit"`
}

// GitHubConfig contains GitHub-specific settings including API endpoints
// and authentication configuration. This allows easy configuration for
// GitHub Enterprise deployments by specifying custom endpoints.
type GitHubConfig struct {
	APIEndpoint     string `yaml:"api_endpoint"`
	GraphQLEndpoint string `yaml:"graphql_endpoint"`
	TokenEnv        string `yaml:"token_env"`
}

// DefaultsConfig contains default settings that apply to all fetch operations
// unless overridden by repository-specific settings or command-line flags.
// These settings control the core behavior of the fetch process.
type DefaultsConfig struct {
	BatchSize    int    `yaml:"batch_size"`
	OutputFormat string `yaml:"output_format"`
	StateDir     string `yaml:"state_dir"`
}

// RepoConfig contains repository-specific overrides that allow fine-tuning
// fetch behavior for individual repositories. This is useful when certain
// repositories have special requirements, such as lower batch sizes for
// repositories with very large pull requests.
type RepoConfig struct {
	BatchSize int `yaml:"batch_size"`
}

// RateLimitConfig controls rate limit handling behavior when interacting
// with the GitHub API. It determines whether the tool should automatically
// wait when rate limited or exit with an error, and whether to show
// progress during waits.
type RateLimitConfig struct {
	AutoWait     bool `yaml:"auto_wait"`
	ShowProgress bool `yaml:"show_progress"`
}

// DefaultConfig returns a Config with sensible defaults suitable for most
// use cases. These defaults are optimized for public GitHub.com usage but
// can be overridden for GitHub Enterprise or special requirements.
func DefaultConfig() *Config {
	return &Config{
		GitHub: GitHubConfig{
			APIEndpoint:     "https://api.github.com",
			GraphQLEndpoint: "https://api.github.com/graphql",
			TokenEnv:        "GITHUB_TOKEN",
		},
		Defaults: DefaultsConfig{
			BatchSize:    50,
			OutputFormat: "ndjson",
			StateDir:     "~/.sirseer/state",
		},
		Repositories: make(map[string]RepoConfig),
		RateLimit: RateLimitConfig{
			AutoWait:     true,
			ShowProgress: true,
		},
	}
}
