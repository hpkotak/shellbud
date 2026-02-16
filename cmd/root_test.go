package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/hpkotak/shellbud/internal/provider"
)

// mockProvider implements provider.Provider with configurable return values.
type mockProvider struct {
	chatResult string
	chatErr    error
}

func (m *mockProvider) Chat(_ context.Context, _ provider.ChatRequest) (provider.ChatResponse, error) {
	if m.chatErr != nil {
		return provider.ChatResponse{}, m.chatErr
	}
	return provider.ChatResponse{
		Text: m.chatResult,
		Raw:  m.chatResult,
	}, nil
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{JSONMode: true}
}
func (m *mockProvider) Available(_ context.Context) error { return nil }

// saveCmdVars saves the package-level function vars and returns a restore function.
func saveCmdVars(t *testing.T) func() {
	t.Helper()
	origNewProvider := newProvider
	origRunCommand := runCommand
	origIoIn := ioIn
	origIoOut := ioOut
	origModelFlag := modelFlag
	return func() {
		newProvider = origNewProvider
		runCommand = origRunCommand
		ioIn = origIoIn
		ioOut = origIoOut
		modelFlag = origModelFlag
	}
}

// setupMalformedConfig writes invalid YAML to the config path so config.Load() returns a parse error.
func setupMalformedConfig(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	cfgDir := filepath.Join(tmpDir, ".shellbud")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(": bad\n\t:"), 0o644); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}
}

// setupTestConfig creates a temp config file and sets HOME so config.Load() works.
func setupTestConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	cfgDir := filepath.Join(tmpDir, ".shellbud")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func TestRunTranslate(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		hasConfig       bool
		malformedConfig bool
		mock            *mockProvider
		input           string // user confirmation input
		runErr          error  // error from runCommand
		wantErr         string
		wantInOut       string // substring expected in output
		wantRun         bool   // expect runCommand to be called
	}{
		{
			name:    "no config",
			args:    []string{"compress", "folder"},
			mock:    &mockProvider{chatResult: "```bash\ntar -czvf folder.tar.gz folder\n```"},
			wantErr: "sb setup",
		},
		{
			name:            "malformed config",
			args:            []string{"list", "files"},
			malformedConfig: true,
			mock:            &mockProvider{chatResult: `{"text":"test","commands":[]}`},
			wantErr:         "parsing config",
		},
		{
			name:      "safe command, user confirms",
			args:      []string{"list", "files"},
			hasConfig: true,
			mock:      &mockProvider{chatResult: `{"text":"Here you go.","commands":["ls -la"]}`},
			input:     "y\n",
			wantInOut: "ls -la",
			wantRun:   true,
		},
		{
			name:      "invalid plain text response is fail closed",
			args:      []string{"list", "files"},
			hasConfig: true,
			mock:      &mockProvider{chatResult: "ls -la"},
			input:     "",
			wantInOut: "not valid structured output",
			wantRun:   false,
		},
		{
			name:      "safe command, user declines",
			args:      []string{"list", "files"},
			hasConfig: true,
			mock:      &mockProvider{chatResult: `{"text":"Try this.","commands":["ls -la"]}`},
			input:     "n\n",
			wantInOut: "Skipped",
		},
		{
			name:      "invalid fenced response is fail closed",
			args:      []string{"show", "example"},
			hasConfig: true,
			mock:      &mockProvider{chatResult: "```text\nls -la\n```"},
			input:     "",
			wantInOut: "not valid structured output",
			wantRun:   false,
		},
		{
			name:      "destructive command, user confirms",
			args:      []string{"delete", "files"},
			hasConfig: true,
			mock:      &mockProvider{chatResult: `{"text":"This will remove files.","commands":["rm -rf /tmp/test"]}`},
			input:     "y\n",
			wantInOut: "Warning: this is a destructive command",
			wantRun:   true,
		},
		{
			name:      "destructive command, user declines",
			args:      []string{"delete", "files"},
			hasConfig: true,
			mock:      &mockProvider{chatResult: `{"text":"This will remove files.","commands":["rm -rf /tmp/test"]}`},
			input:     "n\n",
			wantInOut: "Skipped",
		},
		{
			name:      "provider error",
			args:      []string{"translate", "something"},
			hasConfig: true,
			mock:      &mockProvider{chatErr: fmt.Errorf("model not available")},
			wantErr:   "query failed",
		},
		{
			name:      "run command error",
			args:      []string{"list", "files"},
			hasConfig: true,
			mock:      &mockProvider{chatResult: `{"text":"Try this.","commands":["ls -la"]}`},
			input:     "y\n",
			runErr:    fmt.Errorf("exit status 1"),
			wantErr:   "exit status 1",
			wantRun:   true,
		},
		{
			name:      "text only response, no commands",
			args:      []string{"what", "time", "is", "it"},
			hasConfig: true,
			mock:      &mockProvider{chatResult: `{"text":"I can't tell the current time directly.","commands":[]}`},
			wantInOut: "I can't tell the current time directly.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := saveCmdVars(t)
			defer restore()

			switch {
			case tt.hasConfig:
				setupTestConfig(t, config.Default())
			case tt.malformedConfig:
				setupMalformedConfig(t)
			default:
				t.Setenv("HOME", t.TempDir()) // no config
			}

			newProvider = func(cfg *config.Config, model string) (provider.Provider, error) {
				return tt.mock, nil
			}

			ranCommand := false
			runErr := tt.runErr
			runCommand = func(cmd string) error {
				ranCommand = true
				return runErr
			}

			ioIn = strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			ioOut = out

			cmd := rootCmd
			err := runTranslate(cmd, tt.args)

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
			if tt.wantInOut != "" && !strings.Contains(out.String(), tt.wantInOut) {
				t.Errorf("output = %q, want substring %q", out.String(), tt.wantInOut)
			}

			if tt.wantRun && !ranCommand {
				t.Error("expected runCommand to be called, but it wasn't")
			}
			if !tt.wantRun && ranCommand {
				t.Error("runCommand was called but shouldn't have been")
			}
		})
	}
}

