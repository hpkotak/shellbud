// Package setup handles first-run onboarding: detecting, installing, and
// configuring Ollama. All actions require explicit user consent.
package setup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/hpkotak/shellbud/internal/executor"
	"github.com/hpkotak/shellbud/internal/platform"
	"github.com/ollama/ollama/api"
)

// Run executes the interactive setup flow.
// in and out are injectable for testability.
func Run(in io.Reader, out io.Writer) error {
	_, _ = fmt.Fprintln(out, "ShellBud Setup")
	_, _ = fmt.Fprintln(out, "==============")
	_, _ = fmt.Fprintf(out, "Platform: %s\n\n", platform.OS())

	if err := ensureOllamaInstalled(in, out); err != nil {
		return err
	}

	host := config.DefaultHost
	if err := ensureOllamaRunning(host, in, out); err != nil {
		return err
	}

	client, err := ollamaClient(host)
	if err != nil {
		return err
	}

	model, err := selectModel(client, in, out)
	if err != nil {
		return err
	}

	cfg := &config.Config{
		Provider: config.DefaultProvider,
		Model:    model,
		Ollama:   config.Ollama{Host: host},
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	_, _ = fmt.Fprintf(out, "\nConfig saved to %s\n", config.Path())
	_, _ = fmt.Fprintln(out, "Ready! Try: sb compress this folder as tar.gz")
	return nil
}

func ensureOllamaInstalled(in io.Reader, out io.Writer) error {
	if _, err := exec.LookPath("ollama"); err == nil {
		_, _ = fmt.Fprintln(out, "[ok] Ollama is installed")
		return nil
	}

	_, _ = fmt.Fprintln(out, "[!!] Ollama not found")

	switch platform.OS() {
	case "darwin":
		if !executor.Confirm("Install Ollama via Homebrew?", true, in, out) {
			return fmt.Errorf("ollama is required. Install it manually from https://ollama.com")
		}
		_, _ = fmt.Fprintln(out, "Running: brew install ollama")
		cmd := exec.Command("brew", "install", "ollama")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install ollama: %w", err)
		}
	case "linux":
		if !executor.Confirm("Install Ollama via install script?", true, in, out) {
			return fmt.Errorf("ollama is required. Install it manually from https://ollama.com")
		}
		_, _ = fmt.Fprintln(out, "Running: curl -fsSL https://ollama.com/install.sh | sh")
		cmd := exec.Command("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install ollama: %w", err)
		}
	default:
		return fmt.Errorf("unsupported platform %s. Install Ollama manually from https://ollama.com", platform.OS())
	}

	_, _ = fmt.Fprintln(out, "[ok] Ollama installed")
	return nil
}

func ensureOllamaRunning(host string, in io.Reader, out io.Writer) error {
	if isOllamaReachable(host) {
		_, _ = fmt.Fprintln(out, "[ok] Ollama is running")
		return nil
	}

	_, _ = fmt.Fprintln(out, "[!!] Ollama is not running")
	if !executor.Confirm("Start Ollama?", true, in, out) {
		return fmt.Errorf("ollama must be running. Start it with: ollama serve")
	}

	_, _ = fmt.Fprintln(out, "Starting Ollama in background...")
	// Ollama is a persistent service — we start it but don't own its lifecycle.
	// It continues running after sb exits, which is the expected behavior.
	cmd := exec.Command("ollama", "serve")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama: %w", err)
	}

	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		if isOllamaReachable(host) {
			_, _ = fmt.Fprintln(out, "[ok] Ollama is running")
			return nil
		}
		_, _ = fmt.Fprint(out, ".")
	}

	return fmt.Errorf("ollama did not start within 10 seconds")
}

func selectModel(client *api.Client, in io.Reader, out io.Writer) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := client.List(ctx)
	if err != nil {
		return "", fmt.Errorf("listing models: %w", err)
	}

	if len(models.Models) == 0 {
		return pullRecommendedModel(client, in, out)
	}

	_, _ = fmt.Fprintln(out, "\nAvailable models:")
	for i, m := range models.Models {
		_, _ = fmt.Fprintf(out, "  %d. %s\n", i+1, m.Name)
	}
	_, _ = fmt.Fprint(out, "\nSelect default model [1]: ")

	input := readLine(in)

	idx := 0
	if input != "" {
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > len(models.Models) {
			return "", fmt.Errorf("invalid selection: %s", input)
		}
		idx = n - 1
	}

	selected := models.Models[idx].Name
	_, _ = fmt.Fprintf(out, "[ok] Selected: %s\n", selected)
	return selected, nil
}

func pullRecommendedModel(client *api.Client, in io.Reader, out io.Writer) (string, error) {
	_, _ = fmt.Fprintln(out, "\nNo models found. Pull a recommended model?")
	_, _ = fmt.Fprintln(out, "  1. llama3.2:3b   (fast, ~2GB)")
	_, _ = fmt.Fprintln(out, "  2. codellama:7b  (better for code, ~4GB)")
	_, _ = fmt.Fprintln(out, "  3. Skip")
	_, _ = fmt.Fprint(out, "\nSelect [1]: ")

	input := readLine(in)

	var model string
	switch input {
	case "", "1":
		model = "llama3.2:3b"
	case "2":
		model = "codellama:7b"
	case "3":
		return "", fmt.Errorf("no model selected. Pull a model manually with: ollama pull <model>")
	default:
		return "", fmt.Errorf("invalid selection: %s", input)
	}

	_, _ = fmt.Fprintf(out, "Pulling %s (this may take a few minutes)...\n", model)

	// Model pulls can be large (GBs) — use a generous timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := client.Pull(ctx, &api.PullRequest{Model: model}, func(resp api.ProgressResponse) error {
		if resp.Total > 0 {
			pct := float64(resp.Completed) / float64(resp.Total) * 100
			_, _ = fmt.Fprintf(out, "\r  %.0f%% downloaded", pct)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("pulling model: %w", err)
	}
	_, _ = fmt.Fprintf(out, "\n[ok] %s ready\n", model)
	return model, nil
}

func ollamaClient(host string) (*api.Client, error) {
	base, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parsing host URL: %w", err)
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	return api.NewClient(base, httpClient), nil
}

func isOllamaReachable(host string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(host)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// readLine reads a single line from the reader, trimming whitespace.
func readLine(in io.Reader) string {
	scanner := bufio.NewScanner(in)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}
