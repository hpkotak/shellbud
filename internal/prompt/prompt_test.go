package prompt

import (
	"strings"
	"testing"
)

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "clean command",
			raw:  "tar -czvf archive.tar.gz ./folder",
			want: "tar -czvf archive.tar.gz ./folder",
		},
		{
			name: "with leading/trailing whitespace",
			raw:  "  tar -czvf archive.tar.gz ./folder  \n",
			want: "tar -czvf archive.tar.gz ./folder",
		},
		{
			name: "with code fence bash",
			raw:  "```bash\ntar -czvf archive.tar.gz ./folder\n```",
			want: "tar -czvf archive.tar.gz ./folder",
		},
		{
			name: "with code fence sh",
			raw:  "```sh\ntar -czvf archive.tar.gz ./folder\n```",
			want: "tar -czvf archive.tar.gz ./folder",
		},
		{
			name: "with plain code fence",
			raw:  "```\nls -la\n```",
			want: "ls -la",
		},
		{
			name: "with inline backticks",
			raw:  "`ls -la`",
			want: "ls -la",
		},
		{
			name: "with dollar sign prefix",
			raw:  "$ ls -la",
			want: "ls -la",
		},
		{
			name: "with explanation after command",
			raw:  "find . -name '*.log' -size +100M\nThis finds all log files larger than 100MB",
			want: "find . -name '*.log' -size +100M",
		},
		{
			name: "with preamble before code fence",
			raw:  "```\ndu -sh * | sort -rh\n```",
			want: "du -sh * | sort -rh",
		},
		{
			name: "empty string",
			raw:  "",
			want: "",
		},
		{
			name: "only whitespace",
			raw:  "   \n  \n  ",
			want: "",
		},
		{
			name: "dollar sign in code fence",
			raw:  "```bash\n$ grep -r 'TODO' .\n```",
			want: "grep -r 'TODO' .",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseResponse(tt.raw)
			if got != tt.want {
				t.Errorf("ParseResponse(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestBuildUserPrompt(t *testing.T) {
	got := BuildUserPrompt("compress folder", "darwin", "/bin/zsh")
	want := "OS: darwin, Shell: /bin/zsh\nTranslate to shell command: compress folder"
	if got != want {
		t.Errorf("BuildUserPrompt() = %q, want %q", got, want)
	}
}

func TestSystemPrompt(t *testing.T) {
	sp := SystemPrompt()

	if sp == "" {
		t.Fatal("SystemPrompt() returned empty string")
	}

	// Key phrases that the system prompt must contain for correct LLM behavior.
	required := []string{
		"shell command",
		"ONLY",
		"No explanation",
	}
	for _, phrase := range required {
		if !strings.Contains(sp, phrase) {
			t.Errorf("SystemPrompt() missing required phrase %q", phrase)
		}
	}

	// Stability: multiple calls return the same value.
	if sp2 := SystemPrompt(); sp2 != sp {
		t.Error("SystemPrompt() not stable across calls")
	}
}
