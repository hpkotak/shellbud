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

func TestOllamaNameAndCapabilities(t *testing.T) {
	p, _ := NewOllama("http://localhost:11434", "test")

	if got := p.Name(); got != "ollama" {
		t.Errorf("Name() = %q, want %q", got, "ollama")
	}

	gotCaps := p.Capabilities()
	wantCaps := Capabilities{
		JSONMode:     true,
		Usage:        true,
		FinishReason: true,
	}
	if gotCaps != wantCaps {
		t.Errorf("Capabilities() = %+v, want %+v", gotCaps, wantCaps)
	}
}

func TestOllamaAvailable(t *testing.T) {
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
					Models: []api.ListModelResponse{{Name: "llama3.2:latest"}},
				}
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

func TestOllamaChat(t *testing.T) {
	tests := []struct {
		name       string
		expectJSON bool
		handler    http.HandlerFunc
		wantText   string
		wantStruct bool
		wantFinish string
		wantUsage  Usage
		wantErr    string
	}{
		{
			name:       "json response with metadata",
			expectJSON: true,
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ChatResponse{
					Message:    api.Message{Role: "assistant", Content: `{"text":"ok","commands":[]}`},
					Done:       true,
					DoneReason: "stop",
					Metrics: api.Metrics{
						PromptEvalCount: 12,
						EvalCount:       5,
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantText:   `{"text":"ok","commands":[]}`,
			wantStruct: true,
			wantFinish: "stop",
			wantUsage: Usage{
				InputTokens:  12,
				OutputTokens: 5,
				TotalTokens:  17,
			},
		},
		{
			name:       "plain response with expect json disabled",
			expectJSON: false,
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := api.ChatResponse{
					Message: api.Message{Role: "assistant", Content: "ls -la"},
					Done:    true,
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantText:   "ls -la",
			wantStruct: false,
		},
		{
			name:       "empty response",
			expectJSON: true,
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
			name:       "server error",
			expectJSON: true,
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "model not loaded", http.StatusInternalServerError)
			},
			wantErr: "ollama chat",
		},
		{
			name:       "context cancelled",
			expectJSON: true,
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

			got, err := p.Chat(ctx, ChatRequest{
				Messages: []Message{
					{Role: "system", Content: "you are a test assistant"},
					{Role: "user", Content: "test query"},
				},
				ExpectJSON: tt.expectJSON,
			})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Chat() unexpected error: %v", err)
				}
				if got.Text != tt.wantText {
					t.Errorf("Text = %q, want %q", got.Text, tt.wantText)
				}
				if got.Raw != tt.wantText {
					t.Errorf("Raw = %q, want %q", got.Raw, tt.wantText)
				}
				if got.Structured != tt.wantStruct {
					t.Errorf("Structured = %v, want %v", got.Structured, tt.wantStruct)
				}
				if got.FinishReason != tt.wantFinish {
					t.Errorf("FinishReason = %q, want %q", got.FinishReason, tt.wantFinish)
				}
				if got.Usage != tt.wantUsage {
					t.Errorf("Usage = %+v, want %+v", got.Usage, tt.wantUsage)
				}
				return
			}
			if err == nil {
				t.Fatalf("Chat() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Chat() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestOllamaChatPassesMessagesAndFormat(t *testing.T) {
	var receivedReq api.ChatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := api.ChatResponse{
			Message: api.Message{Role: "assistant", Content: `{"text":"ok","commands":[]}`},
			Done:    true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newTestOllama(t, srv.URL, "default-model")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages := []Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "hello"},
	}

	_, err := p.Chat(ctx, ChatRequest{
		Messages:   messages,
		Model:      "override-model",
		ExpectJSON: true,
	})
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if receivedReq.Model != "override-model" {
		t.Errorf("request model = %q, want %q", receivedReq.Model, "override-model")
	}
	if len(receivedReq.Messages) != len(messages) {
		t.Fatalf("received %d messages, want %d", len(receivedReq.Messages), len(messages))
	}
	for i, want := range messages {
		got := receivedReq.Messages[i]
		if got.Role != want.Role || got.Content != want.Content {
			t.Errorf("message[%d] = {%q, %q}, want {%q, %q}", i, got.Role, got.Content, want.Role, want.Content)
		}
	}
	if string(receivedReq.Format) != `"json"` {
		t.Errorf("request format = %s, want %q", string(receivedReq.Format), `"json"`)
	}
}

func TestOllamaChatDisablesFormatWhenNotExpected(t *testing.T) {
	var receivedReq api.ChatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := api.ChatResponse{
			Message: api.Message{Role: "assistant", Content: "ls -la"},
			Done:    true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newTestOllama(t, srv.URL, "default-model")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := p.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if len(receivedReq.Format) != 0 {
		t.Errorf("request format = %q, want empty", string(receivedReq.Format))
	}
}
