package config

import (
	"os"
	"path/filepath"
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
