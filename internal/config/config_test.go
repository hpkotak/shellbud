package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	// Use a temp dir to avoid touching the real config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		Provider: "ollama",
		Model:    "llama3.2:latest",
		Ollama:   Ollama{Host: "http://localhost:11434"},
	}

	// Save to temp path
	data, err := marshalConfig(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Load from temp path
	loaded, err := loadFrom(configPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Provider != cfg.Provider {
		t.Errorf("Provider = %q, want %q", loaded.Provider, cfg.Provider)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, cfg.Model)
	}
	if loaded.Ollama.Host != cfg.Ollama.Host {
		t.Errorf("Ollama.Host = %q, want %q", loaded.Ollama.Host, cfg.Ollama.Host)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := loadFrom("/nonexistent/path/config.yaml")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid config",
			cfg:  Config{Provider: "ollama", Model: "llama3.2:latest", Ollama: Ollama{Host: "http://localhost:11434"}},
		},
		{
			name:    "invalid provider",
			cfg:     Config{Provider: "openai", Model: "gpt-4", Ollama: Ollama{Host: "http://localhost:11434"}},
			wantErr: "invalid provider",
		},
		{
			name:    "empty provider",
			cfg:     Config{Provider: "", Model: "llama3.2:latest", Ollama: Ollama{Host: "http://localhost:11434"}},
			wantErr: "invalid provider",
		},
		{
			name:    "empty model",
			cfg:     Config{Provider: "ollama", Model: "", Ollama: Ollama{Host: "http://localhost:11434"}},
			wantErr: "model cannot be empty",
		},
		{
			name: "valid https host",
			cfg:  Config{Provider: "ollama", Model: "llama3.2:latest", Ollama: Ollama{Host: "https://ollama.example.com"}},
		},
		{
			name: "valid host with port",
			cfg:  Config{Provider: "ollama", Model: "llama3.2:latest", Ollama: Ollama{Host: "http://192.168.1.100:11434"}},
		},
		{
			// url.Parse is very permissive — empty string parses without error.
			// This documents current behavior rather than asserting it should fail.
			name: "empty host accepted by url.Parse",
			cfg:  Config{Provider: "ollama", Model: "llama3.2:latest", Ollama: Ollama{Host: ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Provider != DefaultProvider {
		t.Errorf("Provider = %q, want %q", cfg.Provider, DefaultProvider)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("Model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.Ollama.Host != DefaultHost {
		t.Errorf("Ollama.Host = %q, want %q", cfg.Ollama.Host, DefaultHost)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Default() config fails Validate(): %v", err)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if Exists() {
		t.Error("Exists() = true before any config saved")
	}

	// Create the config file
	cfgDir := filepath.Join(tmpDir, ".shellbud")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := marshalConfig(Default())
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if !Exists() {
		t.Error("Exists() = false after config created")
	}
}

func TestSaveCreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := Default()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify directory exists
	dirInfo, err := os.Stat(Dir())
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !dirInfo.IsDir() {
		t.Error("config dir is not a directory")
	}

	// Verify file exists and is readable
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if loaded.Provider != cfg.Provider {
		t.Errorf("Provider = %q, want %q", loaded.Provider, cfg.Provider)
	}
}

func TestLoadCorruptedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(path, []byte("{{{{not valid yaml"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := loadFrom(path)
	if err == nil {
		t.Error("loadFrom() expected error for corrupted YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("error = %q, want substring %q", err.Error(), "parsing config")
	}
}

func TestLoadExtraFields(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	// YAML with an extra field that doesn't exist in Config struct
	yamlData := `provider: ollama
model: llama3.2:latest
ollama:
  host: http://localhost:11434
future_field: some_value
`
	if err := os.WriteFile(path, []byte(yamlData), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom() error: %v", err)
	}
	if cfg.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "ollama")
	}
}
