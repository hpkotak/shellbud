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

func (m *mockProvider) Chat(_ context.Context, _ []provider.Message) (string, error) {
	return m.chatResult, m.chatErr
}

func (m *mockProvider) Name() string                      { return "mock" }
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
		name      string
		args      []string
		hasConfig bool
		mock      *mockProvider
		input     string // user confirmation input
		runErr    error  // error from runCommand
		wantErr   string
		wantInOut string // substring expected in output
		wantRun   bool   // expect runCommand to be called
	}{
		{
			name:    "no config",
			args:    []string{"compress", "folder"},
			mock:    &mockProvider{chatResult: "```bash\ntar -czvf folder.tar.gz folder\n```"},
			wantErr: "sb setup",
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

			if tt.hasConfig {
				setupTestConfig(t, config.Default())
			} else {
				t.Setenv("HOME", t.TempDir()) // no config
			}

			newProvider = func(host, model string) (provider.Provider, error) {
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
	newProvider = func(host, model string) (provider.Provider, error) {
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
