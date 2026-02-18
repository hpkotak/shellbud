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

// rule pairs a destructive pattern with an optional exclusion pattern.
// A command that matches both pattern and exclude is not flagged by this rule.
type rule struct {
	pattern *regexp.Regexp
	exclude *regexp.Regexp // nil means no exclusion
}

var (
	rules     []rule
	rulesOnce sync.Once
)

// rawRule holds the uncompiled pattern/exclusion strings.
type rawRule struct {
	pattern string
	exclude string // empty means no exclusion
}

// destructiveRules defines patterns for destructive commands.
// exclude is checked after pattern matches — if exclude also matches, this rule is skipped.
var destructiveRules = []rawRule{
	{`\brm\s`, ""},
	{`\brm$`, ""},
	{`\bsudo\s`, ""},
	{`\bdd\s+if=`, ""},
	{`\bmkfs\b`, ""},
	{`\bfdisk\b`, ""},
	// Redirections to /dev/ are destructive, but /dev/null, /dev/stdout, /dev/stderr are safe.
	{`>+\s*/dev/`, `>+\s*/dev/(null|stdout|stderr)(\s|;|&|$)`},
	{`\bchmod\s+000\b`, ""},
	{`\bkill\s+-9\b`, ""},
	{`\bkillall\s`, ""},
	{`\bshutdown\b`, ""},
	{`\breboot\b`, ""},
	{`\bsystemctl\s+(stop|disable|mask)\b`, ""},
	{`\bmv\s+/`, ""},
	{`\bchown\s+-R\b`, ""},
	{`:\s*>\s*\S`, ""}, // truncate via : > file
	{`\btruncate\b`, ""},
	{`\bshred\b`, ""},
}

func compileRules() {
	rulesOnce.Do(func() {
		rules = make([]rule, len(destructiveRules))
		for i, r := range destructiveRules {
			rules[i].pattern = regexp.MustCompile(r.pattern)
			if r.exclude != "" {
				rules[i].exclude = regexp.MustCompile(r.exclude)
			}
		}
	})
}

// Classify examines a shell command and returns its safety level.
func Classify(command string) Level {
	compileRules()
	for _, r := range rules {
		if r.pattern.MatchString(command) {
			if r.exclude != nil && r.exclude.MatchString(command) {
				continue
			}
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
