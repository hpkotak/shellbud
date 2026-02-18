package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/ollama/ollama/api"
)

// saveFuncVars saves the current package-level function vars and returns
// a restore function. Call restore in a defer.
func saveFuncVars(t *testing.T) func() {
	t.Helper()
	origLookPath := lookPath
	origOsStat := osStat
	origExecCommand := execCommand
	origPlatformOS := platformOS
	origEnsureInstalled := ensureInstalled
	origEnsureRunning := ensureRunning
	origNewOllamaClient := newOllamaClient
	origChooseModel := chooseModel
	origSaveConfig := saveConfig
	origConfigPath := configPath
	origReachabilityCheck := reachabilityCheck
	origSleep := sleep
	origCheckAFMAvailability := checkAFMAvailability
	return func() {
		lookPath = origLookPath
		osStat = origOsStat
		execCommand = origExecCommand
		platformOS = origPlatformOS
		ensureInstalled = origEnsureInstalled
		ensureRunning = origEnsureRunning
		newOllamaClient = origNewOllamaClient
		chooseModel = origChooseModel
		saveConfig = origSaveConfig
		configPath = origConfigPath
		reachabilityCheck = origReachabilityCheck
		sleep = origSleep
		checkAFMAvailability = origCheckAFMAvailability
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

func TestRun(t *testing.T) {
	t.Run("success on non-darwin", func(t *testing.T) {
		restore := saveFuncVars(t)
		defer restore()

		platformOS = func() string { return "linux" }
		var gotCfg *config.Config
		ensureInstalled = func(_ io.Reader, _ io.Writer) error { return nil }
		ensureRunning = func(host string, _ io.Reader, _ io.Writer) error {
			if host != config.DefaultOllamaHost {
				t.Fatalf("host = %q, want %q", host, config.DefaultOllamaHost)
			}
			return nil
		}
		newOllamaClient = func(host string) (*api.Client, error) {
			if host != config.DefaultOllamaHost {
				t.Fatalf("host = %q, want %q", host, config.DefaultOllamaHost)
			}
			return &api.Client{}, nil
		}
		chooseModel = func(_ *api.Client, _ io.Reader, _ io.Writer) (string, error) {
			return "llama3.2:3b", nil
		}
		saveConfig = func(cfg *config.Config) error {
			gotCfg = cfg
			return nil
		}
		configPath = func() string { return "/tmp/test-config.yaml" }

		out := &bytes.Buffer{}
		err := Run(strings.NewReader(""), out)
		if err != nil {
			t.Fatalf("Run() unexpected error: %v", err)
		}

		if gotCfg == nil {
			t.Fatal("Run() should save config")
		}
		if gotCfg.Provider != config.DefaultProvider {
			t.Errorf("provider = %q, want %q", gotCfg.Provider, config.DefaultProvider)
		}
		if gotCfg.Model != "llama3.2:3b" {
			t.Errorf("model = %q, want %q", gotCfg.Model, "llama3.2:3b")
		}
		if gotCfg.Ollama.Host != config.DefaultOllamaHost {
			t.Errorf("host = %q, want %q", gotCfg.Ollama.Host, config.DefaultOllamaHost)
		}
		if gotCfg.OpenAI.Host != config.DefaultOpenAIHost {
			t.Errorf("openai host = %q, want %q", gotCfg.OpenAI.Host, config.DefaultOpenAIHost)
		}
		if gotCfg.AFM.Command != config.DefaultAFMCommand {
			t.Errorf("afm command = %q, want %q", gotCfg.AFM.Command, config.DefaultAFMCommand)
		}
		if !strings.Contains(out.String(), "Config saved to /tmp/test-config.yaml") {
			t.Errorf("output should mention saved config path, got:\n%s", out.String())
		}
	})

	tests := []struct {
		name    string
		setFail func()
		wantErr string
	}{
		{
			name: "ensure installed fails",
			setFail: func() {
				ensureInstalled = func(_ io.Reader, _ io.Writer) error {
					return errors.New("install failed")
				}
			},
			wantErr: "install failed",
		},
		{
			name: "ensure running fails",
			setFail: func() {
				ensureRunning = func(string, io.Reader, io.Writer) error {
					return errors.New("not running")
				}
			},
			wantErr: "not running",
		},
		{
			name: "client creation fails",
			setFail: func() {
				newOllamaClient = func(string) (*api.Client, error) {
					return nil, errors.New("bad host")
				}
			},
			wantErr: "bad host",
		},
		{
			name: "model selection fails",
			setFail: func() {
				chooseModel = func(*api.Client, io.Reader, io.Writer) (string, error) {
					return "", errors.New("no model selected")
				}
			},
			wantErr: "no model selected",
		},
		{
			name: "save config fails",
			setFail: func() {
				saveConfig = func(*config.Config) error {
					return errors.New("disk full")
				}
			},
			wantErr: "saving config: disk full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := saveFuncVars(t)
			defer restore()

			platformOS = func() string { return "linux" }
			ensureInstalled = func(_ io.Reader, _ io.Writer) error { return nil }
			ensureRunning = func(string, io.Reader, io.Writer) error { return nil }
			newOllamaClient = func(string) (*api.Client, error) { return &api.Client{}, nil }
			chooseModel = func(*api.Client, io.Reader, io.Writer) (string, error) { return "llama3.2:3b", nil }
			saveConfig = func(*config.Config) error { return nil }
			configPath = func() string { return "/tmp/test-config.yaml" }

			tt.setFail()

			err := Run(strings.NewReader(""), &bytes.Buffer{})
			if err == nil {
				t.Fatalf("Run() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestRunWithAFMOnDarwin tests the full Run() flow on darwin with AFM chosen.
func TestRunWithAFMOnDarwin(t *testing.T) {
	t.Run("AFM chosen, config saved with provider=afm and full bridge path", func(t *testing.T) {
		restore := saveFuncVars(t)
		defer restore()

		platformOS = func() string { return "darwin" }
		lookPath = func(file string) (string, error) {
			if file == "afm-bridge" {
				return "/usr/local/bin/afm-bridge", nil
			}
			return "", exec.ErrNotFound
		}
		checkAFMAvailability = func(_ string) (bool, string) { return true, "" }

		var gotCfg *config.Config
		saveConfig = func(cfg *config.Config) error {
			gotCfg = cfg
			return nil
		}
		configPath = func() string { return "/tmp/test-config.yaml" }

		out := &bytes.Buffer{}
		err := Run(strings.NewReader("1\n"), out)
		if err != nil {
			t.Fatalf("Run() unexpected error: %v", err)
		}

		if gotCfg == nil {
			t.Fatal("Run() should save config")
		}
		if gotCfg.Provider != "afm" {
			t.Errorf("provider = %q, want %q", gotCfg.Provider, "afm")
		}
		if gotCfg.Model != config.DefaultAFMModel {
			t.Errorf("model = %q, want %q", gotCfg.Model, config.DefaultAFMModel)
		}
		if gotCfg.AFM.Command != "/usr/local/bin/afm-bridge" {
			t.Errorf("afm command = %q, want full path %q", gotCfg.AFM.Command, "/usr/local/bin/afm-bridge")
		}
	})

	t.Run("Ollama chosen on darwin falls through to Ollama flow", func(t *testing.T) {
		restore := saveFuncVars(t)
		defer restore()

		platformOS = func() string { return "darwin" }
		lookPath = func(file string) (string, error) {
			if file == "afm-bridge" {
				return "/usr/local/bin/afm-bridge", nil
			}
			return "/usr/local/bin/ollama", nil
		}
		checkAFMAvailability = func(_ string) (bool, string) { return true, "" }
		ensureInstalled = func(_ io.Reader, _ io.Writer) error { return nil }
		ensureRunning = func(string, io.Reader, io.Writer) error { return nil }
		newOllamaClient = func(string) (*api.Client, error) { return &api.Client{}, nil }
		chooseModel = func(*api.Client, io.Reader, io.Writer) (string, error) { return "llama3.2:3b", nil }

		var gotCfg *config.Config
		saveConfig = func(cfg *config.Config) error {
			gotCfg = cfg
			return nil
		}
		configPath = func() string { return "/tmp/test-config.yaml" }

		out := &bytes.Buffer{}
		// User selects "2" for Ollama.
		err := Run(strings.NewReader("2\n"), out)
		if err != nil {
			t.Fatalf("Run() unexpected error: %v", err)
		}

		if gotCfg == nil {
			t.Fatal("Run() should save config")
		}
		if gotCfg.Provider != "ollama" {
			t.Errorf("provider = %q, want %q", gotCfg.Provider, "ollama")
		}
	})
}

// TestOfferProviderChoice tests the provider selection logic.
func TestOfferProviderChoice(t *testing.T) {
	tests := []struct {
		name           string
		bridgeFound    bool
		afmAvailable   bool
		afmReason      string
		input          string
		wantProvider   string
		wantBridgePath string // non-empty only when wantProvider=="afm"
		wantErr        string
	}{
		{
			name:         "bridge not found → ollama",
			bridgeFound:  false,
			wantProvider: "ollama",
		},
		{
			name:         "bridge found but AFM unavailable → ollama",
			bridgeFound:  true,
			afmAvailable: false,
			afmReason:    "device_not_eligible",
			wantProvider: "ollama",
		},
		{
			name:           "AFM available, user picks AFM (default)",
			bridgeFound:    true,
			afmAvailable:   true,
			input:          "\n",
			wantProvider:   "afm",
			wantBridgePath: "/usr/local/bin/afm-bridge",
		},
		{
			name:           "AFM available, user picks AFM explicitly",
			bridgeFound:    true,
			afmAvailable:   true,
			input:          "1\n",
			wantProvider:   "afm",
			wantBridgePath: "/usr/local/bin/afm-bridge",
		},
		{
			name:         "AFM available, user picks Ollama",
			bridgeFound:  true,
			afmAvailable: true,
			input:        "2\n",
			wantProvider: "ollama",
		},
		{
			name:         "all invalid selections exhausts retries → error",
			bridgeFound:  true,
			afmAvailable: true,
			input:        "9\n8\n7\n",
			wantErr:      "no valid selection",
		},
		{
			name:           "invalid then valid selection → succeeds",
			bridgeFound:    true,
			afmAvailable:   true,
			input:          "9\n1\n",
			wantProvider:   "afm",
			wantBridgePath: "/usr/local/bin/afm-bridge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := saveFuncVars(t)
			defer restore()

			if tt.bridgeFound {
				lookPath = func(file string) (string, error) {
					if file == "afm-bridge" {
						return "/usr/local/bin/afm-bridge", nil
					}
					return "", exec.ErrNotFound
				}
				checkAFMAvailability = func(_ string) (bool, string) {
					return tt.afmAvailable, tt.afmReason
				}
			} else {
				lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
				osStat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
			}

			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			gotProvider, gotBridgePath, err := offerProviderChoice(in, out)

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
			if gotProvider != tt.wantProvider {
				t.Errorf("provider = %q, want %q", gotProvider, tt.wantProvider)
			}
			if tt.wantBridgePath != "" && gotBridgePath != tt.wantBridgePath {
				t.Errorf("bridgePath = %q, want %q", gotBridgePath, tt.wantBridgePath)
			}
			if tt.wantProvider != "afm" && gotBridgePath != "" {
				t.Errorf("bridgePath should be empty for provider=%q, got %q", tt.wantProvider, gotBridgePath)
			}
		})
	}
}

// TestSetupAFM verifies that setupAFM saves the correct config values.
func TestSetupAFM(t *testing.T) {
	t.Run("saves full bridge path", func(t *testing.T) {
		restore := saveFuncVars(t)
		defer restore()

		var gotCfg *config.Config
		saveConfig = func(cfg *config.Config) error {
			gotCfg = cfg
			return nil
		}
		configPath = func() string { return "/tmp/test-config.yaml" }

		out := &bytes.Buffer{}
		if err := setupAFM("/usr/local/bin/afm-bridge", out); err != nil {
			t.Fatalf("setupAFM() unexpected error: %v", err)
		}

		if gotCfg == nil {
			t.Fatal("setupAFM() should save config")
		}
		if gotCfg.Provider != "afm" {
			t.Errorf("provider = %q, want %q", gotCfg.Provider, "afm")
		}
		if gotCfg.Model != config.DefaultAFMModel {
			t.Errorf("model = %q, want %q", gotCfg.Model, config.DefaultAFMModel)
		}
		if gotCfg.AFM.Command != "/usr/local/bin/afm-bridge" {
			t.Errorf("afm command = %q, want %q", gotCfg.AFM.Command, "/usr/local/bin/afm-bridge")
		}
		if gotCfg.Ollama.Host != config.DefaultOllamaHost {
			t.Errorf("ollama host = %q, want %q", gotCfg.Ollama.Host, config.DefaultOllamaHost)
		}
		if !strings.Contains(out.String(), "Config saved") {
			t.Errorf("output should mention config saved, got:\n%s", out.String())
		}
	})

	t.Run("falls back to DefaultAFMCommand when path is empty", func(t *testing.T) {
		restore := saveFuncVars(t)
		defer restore()

		var gotCfg *config.Config
		saveConfig = func(cfg *config.Config) error { gotCfg = cfg; return nil }
		configPath = func() string { return "/tmp/test-config.yaml" }

		if err := setupAFM("", &bytes.Buffer{}); err != nil {
			t.Fatalf("setupAFM() unexpected error: %v", err)
		}
		if gotCfg.AFM.Command != config.DefaultAFMCommand {
			t.Errorf("afm command = %q, want default %q", gotCfg.AFM.Command, config.DefaultAFMCommand)
		}
	})
}

// TestSetupAFMSaveError verifies error propagation from saveConfig.
func TestSetupAFMSaveError(t *testing.T) {
	restore := saveFuncVars(t)
	defer restore()

	saveConfig = func(*config.Config) error { return errors.New("disk full") }

	err := setupAFM("/usr/local/bin/afm-bridge", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "saving config: disk full") {
		t.Errorf("error = %q, want substring %q", err.Error(), "saving config: disk full")
	}
}

// TestAFMAvailable tests the bridge invocation logic with a fake bridge script.
func TestAFMAvailable(t *testing.T) {
	t.Run("bridge returns available=true", func(t *testing.T) {
		// Create a fake bridge script that prints available JSON.
		script := `#!/bin/sh
echo '{"available":true}'`
		path := writeTempScript(t, script)

		ok, reason := afmAvailable(path)
		if !ok {
			t.Errorf("afmAvailable() = false, want true (reason: %s)", reason)
		}
		if reason != "" {
			t.Errorf("reason = %q, want empty", reason)
		}
	})

	t.Run("bridge returns available=false with reason", func(t *testing.T) {
		script := `#!/bin/sh
echo '{"available":false,"reason":"device_not_eligible"}'`
		path := writeTempScript(t, script)

		ok, reason := afmAvailable(path)
		if ok {
			t.Error("afmAvailable() = true, want false")
		}
		if reason != "device_not_eligible" {
			t.Errorf("reason = %q, want %q", reason, "device_not_eligible")
		}
	})

	t.Run("bridge exits with non-zero → unavailable", func(t *testing.T) {
		script := `#!/bin/sh
exit 1`
		path := writeTempScript(t, script)

		ok, reason := afmAvailable(path)
		if ok {
			t.Error("afmAvailable() = true for failing bridge, want false")
		}
		if reason != "bridge execution failed" {
			t.Errorf("reason = %q, want %q", reason, "bridge execution failed")
		}
	})

	t.Run("bridge exits non-zero with stderr → reason includes stderr", func(t *testing.T) {
		script := `#!/bin/sh
echo "framework not found" >&2
exit 1`
		path := writeTempScript(t, script)

		ok, reason := afmAvailable(path)
		if ok {
			t.Error("afmAvailable() = true for failing bridge, want false")
		}
		if !strings.Contains(reason, "framework not found") {
			t.Errorf("reason = %q, want substring %q", reason, "framework not found")
		}
	})

	t.Run("bridge prints invalid JSON → unavailable", func(t *testing.T) {
		script := `#!/bin/sh
echo 'not json'`
		path := writeTempScript(t, script)

		ok, reason := afmAvailable(path)
		if ok {
			t.Error("afmAvailable() = true for invalid JSON, want false")
		}
		if reason != "could not parse bridge response" {
			t.Errorf("reason = %q, want %q", reason, "could not parse bridge response")
		}
	})

	t.Run("bridge prints invalid JSON with stderr → reason includes stderr", func(t *testing.T) {
		script := `#!/bin/sh
echo "warning: deprecated API" >&2
echo 'not json'`
		path := writeTempScript(t, script)

		ok, reason := afmAvailable(path)
		if ok {
			t.Error("afmAvailable() = true for invalid JSON, want false")
		}
		if !strings.Contains(reason, "warning: deprecated API") {
			t.Errorf("reason = %q, want substring %q", reason, "warning: deprecated API")
		}
	})
}

// writeTempScript writes a shell script to a temp file and makes it executable.
func writeTempScript(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fake-bridge-*.sh")
	if err != nil {
		t.Fatalf("creating temp script: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp script: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing temp script: %v", err)
	}
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		t.Fatalf("chmod temp script: %v", err)
	}
	return f.Name()
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
		installOK bool
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
		{
			name:      "not installed, darwin, install succeeds",
			found:     false,
			os:        "darwin",
			input:     "y\n",
			installOK: true,
			wantInOut: "[ok] Ollama installed",
		},
		{
			name:      "not installed, darwin, install fails",
			found:     false,
			os:        "darwin",
			input:     "y\n",
			installOK: false,
			wantErr:   "failed to install ollama",
		},
		{
			name:      "not installed, linux, install succeeds",
			found:     false,
			os:        "linux",
			input:     "y\n",
			installOK: true,
			wantInOut: "[ok] Ollama installed",
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
			if !tt.found && (tt.os == "darwin" || tt.os == "linux") {
				execCommand = func(_ string, _ ...string) *exec.Cmd {
					if tt.installOK {
						return exec.Command("sh", "-c", "exit 0")
					}
					return exec.Command("sh", "-c", "exit 1")
				}
			}

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
		restore := saveFuncVars(t)
		defer restore()

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
		restore := saveFuncVars(t)
		defer restore()

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

	t.Run("start command fails", func(t *testing.T) {
		restore := saveFuncVars(t)
		defer restore()

		reachabilityCheck = func(string) bool { return false }
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("definitely-not-a-command")
		}

		out := &bytes.Buffer{}
		err := ensureOllamaRunning("http://localhost:11434", strings.NewReader("y\n"), out)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to start ollama") {
			t.Errorf("error = %q, want substring %q", err.Error(), "failed to start ollama")
		}
	})

	t.Run("starts and becomes reachable", func(t *testing.T) {
		restore := saveFuncVars(t)
		defer restore()

		calls := 0
		reachabilityCheck = func(string) bool {
			calls++
			return calls >= 3
		}
		sleep = func(time.Duration) {}
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "exit 0")
		}

		out := &bytes.Buffer{}
		err := ensureOllamaRunning("http://localhost:11434", strings.NewReader("y\n"), out)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "[ok] Ollama is running") {
			t.Errorf("output = %q, want substring %q", out.String(), "[ok] Ollama is running")
		}
	})

	t.Run("times out if still unreachable", func(t *testing.T) {
		restore := saveFuncVars(t)
		defer restore()

		reachabilityCheck = func(string) bool { return false }
		sleep = func(time.Duration) {}
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "exit 0")
		}

		out := &bytes.Buffer{}
		err := ensureOllamaRunning("http://localhost:11434", strings.NewReader("y\n"), out)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "did not start within 10 seconds") {
			t.Errorf("error = %q, want timeout message", err.Error())
		}
	})
}
