package platform

import (
	"runtime"
	"testing"
)

func TestOS(t *testing.T) {
	got := OS()
	if got == "" {
		t.Fatal("OS() returned empty string")
	}
	if got != runtime.GOOS {
		t.Errorf("OS() = %q, want %q", got, runtime.GOOS)
	}
}

func TestShell(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		want  string
		unset bool // if true, unset SHELL instead of setting it
	}{
		{"zsh", "/bin/zsh", "/bin/zsh", false},
		{"bash", "/bin/bash", "/bin/bash", false},
		{"empty falls back", "", "/bin/sh", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.env)
			got := Shell()
			if got != tt.want {
				t.Errorf("Shell() = %q, want %q", got, tt.want)
			}
		})
	}
}
