package cmd

import (
	"strings"
	"testing"

	"github.com/hpkotak/shellbud/internal/config"
)

func TestRunConfigShow(t *testing.T) {
	t.Run("config exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		if err := config.Save(config.Default()); err != nil {
			t.Fatalf("save config: %v", err)
		}

		err := runConfigShow(nil, nil)
		// runConfigShow prints to fmt.Printf (not ioOut), so we can only check error
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("config missing", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		err := runConfigShow(nil, nil)
		if err == nil {
			t.Fatal("expected error for missing config, got nil")
		}
		if !strings.Contains(err.Error(), "sb setup") {
			t.Errorf("error = %q, want substring %q", err.Error(), "sb setup")
		}
	})
}
