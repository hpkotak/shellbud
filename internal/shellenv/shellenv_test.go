package shellenv

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockExec returns a function that maps command names to canned outputs.
// Commands not in the map return an error.
func mockExec(responses map[string]string) func(context.Context, string, ...string) (string, error) {
	return func(_ context.Context, name string, args ...string) (string, error) {
		// Use "name args[0]" as key for specificity (e.g., "git rev-parse")
		key := name
		if len(args) > 0 {
			key = name + " " + args[0]
		}
		if out, ok := responses[key]; ok {
			return out, nil
		}
		return "", fmt.Errorf("command not found: %s", key)
	}
}

func TestGather(t *testing.T) {
	origExec := execCommandFn
	defer func() { execCommandFn = origExec }()

	execCommandFn = mockExec(map[string]string{
		"ls -la":        "total 8\ndrwxr-xr-x  3 user staff  96 Jan  1 00:00 .\n-rw-r--r--  1 user staff 100 Jan  1 00:00 main.go\n",
		"git rev-parse": "main\n",
		"git status":    " M main.go\n",
		"git log":       "abc1234 feat: initial commit\ndef5678 fix: something\n",
	})

	snap := Gather()

	if snap.OS == "" {
		t.Error("OS should not be empty")
	}
	if snap.Shell == "" {
		t.Error("Shell should not be empty")
	}
	if snap.Arch == "" {
		t.Error("Arch should not be empty")
	}
	if snap.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", snap.GitBranch, "main")
	}
	if !snap.GitDirty {
		t.Error("GitDirty should be true when there are modified files")
	}
	if !strings.Contains(snap.GitRecent, "abc1234") {
		t.Errorf("GitRecent should contain commit hash, got %q", snap.GitRecent)
	}
	if !strings.Contains(snap.DirList, "main.go") {
		t.Errorf("DirList should contain main.go, got %q", snap.DirList)
	}
}

func TestGatherNoGit(t *testing.T) {
	origExec := execCommandFn
	defer func() { execCommandFn = origExec }()

	// Only ls succeeds — git commands all fail (not a git repo)
	execCommandFn = mockExec(map[string]string{
		"ls -la": "total 0\n-rw-r--r-- 1 user staff 0 Jan  1 00:00 file.txt\n",
	})

	snap := Gather()

	if snap.GitBranch != "" {
		t.Errorf("GitBranch should be empty in non-git dir, got %q", snap.GitBranch)
	}
	if snap.GitDirty {
		t.Error("GitDirty should be false in non-git dir")
	}
	if snap.GitRecent != "" {
		t.Errorf("GitRecent should be empty in non-git dir, got %q", snap.GitRecent)
	}
	if !strings.Contains(snap.DirList, "file.txt") {
		t.Errorf("DirList should still work, got %q", snap.DirList)
	}
}

func TestGatherCleanRepo(t *testing.T) {
	origExec := execCommandFn
	defer func() { execCommandFn = origExec }()

	execCommandFn = mockExec(map[string]string{
		"ls -la":        "total 0\n",
		"git rev-parse": "develop\n",
		"git status":    "\n", // clean: no output
		"git log":       "abc1234 commit one\n",
	})

	snap := Gather()

	if snap.GitBranch != "develop" {
		t.Errorf("GitBranch = %q, want %q", snap.GitBranch, "develop")
	}
	if snap.GitDirty {
		t.Error("GitDirty should be false for clean repo")
	}
}

func TestFormat(t *testing.T) {
	snap := Snapshot{
		CWD:       "/home/user/project",
		DirList:   "main.go\ngo.mod",
		OS:        "darwin",
		Shell:     "/bin/zsh",
		Arch:      "arm64",
		GitBranch: "feature-x",
		GitDirty:  true,
		GitRecent: "abc1234 initial commit",
		Env:       map[string]string{"EDITOR": "vim", "LANG": "en_US.UTF-8"},
	}

	formatted := snap.Format()

	checks := []string{
		"OS: darwin (arm64)",
		"Shell: /bin/zsh",
		"Working directory: /home/user/project",
		"Git branch: feature-x (dirty)",
		"abc1234 initial commit",
		"main.go",
		"EDITOR=vim",
		"LANG=en_US.UTF-8",
	}

	for _, want := range checks {
		if !strings.Contains(formatted, want) {
			t.Errorf("Format() missing %q\ngot:\n%s", want, formatted)
		}
	}
}

func TestFormatMinimal(t *testing.T) {
	snap := Snapshot{
		OS:    "linux",
		Shell: "/bin/bash",
		Arch:  "amd64",
		Env:   map[string]string{},
	}

	formatted := snap.Format()

	if !strings.Contains(formatted, "OS: linux (amd64)") {
		t.Errorf("Format() missing OS line, got:\n%s", formatted)
	}
	// Should not contain git or directory sections
	if strings.Contains(formatted, "Git branch") {
		t.Error("Format() should not have Git section when no git info")
	}
	if strings.Contains(formatted, "Directory contents") {
		t.Error("Format() should not have dir section when no dir list")
	}
}

func TestTruncateLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{
			name:  "under limit",
			input: "line1\nline2\nline3",
			max:   5,
			want:  "line1\nline2\nline3",
		},
		{
			name:  "at limit",
			input: "line1\nline2\nline3",
			max:   3,
			want:  "line1\nline2\nline3",
		},
		{
			name:  "over limit",
			input: "line1\nline2\nline3\nline4\nline5",
			max:   2,
			want:  "line1\nline2\n[... 3 more entries]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateLines(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncateLines() = %q, want %q", got, tt.want)
			}
		})
	}
}
