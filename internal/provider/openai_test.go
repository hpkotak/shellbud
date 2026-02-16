package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestOpenAI(t *testing.T, serverURL, model string) *OpenAIProvider {
	t.Helper()
	p, err := NewOpenAI(serverURL, model, "test-key")
	if err != nil {
		t.Fatalf("NewOpenAI(%q, %q): %v", serverURL, model, err)
	}
	return p
}

func TestNewOpenAI(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		model   string
		key     string
		wantErr string
	}{
		{name: "valid", host: "https://api.openai.com/v1", model: "gpt-4o-mini", key: "sk-test"},
		{name: "empty host", host: "", model: "gpt-4o-mini", key: "sk-test", wantErr: "host cannot be empty"},
		{name: "invalid host", host: "://broken", model: "gpt-4o-mini", key: "sk-test", wantErr: "parsing openai host URL"},
		{name: "empty model", host: "https://api.openai.com/v1", model: "", key: "sk-test", wantErr: "model cannot be empty"},
		{name: "empty key", host: "https://api.openai.com/v1", model: "gpt-4o-mini", key: "", wantErr: "OPENAI_API_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewOpenAI(tt.host, tt.model, tt.key)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NewOpenAI() unexpected error: %v", err)
				}
				if p == nil {
					t.Fatal("NewOpenAI() returned nil provider")
				}
				return
			}
			if err == nil {
				t.Fatalf("NewOpenAI() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestOpenAINameAndCapabilities(t *testing.T) {
	p, _ := NewOpenAI("https://api.openai.com/v1", "gpt-4o-mini", "test-key")

	if got := p.Name(); got != "openai" {
		t.Errorf("Name() = %q, want %q", got, "openai")
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

func TestOpenAIAvailable(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name:  "model found",
			model: "gpt-4o-mini",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Errorf("Authorization header = %q, want %q", got, "Bearer test-key")
				}
				resp := map[string]any{
					"data": []map[string]string{
						{"id": "gpt-4o-mini"},
						{"id": "gpt-4.1"},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
		},
		{
			name:  "model missing",
			model: "missing-model",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]any{
					"data": []map[string]string{{"id": "gpt-4o-mini"}},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr: "not found",
		},
		{
			name:  "server error",
			model: "gpt-4o-mini",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			},
			wantErr: "availability check failed",
		},
		{
			name:  "invalid JSON",
			model: "gpt-4o-mini",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("{not-json"))
			},
			wantErr: "decoding openai models response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			p := newTestOpenAI(t, srv.URL, tt.model)
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
				t.Fatalf("Available() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Available() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestOpenAIChat(t *testing.T) {
	var gotModel string
	var gotFormat string
	var gotMessages []map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/chat/completions")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q, want %q", got, "Bearer test-key")
		}
		var req struct {
			Model          string              `json:"model"`
			Messages       []map[string]string `json:"messages"`
			ResponseFormat struct {
				Type string `json:"type"`
			} `json:"response_format,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotModel = req.Model
		gotFormat = req.ResponseFormat.Type
		gotMessages = req.Messages

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"content": `{"text":"ok","commands":[]}`,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     9,
				"completion_tokens": 4,
				"total_tokens":      13,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newTestOpenAI(t, srv.URL, "default-model")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := p.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hello"},
		},
		Model:      "override-model",
		ExpectJSON: true,
	})
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if gotModel != "override-model" {
		t.Errorf("request model = %q, want %q", gotModel, "override-model")
	}
	if gotFormat != "json_object" {
		t.Errorf("response_format.type = %q, want %q", gotFormat, "json_object")
	}
	if len(gotMessages) != 2 {
		t.Fatalf("messages len = %d, want %d", len(gotMessages), 2)
	}
	if gotMessages[1]["content"] != "hello" {
		t.Errorf("messages[1].content = %q, want %q", gotMessages[1]["content"], "hello")
	}

	if got.Text != `{"text":"ok","commands":[]}` {
		t.Errorf("Text = %q, want %q", got.Text, `{"text":"ok","commands":[]}`)
	}
	if got.Raw != got.Text {
		t.Errorf("Raw = %q, want same as Text", got.Raw)
	}
	if !got.Structured {
		t.Error("Structured = false, want true")
	}
	if got.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", got.FinishReason, "stop")
	}
	wantUsage := Usage{InputTokens: 9, OutputTokens: 4, TotalTokens: 13}
	if got.Usage != wantUsage {
		t.Errorf("Usage = %+v, want %+v", got.Usage, wantUsage)
	}
}

func TestOpenAIChatWithoutJSONMode(t *testing.T) {
	var gotFormat bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, gotFormat = req["response_format"]

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{"content": "ls -la"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newTestOpenAI(t, srv.URL, "default-model")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := p.Chat(ctx, ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}
	if gotFormat {
		t.Error("response_format should be omitted when ExpectJSON is false")
	}
	if got.Structured {
		t.Error("Structured = true, want false")
	}
}

func TestOpenAIChatErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "rate limited", http.StatusTooManyRequests)
			},
			wantErr: "openai chat failed",
		},
		{
			name: "invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("{not-json"))
			},
			wantErr: "decoding openai chat response",
		},
		{
			name: "empty choices",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
			},
			wantErr: "empty response from model",
		},
		{
			name: "empty content",
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]any{
					"choices": []map[string]any{
						{"message": map[string]string{"content": "   "}},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr: "empty response from model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			p := newTestOpenAI(t, srv.URL, "gpt-4o-mini")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := p.Chat(ctx, ChatRequest{
				Messages: []Message{{Role: "user", Content: "hello"}},
			})
			if err == nil {
				t.Fatalf("Chat() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Chat() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
