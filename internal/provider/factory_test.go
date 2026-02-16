package provider

import (
	"strings"
	"testing"
)

func TestNewFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      BuildConfig
		want     string
		wantCaps Capabilities
		wantErr  string
	}{
		{
			name: "ollama",
			cfg: BuildConfig{
				Name:       "ollama",
				Model:      "llama3.2:latest",
				OllamaHost: "http://localhost:11434",
			},
			want: "ollama",
			wantCaps: Capabilities{
				JSONMode:     true,
				Usage:        true,
				FinishReason: true,
			},
		},
		{
			name: "openai",
			cfg: BuildConfig{
				Name:         "openai",
				Model:        "gpt-4o-mini",
				OpenAIHost:   "https://api.openai.com/v1",
				OpenAIAPIKey: "test-key",
			},
			want: "openai",
			wantCaps: Capabilities{
				JSONMode:     true,
				Usage:        true,
				FinishReason: true,
			},
		},
		{
			name: "afm",
			cfg: BuildConfig{
				Name:       "afm",
				Model:      "afm-latest",
				AFMCommand: "afm-bridge",
			},
			want: "afm",
			wantCaps: Capabilities{
				JSONMode:     true,
				Usage:        false,
				FinishReason: false,
			},
		},
		{
			name: "openai missing key",
			cfg: BuildConfig{
				Name:       "openai",
				Model:      "gpt-4o-mini",
				OpenAIHost: "https://api.openai.com/v1",
			},
			wantErr: "OPENAI_API_KEY",
		},
		{
			name: "unsupported",
			cfg: BuildConfig{
				Name:  "unknown",
				Model: "model",
			},
			wantErr: "unsupported provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewFromConfig(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NewFromConfig() unexpected error: %v", err)
				}
				if got.Name() != tt.want {
					t.Errorf("provider name = %q, want %q", got.Name(), tt.want)
				}
				if got.Capabilities() != tt.wantCaps {
					t.Errorf("provider caps = %+v, want %+v", got.Capabilities(), tt.wantCaps)
				}
				return
			}

			if err == nil {
				t.Fatalf("NewFromConfig() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
