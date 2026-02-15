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
		name    string
		model   string
		handler http.HandlerFunc
		wantErr string
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

func TestChat(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
		wantErr string
	}{
		{
			name: "clean response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ChatResponse{
					Message: api.Message{Role: "assistant", Content: "tar -czvf archive.tar.gz ./folder"},
					Done:    true,
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			want: "tar -czvf archive.tar.gz ./folder",
		},
		{
			name: "response with explanation",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ChatResponse{
					Message: api.Message{
						Role:    "assistant",
						Content: "Here's the command:\n```bash\nls -la\n```",
					},
					Done: true,
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			want: "Here's the command:\n```bash\nls -la\n```",
		},
		{
			name: "empty response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ChatResponse{
					Message: api.Message{Role: "assistant", Content: ""},
					Done:    true,
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr: "empty response from model",
		},
		{
			name: "whitespace only response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ChatResponse{
					Message: api.Message{Role: "assistant", Content: "   \n  "},
					Done:    true,
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr: "empty response from model",
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "model not loaded", http.StatusInternalServerError)
			},
			wantErr: "ollama chat",
		},
		{
			name: "context cancelled",
			handler: func(w http.ResponseWriter, r *http.Request) {
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

			messages := []Message{
				{Role: "system", Content: "you are a test assistant"},
				{Role: "user", Content: "test query"},
			}

			got, err := p.Chat(ctx, messages)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Chat() unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("Chat() = %q, want %q", got, tt.want)
				}
				return
			}
			if err == nil {
				t.Errorf("Chat() expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Chat() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestChatPassesMessages(t *testing.T) {
	var receivedMessages []api.Message

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req api.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		receivedMessages = req.Messages

		resp := api.ChatResponse{
			Message: api.Message{Role: "assistant", Content: "ok"},
			Done:    true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newTestOllama(t, srv.URL, "test-model")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages := []Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you"},
	}

	_, err := p.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if len(receivedMessages) != len(messages) {
		t.Fatalf("received %d messages, want %d", len(receivedMessages), len(messages))
	}

	for i, want := range messages {
		got := receivedMessages[i]
		if got.Role != want.Role || got.Content != want.Content {
			t.Errorf("message[%d] = {%q, %q}, want {%q, %q}", i, got.Role, got.Content, want.Role, want.Content)
		}
	}
}
