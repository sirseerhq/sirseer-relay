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

// Package config provides configuration management for sirseer-relay with
// support for multiple configuration sources and a well-defined precedence
// order. It enables enterprise deployments to customize behavior through
// configuration files while maintaining flexibility with environment variables
// and command-line overrides.
//
// Configuration sources (in precedence order, highest to lowest):
//  1. Command-line flags
//  2. Environment variables
//  3. Repository-specific configuration
//  4. Global configuration file
//  5. Built-in defaults
//
// The package supports YAML configuration files and provides automatic
// discovery of configuration in standard locations. It's designed to work
// seamlessly with GitHub Enterprise deployments and supports repository-specific
// overrides for fine-grained control.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from multiple sources and applies them in
// the correct precedence order. If configPath is provided, it loads from
// that specific file. Otherwise, it searches standard locations:
//   - .sirseer-relay.yaml (current directory)
//   - .sirseer-relay.yml (current directory)
//   - ~/.sirseer/config.yaml
//   - ~/.sirseer/config.yml
//
// Environment variables are applied after loading the config file, allowing
// runtime overrides. Path expansion (~ and environment variables) is performed
// on directory paths.
//
// Returns an error if the specified config file cannot be loaded, but will
// succeed with defaults if no config file is found in standard locations.
func LoadConfig(configPath string) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Try to load config file if path is provided
	if configPath != "" {
		if err := loadConfigFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	} else {
		// Try default locations
		defaultPaths := []string{
			".sirseer-relay.yaml",
			".sirseer-relay.yml",
			filepath.Join(os.Getenv("HOME"), ".sirseer", "config.yaml"),
			filepath.Join(os.Getenv("HOME"), ".sirseer", "config.yml"),
		}

		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				if err := loadConfigFile(path, cfg); err != nil {
					return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
				}
				break
			}
		}
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	// Expand paths
	cfg.Defaults.StateDir = expandPath(cfg.Defaults.StateDir)

	return cfg, nil
}

// LoadConfigForRepo loads configuration and applies repository-specific
// overrides. This allows different settings for different repositories,
// useful when some repositories require special handling (e.g., lower
// batch sizes for repositories with large PRs).
//
// The repo parameter should be in "owner/repo" format.
func LoadConfigForRepo(configPath, repo string) (*Config, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Apply repository-specific overrides if they exist
	if repoConfig, ok := cfg.Repositories[repo]; ok {
		if repoConfig.BatchSize > 0 {
			cfg.Defaults.BatchSize = repoConfig.BatchSize
		}
	}

	return cfg, nil
}

// loadConfigFile reads and parses a YAML config file
func loadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(cfg *Config) {
	// GitHub endpoints
	if endpoint := os.Getenv("GITHUB_API_ENDPOINT"); endpoint != "" {
		cfg.GitHub.APIEndpoint = endpoint
	}
	if endpoint := os.Getenv("GITHUB_GRAPHQL_ENDPOINT"); endpoint != "" {
		cfg.GitHub.GraphQLEndpoint = endpoint
	}

	// Defaults
	if batchSize := os.Getenv("SIRSEER_BATCH_SIZE"); batchSize != "" {
		if size, err := parsePositiveInt(batchSize); err == nil {
			cfg.Defaults.BatchSize = size
		}
	}
	if stateDir := os.Getenv("SIRSEER_STATE_DIR"); stateDir != "" {
		cfg.Defaults.StateDir = stateDir
	}

	// Rate limit settings
	if autoWait := os.Getenv("SIRSEER_RATE_LIMIT_AUTO_WAIT"); autoWait != "" {
		cfg.RateLimit.AutoWait = parseBool(autoWait)
	}
}

// expandPath expands ~ and environment variables in paths
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("USERPROFILE") // Windows
		}
		path = filepath.Join(home, path[2:])
	}
	return os.ExpandEnv(path)
}

// parsePositiveInt parses a string to a positive integer
func parsePositiveInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil {
		return 0, fmt.Errorf("failed to parse integer from '%s': %w", s, err)
	}
	if i <= 0 {
		return 0, fmt.Errorf("value must be positive, got: %d", i)
	}
	return i, nil
}

// parseBool parses various boolean representations
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "1" || s == "on"
}

// GetBatchSize returns the effective batch size for a repository, taking
// into account repository-specific overrides. If the repository has a
// specific batch size configured, it returns that value. Otherwise, it
// returns the default batch size.
func (c *Config) GetBatchSize(repo string) int {
	if repoConfig, ok := c.Repositories[repo]; ok && repoConfig.BatchSize > 0 {
		return repoConfig.BatchSize
	}
	return c.Defaults.BatchSize
}

// Validate checks if the configuration contains valid values. It ensures
// batch sizes are within GitHub's limits, endpoints are not empty, and
// other constraints are met. This should be called after loading configuration
// to catch invalid settings early.
func (c *Config) Validate() error {
	if c.Defaults.BatchSize <= 0 {
		return fmt.Errorf("default batch size must be positive, got: %d", c.Defaults.BatchSize)
	}
	if c.Defaults.BatchSize > 100 {
		return fmt.Errorf("default batch size %d exceeds GitHub API limit of 100", c.Defaults.BatchSize)
	}
	if c.GitHub.APIEndpoint == "" {
		return fmt.Errorf("GitHub API endpoint cannot be empty")
	}
	if c.GitHub.GraphQLEndpoint == "" {
		return fmt.Errorf("GitHub GraphQL endpoint cannot be empty")
	}
	return nil
}
