package executor

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{"enter with default yes", "\n", true, true},
		{"enter with default no", "\n", false, false},
		{"explicit y", "y\n", false, true},
		{"explicit Y", "Y\n", false, true},
		{"explicit yes", "yes\n", false, true},
		{"explicit n", "n\n", true, false},
		{"explicit no", "no\n", true, false},
		{"explicit N", "N\n", true, false},
		{"garbage input", "asdf\n", true, false},
		{"empty input with spaces", "  \n", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			got := Confirm("Test?", tt.defaultYes, in, out)
			if got != tt.want {
				t.Errorf("Confirm(%q, defaultYes=%v) = %v, want %v",
					tt.input, tt.defaultYes, got, tt.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Run() uses shell -c, not applicable on Windows")
	}

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{"echo succeeds", "echo hello", false},
		{"true succeeds", "true", false},
		{"false fails", "false", true},
		{"nonexistent command", "nonexistent_binary_xyz_12345", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run(%q) error = %v, wantErr %v", tt.command, err, tt.wantErr)
			}
		})
	}
}