func TestRunTranslateModelFlag(t *testing.T) {
	restore := saveCmdVars(t)
	defer restore()

	setupTestConfig(t, config.Default())

	var capturedModel string
	newProvider = func(cfg *config.Config, model string) (provider.Provider, error) {
		capturedModel = model
		return &mockProvider{chatResult: `{"text":"test","commands":["echo test"]}`}, nil
	}
	runCommand = func(cmd string) error { return nil }
	ioIn = strings.NewReader("y\n")
	ioOut = io.Discard

	modelFlag = "custom-model:latest"
	err := runTranslate(rootCmd, []string{"test", "query"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedModel != "custom-model:latest" {
		t.Errorf("model = %q, want %q", capturedModel, "custom-model:latest")
	}
}

func TestRunTranslatePassesConfiguredProvider(t *testing.T) {
	restore := saveCmdVars(t)
	defer restore()

	cfg := config.Default()
	cfg.Provider = "openai"
	setupTestConfig(t, cfg)

	var capturedProvider string
	newProvider = func(cfg *config.Config, model string) (provider.Provider, error) {
		capturedProvider = cfg.Provider
		return &mockProvider{chatResult: `{"text":"test","commands":[]}`}, nil
	}
	ioIn = strings.NewReader("")
	ioOut = io.Discard

	if err := runTranslate(rootCmd, []string{"test"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedProvider != "openai" {
		t.Errorf("provider = %q, want %q", capturedProvider, "openai")
	}
}

func TestCreateProvider(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.Config
		envAPIKey string
		wantName  string
		wantErr   string
	}{
		{
			name: "ollama provider",
			cfg: &config.Config{
				Provider: "ollama",
				Model:    "llama3.2:latest",
				Ollama:   config.Ollama{Host: "http://localhost:11434"},
			},
			wantName: "ollama",
		},
		{
			name: "openai provider",
			cfg: &config.Config{
				Provider: "openai",
				Model:    "gpt-4o-mini",
				OpenAI:   config.OpenAI{Host: "https://api.openai.com/v1"},
			},
			envAPIKey: "test-key",
			wantName:  "openai",
		},
		{
			name: "afm provider",
			cfg: &config.Config{
				Provider: "afm",
				Model:    "afm-latest",
				AFM:      config.AFM{Command: "afm-bridge"},
			},
			wantName: "afm",
		},
		{
			name: "openai missing key",
			cfg: &config.Config{
				Provider: "openai",
				Model:    "gpt-4o-mini",
				OpenAI:   config.OpenAI{Host: "https://api.openai.com/v1"},
			},
			wantErr: "OPENAI_API_KEY",
		},
		{
			name: "unsupported provider",
			cfg: &config.Config{
				Provider: "unknown",
				Model:    "model",
			},
			wantErr: "unsupported provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENAI_API_KEY", tt.envAPIKey)

			got, err := createProvider(tt.cfg, tt.cfg.Model)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("createProvider() unexpected error: %v", err)
				}
				if got.Name() != tt.wantName {
					t.Errorf("provider name = %q, want %q", got.Name(), tt.wantName)
				}
				return
			}
			if err == nil {
				t.Fatalf("createProvider() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
