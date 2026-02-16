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
		{"set second valid provider", "provider", "openai", ""},
		{"set third valid provider", "provider", "afm", ""},
		{"set invalid provider", "provider", "anthropic", "invalid provider"},
		{"set valid model", "model", "codellama:7b", ""},
		{"set empty model after trim", "model", "   ", "model cannot be empty"},
		{"set valid host", "ollama.host", "http://192.168.1.100:11434", ""},
		{"set invalid host", "ollama.host", "://broken", "invalid URL"},
		{"set valid openai host", "openai.host", "https://api.openai.com/v1", ""},
		{"set invalid openai host", "openai.host", "://broken", "invalid URL"},
		{"set valid afm command", "afm.command", "/usr/local/bin/afm-bridge", ""},
		{"set invalid afm command", "afm.command", "", "afm command cannot be empty"},
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
			case "openai.host":
				got = loaded.OpenAI.Host
			case "afm.command":
				got = loaded.AFM.Command
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

func TestRunConfigSetMalformedConfig(t *testing.T) {
	// When config file contains invalid YAML, config set should refuse (not silently reset).
	setupMalformedConfig(t)

	err := runConfigSet(nil, []string{"model", "codellama:7b"})
	if err == nil {
		t.Fatal("expected error for malformed config, got nil")
	}
	if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("error = %q, want substring %q", err.Error(), "parsing config")
	}
}

func TestRunConfigSetModelTrimmed(t *testing.T) {
	// Whitespace in model values should be trimmed before persisting.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	cfgDir := filepath.Join(tmpDir, ".shellbud")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.Save(config.Default()); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	if err := runConfigSet(nil, []string{"model", "  llama3.2:latest  "}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load() after set: %v", err)
	}
	if loaded.Model != "llama3.2:latest" {
		t.Errorf("Model = %q, want %q (trimmed)", loaded.Model, "llama3.2:latest")
	}
}

func TestRunConfigSetProviderSetsMissingDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Simulate an old config without openai/afm sections.
	raw := `provider: ollama
model: llama3.2:latest
ollama:
  host: http://localhost:11434
`
	cfgDir := filepath.Join(tmpDir, ".shellbud")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := runConfigSet(nil, []string{"provider", "openai"}); err != nil {
		t.Fatalf("set provider openai: %v", err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.OpenAI.Host != config.DefaultOpenAIHost {
		t.Errorf("OpenAI.Host = %q, want %q", loaded.OpenAI.Host, config.DefaultOpenAIHost)
	}

	if err := runConfigSet(nil, []string{"provider", "afm"}); err != nil {
		t.Fatalf("set provider afm: %v", err)
	}
	loaded, err = config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.AFM.Command != config.DefaultAFMCommand {
		t.Errorf("AFM.Command = %q, want %q", loaded.AFM.Command, config.DefaultAFMCommand)
	}
}
