// Package config manages the shellbud configuration file at ~/.shellbud/config.yaml.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Defaults and valid values — single source of truth for the codebase.
const (
	DefaultHost     = "http://localhost:11434"
	DefaultProvider = "ollama"
	DefaultModel    = "llama3.2:latest"
)

var ValidProviders = []string{"ollama"}

var ErrNotFound = errors.New("config file not found")

type Config struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	Ollama   Ollama `yaml:"ollama"`
}

type Ollama struct {
	Host string `yaml:"host"`
}

// Validate checks that config values are valid.
func (c *Config) Validate() error {
	if !isValidProvider(c.Provider) {
		return fmt.Errorf("invalid provider %q (valid: %v)", c.Provider, ValidProviders)
	}
	if c.Model == "" {
		return fmt.Errorf("model cannot be empty")
	}
	if _, err := url.Parse(c.Ollama.Host); err != nil {
		return fmt.Errorf("invalid ollama host URL %q: %w", c.Ollama.Host, err)
	}
	return nil
}

func isValidProvider(p string) bool {
	for _, v := range ValidProviders {
		if p == v {
			return true
		}
	}
	return false
}

// Dir returns the config directory path (~/.shellbud).
func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".shellbud")
}

// Path returns the config file path (~/.shellbud/config.yaml).
func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

// Exists checks if the config file exists.
func Exists() bool {
	_, err := os.Stat(Path())
	return err == nil
}

// Load reads and parses the config file. Returns ErrNotFound if it doesn't exist.
func Load() (*Config, error) {
	return loadFrom(Path())
}

func loadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to disk, creating the directory if needed.
func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := marshalConfig(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(Path(), data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func marshalConfig(cfg *Config) ([]byte, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encoding config: %w", err)
	}
	return data, nil
}

// Default returns a config with sensible defaults.
func Default() *Config {
	return &Config{
		Provider: DefaultProvider,
		Model:    DefaultModel,
		Ollama: Ollama{
			Host: DefaultHost,
		},
	}
}
