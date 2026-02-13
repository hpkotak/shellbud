package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ollama/ollama/api"
)

// newTestOllama creates an OllamaProvider pointed at a test server.
func newTestOllama(t *testing.T, serverURL, model string) *OllamaProvider {
	t.Helper()
	p, err := NewOllama(serverURL, model)
	if err != nil {
		t.Fatalf("NewOllama(%q, %q): %v", serverURL, model, err)
	}
	return p
}

func TestNewOllama(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		model   string
		wantErr bool
	}{
		{"valid localhost", "http://localhost:11434", "llama3.2:latest", false},
		{"valid https", "https://ollama.example.com:443", "codellama:7b", false},
		// url.Parse is very permissive — empty string and bare paths parse fine.
		{"empty host accepted", "", "llama3.2:latest", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewOllama(tt.host, tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOllama() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && p == nil {
				t.Error("NewOllama() returned nil provider without error")
			}
		})
	}
}

func TestName(t *testing.T) {
	p, _ := NewOllama("http://localhost:11434", "test")
	if got := p.Name(); got != "ollama" {
		t.Errorf("Name() = %q, want %q", got, "ollama")
	}
}

func TestAvailable(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		handler    http.HandlerFunc
		wantErr    string
	}{
		{
			name:  "model found",
			model: "llama3.2:latest",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ListResponse{
					Models: []api.ListModelResponse{
						{Name: "llama3.2:latest"},
						{Name: "codellama:7b"},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
		},
		{
			name:  "model not found",
			model: "missing-model",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ListResponse{
					Models: []api.ListModelResponse{
						{Name: "llama3.2:latest"},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr: "not found",
		},
		{
			name:  "empty model list",
			model: "llama3.2:latest",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ListResponse{Models: []api.ListModelResponse{}}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr: "not found",
		},
		{
			name:  "server error",
			model: "llama3.2:latest",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "internal error", http.StatusInternalServerError)
			},
			wantErr: "cannot reach Ollama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			p := newTestOllama(t, srv.URL, tt.model)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := p.Available(ctx)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Available() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("Available() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Available() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
		wantErr string
	}{
		{
			name: "clean command",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.GenerateResponse{Response: "tar -czvf archive.tar.gz ./folder", Done: true}
				_ = json.NewEncoder(w).Encode(resp)
			},
			want: "tar -czvf archive.tar.gz ./folder",
		},
		{
			name: "code fenced response stripped",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.GenerateResponse{Response: "```bash\nls -la\n```", Done: true}
				_ = json.NewEncoder(w).Encode(resp)
			},
			want: "ls -la",
		},
		{
			name: "empty response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.GenerateResponse{Response: "", Done: true}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr: "empty response from model",
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "model not loaded", http.StatusInternalServerError)
			},
			wantErr: "ollama generate",
		},
		{
			name: "context cancelled",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Delay just long enough for the client context to expire.
				time.Sleep(500 * time.Millisecond)
			},
			wantErr: "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			p := newTestOllama(t, srv.URL, "test-model")

			timeout := 5 * time.Second
			if tt.wantErr == "context deadline exceeded" {
				timeout = 50 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			got, err := p.Translate(ctx, "test query", "darwin", "/bin/zsh")
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Translate() unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("Translate() = %q, want %q", got, tt.want)
				}
				return
			}
			if err == nil {
				t.Errorf("Translate() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Translate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
