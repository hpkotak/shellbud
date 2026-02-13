// Package executor handles user confirmation and shell command execution.
// Confirm uses injectable io.Reader/io.Writer for testability.
package executor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/hpkotak/shellbud/internal/platform"
)

// Confirm prompts the user for yes/no confirmation.
// defaultYes controls what happens when the user presses Enter without input.
// in and out are injectable for testing.
func Confirm(prompt string, defaultYes bool, in io.Reader, out io.Writer) bool {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	_, _ = fmt.Fprintf(out, "%s %s: ", prompt, hint)

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return false
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))

	switch input {
	case "":
		return defaultYes
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return false
	}
}

// Run executes a shell command, inheriting stdin/stdout/stderr.
func Run(command string) error {
	shell := platform.Shell()
	cmd := exec.Command(shell, "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
