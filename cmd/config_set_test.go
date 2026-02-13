package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hpkotak/shellbud/internal/config"
)

func TestRunConfigSet(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{"set valid provider", "provider", "ollama", ""},
		{"set invalid provider", "provider", "openai", "invalid provider"},
		{"set valid model", "model", "codellama:7b", ""},
		{"set valid host", "ollama.host", "http://192.168.1.100:11434", ""},
		{"set invalid host", "ollama.host", "://broken", "invalid URL"},
		{"unknown key", "unknown.key", "value", "unknown config key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Isolate config directory
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)

			// Create initial config so Load() succeeds
			cfgDir := filepath.Join(tmpDir, ".shellbud")
			if err := os.MkdirAll(cfgDir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := config.Save(config.Default()); err != nil {
				t.Fatalf("save default config: %v", err)
			}

			err := runConfigSet(nil, []string{tt.key, tt.value})

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the value was persisted
			loaded, err := config.Load()
			if err != nil {
				t.Fatalf("Load() after set: %v", err)
			}

			var got string
			switch tt.key {
			case "provider":
				got = loaded.Provider
			case "model":
				got = loaded.Model
			case "ollama.host":
				got = loaded.Ollama.Host
			}
			if got != tt.value {
				t.Errorf("config[%s] = %q after set, want %q", tt.key, got, tt.value)
			}
		})
	}
}

func TestRunConfigSetNoExistingConfig(t *testing.T) {
	// When no config exists, runConfigSet falls back to Default()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	err := runConfigSet(nil, []string{"model", "codellama:7b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load() after set: %v", err)
	}
	if loaded.Model != "codellama:7b" {
		t.Errorf("Model = %q, want %q", loaded.Model, "codellama:7b")
	}
	// Should have default provider since we started from Default()
	if loaded.Provider != config.DefaultProvider {
		t.Errorf("Provider = %q, want default %q", loaded.Provider, config.DefaultProvider)
	}
}
