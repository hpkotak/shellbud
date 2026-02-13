package setup

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/ollama/ollama/api"
)

// saveFuncVars saves the current package-level function vars and returns
// a restore function. Call restore in a defer.
func saveFuncVars(t *testing.T) func() {
	t.Helper()
	origLookPath := lookPath
	origExecCommand := execCommand
	origPlatformOS := platformOS
	return func() {
		lookPath = origLookPath
		execCommand = origExecCommand
		platformOS = origPlatformOS
	}
}

// mockOllamaListServer returns an httptest server that responds to /api/tags.
func mockOllamaListServer(models []string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/tags"):
			resp := api.ListResponse{}
			for _, m := range models {
				resp.Models = append(resp.Models, api.ListModelResponse{Name: m})
			}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/api/pull"):
			// Simulate a successful pull with one progress response.
			resp := api.ProgressResponse{Status: "success", Total: 100, Completed: 100}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusOK) // health check
		}
	}))
}

func TestReadLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal input", "hello\n", "hello"},
		{"whitespace trimming", "  spaces  \n", "spaces"},
		{"empty line", "\n", ""},
		{"multi-line reads first", "first\nsecond\n", "first"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readLine(strings.NewReader(tt.input))
			if got != tt.want {
				t.Errorf("readLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsOllamaReachable(t *testing.T) {
	t.Run("server returns 200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		if !isOllamaReachable(srv.URL) {
			t.Error("isOllamaReachable() = false, want true")
		}
	})

	t.Run("server returns 500", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		if isOllamaReachable(srv.URL) {
			t.Error("isOllamaReachable() = true, want false")
		}
	})

	t.Run("server closed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		srv.Close()

		if isOllamaReachable(srv.URL) {
			t.Error("isOllamaReachable() = true for closed server, want false")
		}
	})
}

func TestOllamaClient(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		client, err := ollamaClient("http://localhost:11434")
		if err != nil {
			t.Errorf("ollamaClient() unexpected error: %v", err)
		}
		if client == nil {
			t.Error("ollamaClient() returned nil")
		}
	})

	t.Run("empty URL accepted", func(t *testing.T) {
		// url.Parse accepts empty strings
		_, err := ollamaClient("")
		if err != nil {
			t.Errorf("ollamaClient(\"\") unexpected error: %v", err)
		}
	})
}

func TestSelectModel(t *testing.T) {
	tests := []struct {
		name    string
		models  []string
		input   string
		want    string
		wantErr string
	}{
		{
			name:   "select first model",
			models: []string{"llama3.2:latest", "codellama:7b"},
			input:  "1\n",
			want:   "llama3.2:latest",
		},
		{
			name:   "select second model",
			models: []string{"llama3.2:latest", "codellama:7b"},
			input:  "2\n",
			want:   "codellama:7b",
		},
		{
			name:   "enter selects default (first)",
			models: []string{"llama3.2:latest", "codellama:7b"},
			input:  "\n",
			want:   "llama3.2:latest",
		},
		{
			name:    "invalid number",
			models:  []string{"llama3.2:latest"},
			input:   "5\n",
			wantErr: "invalid selection",
		},
		{
			name:    "non-numeric input",
			models:  []string{"llama3.2:latest"},
			input:   "abc\n",
			wantErr: "invalid selection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := mockOllamaListServer(tt.models)
			defer srv.Close()

			client, err := ollamaClient(srv.URL)
			if err != nil {
				t.Fatalf("ollamaClient: %v", err)
			}

			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			got, err := selectModel(client, in, out)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("selectModel() expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectModel() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("selectModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRecommendedModel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{"select llama3.2:3b", "1\n", "llama3.2:3b", ""},
		{"default selects llama3.2:3b", "\n", "llama3.2:3b", ""},
		{"select codellama:7b", "2\n", "codellama:7b", ""},
		{"skip", "3\n", "", "no model selected"},
		{"invalid input", "xyz\n", "", "invalid selection"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := mockOllamaListServer(nil)
			defer srv.Close()

			client, err := ollamaClient(srv.URL)
			if err != nil {
				t.Fatalf("ollamaClient: %v", err)
			}

			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			got, err := pullRecommendedModel(client, in, out)

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
			if got != tt.want {
				t.Errorf("pullRecommendedModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnsureOllamaInstalled(t *testing.T) {
	tests := []struct {
		name      string
		found     bool   // whether ollama is in PATH
		os        string // mock platform OS
		input     string // user input for confirmation
		wantErr   string
		wantInOut string // substring expected in output
	}{
		{
			name:      "already installed",
			found:     true,
			os:        "darwin",
			wantInOut: "[ok] Ollama is installed",
		},
		{
			name:    "not installed, unsupported OS",
			found:   false,
			os:      "windows",
			wantErr: "unsupported platform",
		},
		{
			name:    "not installed, darwin, user declines",
			found:   false,
			os:      "darwin",
			input:   "n\n",
			wantErr: "ollama is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := saveFuncVars(t)
			defer restore()

			if tt.found {
				lookPath = func(file string) (string, error) {
					return "/usr/local/bin/ollama", nil
				}
			} else {
				lookPath = func(file string) (string, error) {
					return "", exec.ErrNotFound
				}
			}
			platformOS = func() string { return tt.os }

			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			err := ensureOllamaInstalled(in, out)

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
		})
	}
}

func TestEnsureOllamaRunning(t *testing.T) {
	t.Run("already reachable", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		out := &bytes.Buffer{}
		err := ensureOllamaRunning(srv.URL, strings.NewReader(""), out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "[ok] Ollama is running") {
			t.Errorf("output = %q, want substring %q", out.String(), "[ok] Ollama is running")
		}
	})

	t.Run("not reachable, user declines", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		srv.Close() // closed = unreachable

		out := &bytes.Buffer{}
		err := ensureOllamaRunning(srv.URL, strings.NewReader("n\n"), out)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ollama must be running") {
			t.Errorf("error = %q, want substring %q", err.Error(), "ollama must be running")
		}
	})
}
