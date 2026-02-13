// Package safety classifies shell commands as safe or destructive using regex
// pattern matching. This is intentionally not LLM-based — safety checks must
// be deterministic, fast, and independent of the model that generated the command.
package safety

import (
	"regexp"
	"sync"
)

// Level represents the safety classification of a command.
type Level int

const (
	Safe Level = iota
	Destructive
)

var (
	patterns     []*regexp.Regexp
	patternsOnce sync.Once
)

// Raw patterns for destructive commands.
var destructiveRaw = []string{
	`\brm\s`,
	`\brm$`,
	`\bsudo\s`,
	`\bdd\s+if=`,
	`\bmkfs\b`,
	`\bfdisk\b`,
	`\b>\s*/dev/`,
	`\bchmod\s+000\b`,
	`\bkill\s+-9\b`,
	`\bkillall\s`,
	`\bshutdown\b`,
	`\breboot\b`,
	`\bsystemctl\s+(stop|disable|mask)\b`,
	`\bmv\s+/`,
	`\bchown\s+-R\b`,
	`:\s*>\s*\S`,  // truncate via : > file
	`\btruncate\b`,
	`\bshred\b`,
}

func compilePatterns() {
	patternsOnce.Do(func() {
		patterns = make([]*regexp.Regexp, len(destructiveRaw))
		for i, raw := range destructiveRaw {
			patterns[i] = regexp.MustCompile(raw)
		}
	})
}

// Classify examines a shell command and returns its safety level.
func Classify(command string) Level {
	compilePatterns()
	for _, p := range patterns {
		if p.MatchString(command) {
			return Destructive
		}
	}
	return Safe
}

func (l Level) String() string {
	if l == Destructive {
		return "destructive"
	}
	return "safe"
}
