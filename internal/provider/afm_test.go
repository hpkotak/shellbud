package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

func TestNewAFM(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		command string
		wantErr string
	}{
		{name: "valid", model: "afm-latest", command: "/usr/local/bin/afm-bridge"},
		{name: "empty model", model: "", command: "/usr/local/bin/afm-bridge", wantErr: "model cannot be empty"},
		{name: "empty command", model: "afm-latest", command: "", wantErr: "afm command cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewAFM(tt.model, tt.command)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NewAFM() unexpected error: %v", err)
				}
				if p == nil {
					t.Fatal("NewAFM() returned nil provider")
				}
				return
			}
			if err == nil {
				t.Fatalf("NewAFM() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestAFMNameAndCapabilities(t *testing.T) {
	p, _ := NewAFM("afm-latest", "/tmp/afm-bridge")

	if got := p.Name(); got != "afm" {
		t.Errorf("Name() = %q, want %q", got, "afm")
	}

	gotCaps := p.Capabilities()
	wantCaps := Capabilities{
		JSONMode:     true,
		Usage:        false,
		FinishReason: false,
	}
	if gotCaps != wantCaps {
		t.Errorf("Capabilities() = %+v, want %+v", gotCaps, wantCaps)
	}
}

func TestAFMAvailable(t *testing.T) {
	t.Run("absolute path exists", func(t *testing.T) {
		script := filepath.Join(t.TempDir(), "afm-bridge")
		writeExecutable(t, script, "#!/bin/sh\necho ok\n")

		p, _ := NewAFM("afm-latest", script)
		if err := p.Available(context.Background()); err != nil {
			t.Fatalf("Available() unexpected error: %v", err)
		}
	})

	t.Run("absolute path missing", func(t *testing.T) {
		p, _ := NewAFM("afm-latest", "/path/does/not/exist/afm-bridge")
		err := p.Available(context.Background())
		if err == nil {
			t.Fatal("Available() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q, want substring %q", err.Error(), "not found")
		}
	})

	t.Run("absolute path not executable", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "afm-bridge")
		if err := os.WriteFile(path, []byte("#!/bin/sh\necho ok\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		p, _ := NewAFM("afm-latest", path)
		err := p.Available(context.Background())
		if err == nil {
			t.Fatal("Available() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not executable") {
			t.Errorf("error = %q, want substring %q", err.Error(), "not executable")
		}
	})

	t.Run("path lookup works", func(t *testing.T) {
		tmpDir := t.TempDir()
		script := filepath.Join(tmpDir, "afm-bridge")
		writeExecutable(t, script, "#!/bin/sh\necho ok\n")

		t.Setenv("PATH", tmpDir+":"+os.Getenv("PATH"))
		p, _ := NewAFM("afm-latest", "afm-bridge")
		if err := p.Available(context.Background()); err != nil {
			t.Fatalf("Available() unexpected error: %v", err)
		}
	})
}

func TestAFMChat(t *testing.T) {
	tmpDir := t.TempDir()
	reqPath := filepath.Join(tmpDir, "request.json")
	script := filepath.Join(tmpDir, "afm-bridge")

	body := `#!/bin/sh
cat > "$AFM_REQ_FILE"
echo '{"content":"{\"text\":\"ok\",\"commands\":[]}","finish_reason":"stop","usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}}'
`
	writeExecutable(t, script, body)
	t.Setenv("AFM_REQ_FILE", reqPath)

	p, _ := NewAFM("default-model", script)
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
	wantUsage := Usage{InputTokens: 7, OutputTokens: 3, TotalTokens: 10}
	if got.Usage != wantUsage {
		t.Errorf("Usage = %+v, want %+v", got.Usage, wantUsage)
	}

	reqData, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	reqText := string(reqData)
	if !strings.Contains(reqText, `"model":"override-model"`) {
		t.Errorf("request missing overridden model: %s", reqText)
	}
	if !strings.Contains(reqText, `"content":"hello"`) {
		t.Errorf("request missing message content: %s", reqText)
	}
	if !strings.Contains(reqText, `"expect_json":true`) {
		t.Errorf("request missing expect_json flag: %s", reqText)
	}
}

func TestAFMChatErrors(t *testing.T) {
	tests := []struct {
		name       string
		scriptBody string
		wantErr    string
	}{
		{
			name: "bridge execution fails",
			scriptBody: `#!/bin/sh
echo "boom" 1>&2
exit 1
`,
			wantErr: "boom",
		},
		{
			name: "invalid response json",
			scriptBody: `#!/bin/sh
cat >/dev/null
echo "{not-json"
`,
			wantErr: "decoding afm response",
		},
		{
			name: "empty content",
			scriptBody: `#!/bin/sh
cat >/dev/null
echo '{"content":"   "}'
`,
			wantErr: "empty response from model",
		},
		{
			name: "output too large",
			scriptBody: `#!/bin/sh
cat >/dev/null
yes a | head -c 2000000
`,
			wantErr: "output exceeded limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := filepath.Join(t.TempDir(), "afm-bridge")
			writeExecutable(t, script, tt.scriptBody)

			p, _ := NewAFM("afm-latest", script)
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
