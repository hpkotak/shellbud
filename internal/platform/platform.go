// Package platform provides OS and shell detection helpers.
package platform

import (
	"os"
	"runtime"
)

// OS returns the operating system name (e.g., "darwin", "linux").
func OS() string {
	return runtime.GOOS
}

// Shell returns the user's shell from $SHELL, defaulting to /bin/sh.
func Shell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}
