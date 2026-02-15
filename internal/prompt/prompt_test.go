package prompt

import (
	"strings"
	"testing"
)

func TestChatSystemPrompt(t *testing.T) {
	envContext := "OS: darwin (arm64)\nShell: /bin/zsh\nWorking directory: /tmp/test\n"
	got := ChatSystemPrompt(envContext)

	required := []string{
		"ShellBud",
		"shell assistant",
		"code block",
		"darwin (arm64)",
		"/bin/zsh",
		"/tmp/test",
	}
	for _, phrase := range required {
		if !strings.Contains(got, phrase) {
			t.Errorf("ChatSystemPrompt() missing %q", phrase)
		}
	}
}

func TestParseChatResponse(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantText     string
		wantCommands []string
	}{
		{
			name:         "text only, no commands",
			raw:          "The largest file in the directory is main.go at 4.2KB.",
			wantText:     "The largest file in the directory is main.go at 4.2KB.",
			wantCommands: nil,
		},
		{
			name:         "single command in code block",
			raw:          "Here's the command:\n```bash\nls -la\n```",
			wantText:     "Here's the command:\n```bash\nls -la\n```",
			wantCommands: []string{"ls -la"},
		},
		{
			name:         "plain code fence without language",
			raw:          "Try this:\n```\nfind . -name '*.go'\n```",
			wantText:     "Try this:\n```\nfind . -name '*.go'\n```",
			wantCommands: []string{"find . -name '*.go'"},
		},
		{
			name:         "sh language tag",
			raw:          "```sh\ngrep -r TODO .\n```",
			wantText:     "```sh\ngrep -r TODO .\n```",
			wantCommands: []string{"grep -r TODO ."},
		},
		{
			name: "multiple code blocks",
			raw:  "First, check the branch:\n```bash\ngit branch\n```\nThen check status:\n```bash\ngit status\n```",
			wantText:     "First, check the branch:\n```bash\ngit branch\n```\nThen check status:\n```bash\ngit status\n```",
			wantCommands: []string{"git branch", "git status"},
		},
		{
			name:         "empty code block ignored",
			raw:          "```bash\n\n```",
			wantText:     "```bash\n\n```",
			wantCommands: nil,
		},
		{
			name:         "empty string",
			raw:          "",
			wantText:     "",
			wantCommands: nil,
		},
		{
			name:         "whitespace only",
			raw:          "   \n  ",
			wantText:     "",
			wantCommands: nil,
		},
		{
			name:         "multi-line command in block",
			raw:          "```bash\nfind . -name '*.log' \\\n  -mtime +30 \\\n  -delete\n```",
			wantText:     "```bash\nfind . -name '*.log' \\\n  -mtime +30 \\\n  -delete\n```",
			wantCommands: []string{"find . -name '*.log' \\\n  -mtime +30 \\\n  -delete"},
		},
		{
			name:         "explanation with command in middle",
			raw:          "To find large files, use:\n```bash\nfind . -size +100M\n```\nThis will search from the current directory.",
			wantText:     "To find large files, use:\n```bash\nfind . -size +100M\n```\nThis will search from the current directory.",
			wantCommands: []string{"find . -size +100M"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseChatResponse(tt.raw)

			if got.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", got.Text, tt.wantText)
			}

			if len(got.Commands) != len(tt.wantCommands) {
				t.Fatalf("Commands count = %d, want %d\ngot: %v", len(got.Commands), len(tt.wantCommands), got.Commands)
			}

			for i, want := range tt.wantCommands {
				if got.Commands[i] != want {
					t.Errorf("Commands[%d] = %q, want %q", i, got.Commands[i], want)
				}
			}
		})
	}
}
