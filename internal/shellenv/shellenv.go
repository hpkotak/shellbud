// Package shellenv gathers the user's current shell environment for LLM context.
// All gathering is best-effort: individual failures produce empty fields, never errors.
package shellenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/hpkotak/shellbud/internal/platform"
)

const (
	cmdTimeout     = 2 * time.Second
	maxDirLines    = 50
	maxGitLogLines = 5
)

// Snapshot holds the user's current environment state.
type Snapshot struct {
	CWD       string
	DirList   string            // first maxDirLines lines of ls -la
	OS        string            // runtime.GOOS
	Shell     string            // $SHELL
	Arch      string            // runtime.GOARCH
	GitBranch string            // empty if not a git repo
	GitDirty  bool              // has uncommitted changes
	GitRecent string            // last N commit onelines
	Env       map[string]string // filtered env vars
}

// execCommandFn is injectable for testing. Default calls exec.Command().Output().
var execCommandFn = defaultExecCommand

func defaultExecCommand(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}

// envVarAllowlist is the set of environment variables worth including in context.
var envVarAllowlist = []string{"EDITOR", "VISUAL", "LANG", "TERM", "HOME", "USER"}

// Gather collects the current environment snapshot.
// Individual failures are swallowed — the snapshot is best-effort.
func Gather() Snapshot {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	s := Snapshot{
		OS:    runtime.GOOS,
		Shell: platform.Shell(),
		Arch:  runtime.GOARCH,
		Env:   gatherEnv(),
	}

	s.CWD, _ = os.Getwd()
	s.DirList = gatherDirList(ctx)
	s.GitBranch = gatherGitBranch(ctx)
	s.GitDirty = gatherGitDirty(ctx)
	s.GitRecent = gatherGitRecent(ctx)

	return s
}

// Format renders the snapshot as a string for embedding in the system prompt.
func (s Snapshot) Format() string {
	var b strings.Builder

	fmt.Fprintf(&b, "OS: %s (%s)\n", s.OS, s.Arch)
	fmt.Fprintf(&b, "Shell: %s\n", s.Shell)

	if s.CWD != "" {
		fmt.Fprintf(&b, "Working directory: %s\n", s.CWD)
	}

	if s.GitBranch != "" {
		status := "clean"
		if s.GitDirty {
			status = "dirty"
		}
		fmt.Fprintf(&b, "Git branch: %s (%s)\n", s.GitBranch, status)
	}

	if s.GitRecent != "" {
		fmt.Fprintf(&b, "Recent commits:\n%s\n", s.GitRecent)
	}

	if s.DirList != "" {
		fmt.Fprintf(&b, "Directory contents:\n%s\n", s.DirList)
	}

	if len(s.Env) > 0 {
		fmt.Fprintf(&b, "Environment:\n")
		for _, key := range envVarAllowlist {
			if val, ok := s.Env[key]; ok {
				fmt.Fprintf(&b, "  %s=%s\n", key, val)
			}
		}
	}

	return b.String()
}

func gatherEnv() map[string]string {
	env := make(map[string]string)
	for _, key := range envVarAllowlist {
		if val := os.Getenv(key); val != "" {
			env[key] = val
		}
	}
	return env
}

func gatherDirList(ctx context.Context) string {
	out, err := execCommandFn(ctx, "ls", "-la")
	if err != nil {
		return ""
	}
	return truncateLines(out, maxDirLines)
}

func gatherGitBranch(ctx context.Context) string {
	out, err := execCommandFn(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func gatherGitDirty(ctx context.Context) bool {
	out, err := execCommandFn(ctx, "git", "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

func gatherGitRecent(ctx context.Context) string {
	out, err := execCommandFn(ctx, "git", "log", "--oneline", fmt.Sprintf("-%d", maxGitLogLines))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func truncateLines(s string, max int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= max {
		return strings.TrimSpace(s)
	}
	truncated := strings.Join(lines[:max], "\n")
	return truncated + fmt.Sprintf("\n[... %d more entries]", len(lines)-max)
}
