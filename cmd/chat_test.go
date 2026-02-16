package cmd

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/hpkotak/shellbud/internal/provider"
)

func TestRunChat(t *testing.T) {
	tests := []struct {
		name       string
		hasConfig  bool
		modelFlag  string
		providerFn func(host, model string) (provider.Provider, error)
		wantModel  string
		wantErr    string
	}{
		{
			name:      "no config",
			hasConfig: false,
			providerFn: func(host, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantErr: "sb setup",
		},
		{
			name:      "provider creation error",
			hasConfig: true,
			providerFn: func(host, model string) (provider.Provider, error) {
				return nil, fmt.Errorf("provider down")
			},
			wantErr: "creating provider: provider down",
		},
		{
			name:      "uses configured model",
			hasConfig: true,
			providerFn: func(host, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantModel: config.DefaultModel,
		},
		{
			name:      "model flag override",
			hasConfig: true,
			modelFlag: "codellama:7b",
			providerFn: func(host, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantModel: "codellama:7b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := saveCmdVars(t)
			defer restore()

			if tt.hasConfig {
				setupTestConfig(t, config.Default())
			} else {
				t.Setenv("HOME", t.TempDir())
			}

			var gotModel string
			newProvider = func(host, model string) (provider.Provider, error) {
				gotModel = model
				return tt.providerFn(host, model)
			}
			modelFlag = tt.modelFlag
			ioIn = strings.NewReader("exit\n")
			ioOut = &bytes.Buffer{}

			err := runChat(rootCmd, nil)
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
			if tt.wantModel != "" && gotModel != tt.wantModel {
				t.Errorf("model = %q, want %q", gotModel, tt.wantModel)
			}
		})
	}
}

func TestExecute(t *testing.T) {
	restore := saveCmdVars(t)
	defer restore()

	origOut := rootCmd.OutOrStdout()
	origErr := rootCmd.ErrOrStderr()
	defer rootCmd.SetOut(origOut)
	defer rootCmd.SetErr(origErr)

	rootCmd.SetArgs([]string{"--help"})
	defer rootCmd.SetArgs(nil)

	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(io.Discard)

	if err := Execute(); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "ShellBud (sb)") {
		t.Errorf("help output missing expected content, got:\n%s", buf.String())
	}
}
