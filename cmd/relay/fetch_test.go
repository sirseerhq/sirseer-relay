package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirseerhq/sirseer-relay/internal/config"
	"github.com/sirseerhq/sirseer-relay/internal/metadata"
)

func TestParseRepository(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			input:     "golang/go",
			wantOwner: "golang",
			wantRepo:  "go",
			wantErr:   false,
		},
		{
			input:     "kubernetes/kubernetes",
			wantOwner: "kubernetes",
			wantRepo:  "kubernetes",
			wantErr:   false,
		},
		{
			input:   "invalid",
			wantErr: true,
		},
		{
			input:   "too/many/slashes",
			wantErr: true,
		},
		{
			input:   "/repo",
			wantErr: true,
		},
		{
			input:   "owner/",
			wantErr: true,
		},
		{
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		owner, repo, err := parseRepository(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseRepository(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if owner != tt.wantOwner {
				t.Errorf("parseRepository(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseRepository(%q) repo = %q, want %q", tt.input, repo, tt.wantRepo)
			}
		}
	}
}

func TestParseDate(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		input   string
		wantErr bool
		check   func(time.Time) bool
	}{
		{
			input:   "2024-01-15",
			wantErr: false,
			check: func(t time.Time) bool {
				return t.Year() == 2024 && t.Month() == 1 && t.Day() == 15
			},
		},
		{
			input:   "2024-01-15T10:30:00Z",
			wantErr: false,
			check: func(t time.Time) bool {
				return t.Year() == 2024 && t.Month() == 1 && t.Day() == 15 &&
					t.Hour() == 10 && t.Minute() == 30
			},
		},
		{
			input:   "1d",
			wantErr: false,
			check: func(t time.Time) bool {
				// Should be approximately 24 hours ago
				diff := now.Sub(t)
				return diff >= 23*time.Hour && diff <= 25*time.Hour
			},
		},
		{
			input:   "invalid",
			wantErr: true,
		},
		{
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		got, err := parseDate(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && tt.check != nil {
			if !tt.check(got) {
				t.Errorf("parseDate(%q) = %v, failed check", tt.input, got)
			}
		}
	}
}

func TestGetToken(t *testing.T) {
	// Save current env
	oldToken := os.Getenv("GITHUB_TOKEN")
	oldCustom := os.Getenv("CUSTOM_TOKEN")
	defer func() {
		os.Setenv("GITHUB_TOKEN", oldToken)
		os.Setenv("CUSTOM_TOKEN", oldCustom)
	}()

	tests := []struct {
		name      string
		flagToken string
		envVar    string
		envValue  string
		want      string
	}{
		{
			name:      "flag takes precedence",
			flagToken: "flag-token",
			envVar:    "GITHUB_TOKEN",
			envValue:  "env-token",
			want:      "flag-token",
		},
		{
			name:      "env var fallback",
			flagToken: "",
			envVar:    "GITHUB_TOKEN",
			envValue:  "env-token",
			want:      "env-token",
		},
		{
			name:      "custom env var",
			flagToken: "",
			envVar:    "CUSTOM_TOKEN",
			envValue:  "custom-token",
			want:      "custom-token",
		},
		{
			name:      "no token",
			flagToken: "",
			envVar:    "NONEXISTENT",
			envValue:  "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.envVar, tt.envValue)
			got := getToken(tt.flagToken, tt.envVar)
			if got != tt.want {
				t.Errorf("getToken(%q, %q) = %q, want %q", tt.flagToken, tt.envVar, got, tt.want)
			}
		})
	}
}

func TestMapErrorToExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil error",
			err:      nil,
			wantCode: 0,
		},
		{
			name:     "general error",
			err:      os.ErrClosed,
			wantCode: 1,
		},
		// Add more test cases for specific error types when available
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapErrorToExitCode(tt.err)
			if got != tt.wantCode {
				t.Errorf("mapErrorToExitCode(%v) = %d, want %d", tt.err, got, tt.wantCode)
			}
		})
	}
}

func TestConfigIntegration(t *testing.T) {
	// Test that config loading works with fetch command
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
github:
  token_env: TEST_GITHUB_TOKEN
defaults:
  batch_size: 25
  state_dir: %s
`
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(strings.ReplaceAll(configContent, "%s", tmpDir))), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config for a test repo
	cfg, err := config.LoadConfigForRepo(configPath, "test/repo")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config was loaded
	if cfg.GitHub.TokenEnv != "TEST_GITHUB_TOKEN" {
		t.Errorf("TokenEnv = %s, want TEST_GITHUB_TOKEN", cfg.GitHub.TokenEnv)
	}
	if cfg.GetBatchSize("test/repo") != 25 {
		t.Errorf("BatchSize = %d, want 25", cfg.GetBatchSize("test/repo"))
	}
}

func TestMetadataIntegration(t *testing.T) {
	// Test metadata generation and saving
	tmpDir := t.TempDir()

	// Create a tracker and simulate some activity
	tracker := metadata.New()
	tracker.IncrementAPICall()
	tracker.IncrementAPICall()
	tracker.UpdatePRStats(100, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	tracker.UpdatePRStats(101, time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC))

	// Generate metadata
	params := metadata.FetchParams{
		Organization: "test",
		Repository:   "repo",
		FetchAll:     true,
		BatchSize:    50,
	}

	meta := tracker.GenerateMetadata("v1.0.0", params, false, nil)

	// Verify metadata
	if meta.RelayVersion != "v1.0.0" {
		t.Errorf("RelayVersion = %s, want v1.0.0", meta.RelayVersion)
	}
	if meta.Results.TotalPRs != 2 {
		t.Errorf("TotalPRs = %d, want 2", meta.Results.TotalPRs)
	}
	if meta.Results.APICallCount != 2 {
		t.Errorf("APICallCount = %d, want 2", meta.Results.APICallCount)
	}

	// Save metadata
	if err := metadata.SaveMetadata(meta, tmpDir); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Load it back
	loaded, err := metadata.LoadLatestMetadata(tmpDir, "test/repo")
	if err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	if loaded == nil {
		t.Fatal("Expected to load metadata, got nil")
	}

	if loaded.FetchID != meta.FetchID {
		t.Errorf("Loaded FetchID = %s, want %s", loaded.FetchID, meta.FetchID)
	}
}
