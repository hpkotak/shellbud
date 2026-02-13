package executor

import (
	"bytes"
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
