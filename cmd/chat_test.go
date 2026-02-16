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
		name            string
		hasConfig       bool
		malformedConfig bool
		provider        string
		modelFlag       string
		providerFn      func(cfg *config.Config, model string) (provider.Provider, error)
		wantModel       string
		wantProv        string
		wantErr         string
	}{
		{
			name:      "no config",
			hasConfig: false,
			providerFn: func(cfg *config.Config, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantErr: "sb setup",
		},
		{
			name:            "malformed config",
			malformedConfig: true,
			providerFn: func(cfg *config.Config, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantErr: "parsing config",
		},
		{
			name:      "provider creation error",
			hasConfig: true,
			providerFn: func(cfg *config.Config, model string) (provider.Provider, error) {
				return nil, fmt.Errorf("provider down")
			},
			wantErr: "creating provider: provider down",
		},
		{
			name:      "uses configured model",
			hasConfig: true,
			providerFn: func(cfg *config.Config, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantModel: config.DefaultModel,
		},
		{
			name:      "uses configured provider",
			hasConfig: true,
			provider:  "openai",
			providerFn: func(cfg *config.Config, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantProv: "openai",
		},
		{
			name:      "uses afm provider",
			hasConfig: true,
			provider:  "afm",
			providerFn: func(cfg *config.Config, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantProv: "afm",
		},
		{
			name:      "model flag override",
			hasConfig: true,
			modelFlag: "codellama:7b",
			providerFn: func(cfg *config.Config, model string) (provider.Provider, error) {
				return &mockProvider{}, nil
			},
			wantModel: "codellama:7b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := saveCmdVars(t)
			defer restore()

			switch {
			case tt.hasConfig:
				cfg := config.Default()
				if tt.provider != "" {
					cfg.Provider = tt.provider
				}
				setupTestConfig(t, cfg)
			case tt.malformedConfig:
				setupMalformedConfig(t)
			default:
				t.Setenv("HOME", t.TempDir())
			}

			var gotModel string
			var gotProvider string
			newProvider = func(cfg *config.Config, model string) (provider.Provider, error) {
				gotModel = model
				gotProvider = cfg.Provider
				return tt.providerFn(cfg, model)
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
			if tt.wantProv != "" && gotProvider != tt.wantProv {
				t.Errorf("provider = %q, want %q", gotProvider, tt.wantProv)
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
