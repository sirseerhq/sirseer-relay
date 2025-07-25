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

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirseerhq/sirseer-relay/test/testutil"
	"gopkg.in/yaml.v3"
)

// TestConfigFilePrecedence tests configuration loading and precedence rules
func TestConfigFilePrecedence(t *testing.T) {

	// Track what batch size was used in requests
	var lastBatchSize int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request to get batch size
		var req struct {
			Variables struct {
				First int `json:"first"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		lastBatchSize = req.Variables.First

		// Return a simple response
		response := testutil.GeneratePRResponse(1, 5, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tests := []struct {
		name             string
		configFile       map[string]interface{}
		envVars          map[string]string
		cliArgs          []string
		expectedBatch    int
		expectedEndpoint string
	}{
		{
			name: "config file only",
			configFile: map[string]interface{}{
				"defaults": map[string]interface{}{
					"batch_size": 25,
				},
				"github": map[string]interface{}{
					"graphql_endpoint": server.URL + "/graphql",
				},
			},
			expectedBatch: 25,
		},
		{
			name: "env var overrides config file",
			configFile: map[string]interface{}{
				"defaults": map[string]interface{}{
					"batch_size": 25,
				},
			},
			envVars: map[string]string{
				"SIRSEER_BATCH_SIZE": "30",
			},
			expectedBatch: 30,
		},
		{
			name: "CLI flag overrides both config and env",
			configFile: map[string]interface{}{
				"defaults": map[string]interface{}{
					"batch_size": 25,
				},
			},
			envVars: map[string]string{
				"SIRSEER_BATCH_SIZE": "30",
			},
			cliArgs:       []string{"--batch-size", "40"},
			expectedBatch: 40,
		},
		{
			name: "repository-specific config",
			configFile: map[string]interface{}{
				"defaults": map[string]interface{}{
					"batch_size": 50,
				},
				"repositories": map[string]interface{}{
					"test/repo": map[string]interface{}{
						"batch_size": 15,
					},
				},
			},
			expectedBatch: 15,
		},
		{
			name: "CLI flag overrides repository-specific config",
			configFile: map[string]interface{}{
				"repositories": map[string]interface{}{
					"test/repo": map[string]interface{}{
						"batch_size": 15,
					},
				},
			},
			cliArgs:       []string{"--batch-size", "20"},
			expectedBatch: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := testutil.CreateTempDir(t, "config-precedence")
			outputFile := filepath.Join(testDir, "output.ndjson")

			// Create config file if specified
			var configPath string
			if tt.configFile != nil {
				configPath = filepath.Join(testDir, "config.yaml")
				configData, err := yaml.Marshal(tt.configFile)
				if err != nil {
					t.Fatalf("Failed to marshal config: %v", err)
				}

				// Ensure graphql_endpoint is set
				if tt.configFile["github"] == nil {
					tt.configFile["github"] = map[string]interface{}{
						"graphql_endpoint": server.URL + "/graphql",
					}
					configData, _ = yaml.Marshal(tt.configFile)
				}

				if err := os.WriteFile(configPath, configData, 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			}

			// Build command args
			args := []string{"fetch", "test/repo", "--output", outputFile}
			if configPath != "" {
				args = append([]string{"--config", configPath}, args...)
			}
			args = append(args, tt.cliArgs...)

			// Set up environment
			env := map[string]string{
				"GITHUB_TOKEN": "test-token",
			}
			if tt.configFile == nil && tt.envVars["GITHUB_GRAPHQL_ENDPOINT"] == "" {
				env["GITHUB_GRAPHQL_ENDPOINT"] = server.URL + "/graphql"
			}
			for k, v := range tt.envVars {
				env[k] = v
			}

			// Run command
			result := testutil.RunCLI(t, args, env)

			testutil.AssertCLISuccess(t, result)

			// Verify batch size used
			if lastBatchSize != tt.expectedBatch {
				t.Errorf("Expected batch size %d, got %d", tt.expectedBatch, lastBatchSize)
			}
		})
	}
}

// TestTokenPrecedence tests GitHub token configuration precedence
func TestTokenPrecedence(t *testing.T) {

	// Track what token was used
	var lastToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "" {
			lastToken = auth[7:] // Remove "Bearer " prefix
		}

		response := testutil.GeneratePRResponse(1, 1, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tests := []struct {
		name          string
		configFile    map[string]interface{}
		envVars       map[string]string
		cliArgs       []string
		expectedToken string
	}{
		{
			name: "env var token",
			envVars: map[string]string{
				"GITHUB_TOKEN": "env-token",
			},
			expectedToken: "env-token",
		},
		{
			name: "CLI flag overrides env var",
			envVars: map[string]string{
				"GITHUB_TOKEN": "env-token",
			},
			cliArgs:       []string{"--token", "cli-token"},
			expectedToken: "cli-token",
		},
		{
			name: "custom token env var from config",
			configFile: map[string]interface{}{
				"github": map[string]interface{}{
					"token_env": "CUSTOM_GH_TOKEN",
				},
			},
			envVars: map[string]string{
				"GITHUB_TOKEN":    "default-token",
				"CUSTOM_GH_TOKEN": "custom-token",
			},
			expectedToken: "custom-token",
		},
		{
			name: "CLI flag overrides custom env var",
			configFile: map[string]interface{}{
				"github": map[string]interface{}{
					"token_env": "CUSTOM_GH_TOKEN",
				},
			},
			envVars: map[string]string{
				"CUSTOM_GH_TOKEN": "custom-token",
			},
			cliArgs:       []string{"--token", "cli-override"},
			expectedToken: "cli-override",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := testutil.CreateTempDir(t, "token-precedence")
			outputFile := filepath.Join(testDir, "output.ndjson")

			// Create config file if specified
			var configPath string
			if tt.configFile != nil {
				configPath = filepath.Join(testDir, "config.yaml")

				// Ensure graphql_endpoint is set
				if github, ok := tt.configFile["github"].(map[string]interface{}); ok {
					github["graphql_endpoint"] = server.URL + "/graphql"
				} else {
					tt.configFile["github"] = map[string]interface{}{
						"graphql_endpoint": server.URL + "/graphql",
					}
				}

				configData, err := yaml.Marshal(tt.configFile)
				if err != nil {
					t.Fatalf("Failed to marshal config: %v", err)
				}
				if err := os.WriteFile(configPath, configData, 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			}

			// Build command args
			args := []string{"fetch", "test/repo", "--output", outputFile}
			if configPath != "" {
				args = append([]string{"--config", configPath}, args...)
			}
			args = append(args, tt.cliArgs...)

			// Set up environment
			env := map[string]string{}
			if tt.configFile == nil {
				env["GITHUB_GRAPHQL_ENDPOINT"] = server.URL + "/graphql"
			}
			for k, v := range tt.envVars {
				env[k] = v
			}

			// Run command
			result := testutil.RunCLI(t, args, env)

			testutil.AssertCLISuccess(t, result)

			// Verify token used
			if lastToken != tt.expectedToken {
				t.Errorf("Expected token '%s', got '%s'", tt.expectedToken, lastToken)
			}
		})
	}
}

// TestStateDirPrecedence tests state directory configuration precedence
func TestStateDirPrecedence(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := testutil.GeneratePRResponse(1, 5, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tests := []struct {
		name             string
		configFile       map[string]interface{}
		envVars          map[string]string
		expectedStateLoc string
	}{
		{
			name:             "default state dir",
			expectedStateLoc: ".sirseer/state",
		},
		{
			name: "config file state dir",
			configFile: map[string]interface{}{
				"defaults": map[string]interface{}{
					"state_dir": "custom-state",
				},
			},
			expectedStateLoc: "custom-state",
		},
		{
			name: "env var overrides config",
			configFile: map[string]interface{}{
				"defaults": map[string]interface{}{
					"state_dir": "config-state",
				},
			},
			envVars: map[string]string{
				"SIRSEER_STATE_DIR": "env-state",
			},
			expectedStateLoc: "env-state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := testutil.CreateTempDir(t, "state-dir-precedence")
			outputFile := filepath.Join(testDir, "output.ndjson")

			// Change to test directory
			oldDir, _ := os.Getwd()
			os.Chdir(testDir)
			defer os.Chdir(oldDir)

			// Create config file if specified
			var configPath string
			if tt.configFile != nil {
				configPath = filepath.Join(testDir, "config.yaml")

				// Ensure graphql_endpoint is set
				if tt.configFile["github"] == nil {
					tt.configFile["github"] = map[string]interface{}{
						"graphql_endpoint": server.URL + "/graphql",
					}
				}

				configData, err := yaml.Marshal(tt.configFile)
				if err != nil {
					t.Fatalf("Failed to marshal config: %v", err)
				}
				if err := os.WriteFile(configPath, configData, 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			}

			// Build command args
			args := []string{"fetch", "test/repo", "--output", outputFile, "--all"}
			if configPath != "" {
				args = append([]string{"--config", configPath}, args...)
			}

			// Set up environment
			env := map[string]string{
				"GITHUB_TOKEN": "test-token",
			}
			if tt.configFile == nil {
				env["GITHUB_GRAPHQL_ENDPOINT"] = server.URL + "/graphql"
			}
			for k, v := range tt.envVars {
				env[k] = v
			}

			// Run command
			result := testutil.RunCLI(t, args, env)

			testutil.AssertCLISuccess(t, result)

			// Verify state file location
			stateFile := filepath.Join(tt.expectedStateLoc, "test-repo.state")
			if _, err := os.Stat(stateFile); os.IsNotExist(err) {
				t.Errorf("Expected state file at %s, but it doesn't exist", stateFile)
			}
		})
	}
}
