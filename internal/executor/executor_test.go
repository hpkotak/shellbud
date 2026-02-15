package executor

import (
	"bytes"
	"fmt"
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

func TestRunCapture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("RunCapture() uses shell -c, not applicable on Windows")
	}

	tests := []struct {
		name         string
		command      string
		wantOutput   string
		wantExitCode int
		wantErr      bool
	}{
		{
			name:         "echo captures output",
			command:      "echo hello",
			wantOutput:   "hello\n",
			wantExitCode: 0,
		},
		{
			name:         "nonzero exit code",
			command:      "exit 42",
			wantExitCode: 42,
		},
		{
			name:         "stderr also captured",
			command:      "echo oops >&2",
			wantOutput:   "oops\n",
			wantExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, exitCode, err := RunCapture(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunCapture(%q) error = %v, wantErr %v", tt.command, err, tt.wantErr)
			}
			if exitCode != tt.wantExitCode {
				t.Errorf("RunCapture(%q) exitCode = %d, want %d", tt.command, exitCode, tt.wantExitCode)
			}
			if tt.wantOutput != "" && output != tt.wantOutput {
				t.Errorf("RunCapture(%q) output = %q, want %q", tt.command, output, tt.wantOutput)
			}
		})
	}
}

func TestRunCaptureTruncation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("RunCapture() uses shell -c, not applicable on Windows")
	}

	// Generate output larger than MaxOutputBytes
	command := fmt.Sprintf("head -c %d /dev/zero | tr '\\0' 'A'", MaxOutputBytes+1000)
	output, exitCode, err := RunCapture(command)
	if err != nil {
		t.Fatalf("RunCapture() error = %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if !strings.HasSuffix(output, "[output truncated]") {
		t.Errorf("output should end with truncation marker, got last 50 chars: %q", output[len(output)-50:])
	}
	if len(output) > MaxOutputBytes+50 { // some slack for the truncation message
		t.Errorf("output length = %d, should be near MaxOutputBytes (%d)", len(output), MaxOutputBytes)
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
