// Package setup handles first-run onboarding: detecting available LLM providers
// (AFM on macOS, Ollama) and configuring the user's choice. All actions require
// explicit user consent.
package setup

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/hpkotak/shellbud/internal/executor"
	"github.com/hpkotak/shellbud/internal/platform"
	"github.com/ollama/ollama/api"
)

// Package-level function variables for testability.
// Tests override these to avoid real exec/network calls.
var (
	lookPath             = exec.LookPath
	osStat               = os.Stat
	execCommand          = exec.Command
	platformOS           = platform.OS
	ensureInstalled      = ensureOllamaInstalled
	ensureRunning        = ensureOllamaRunning
	newOllamaClient      = ollamaClient
	chooseModel          = selectModel
	saveConfig           = config.Save
	configPath           = config.Path
	reachabilityCheck    = isOllamaReachable
	sleep                = time.Sleep
	checkAFMAvailability = afmAvailable
)

// Run executes the interactive setup flow.
// in and out are injectable for testability.
func Run(in io.Reader, out io.Writer) error {
	_, _ = fmt.Fprintln(out, "ShellBud Setup")
	_, _ = fmt.Fprintln(out, "==============")
	_, _ = fmt.Fprintf(out, "Platform: %s\n\n", platformOS())

	// On macOS, offer AFM (Apple Foundation Models) as an alternative to Ollama.
	if platformOS() == "darwin" {
		provider, bridgePath, err := offerProviderChoice(in, out)
		if err != nil {
			return err
		}
		if provider == "afm" {
			return setupAFM(bridgePath, out)
		}
		// provider == "ollama": fall through to the Ollama flow below.
	}

	if err := ensureInstalled(in, out); err != nil {
		return err
	}

	host := config.DefaultOllamaHost
	if err := ensureRunning(host, in, out); err != nil {
		return err
	}

	client, err := newOllamaClient(host)
	if err != nil {
		return err
	}

	model, err := chooseModel(client, in, out)
	if err != nil {
		return err
	}

	cfg := &config.Config{
		Provider: config.DefaultProvider,
		Model:    model,
		Ollama:   config.Ollama{Host: host},
		OpenAI:   config.OpenAI{Host: config.DefaultOpenAIHost},
		AFM:      config.AFM{Command: config.DefaultAFMCommand},
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	_, _ = fmt.Fprintf(out, "\nConfig saved to %s\n", configPath())
	_, _ = fmt.Fprintln(out, "Ready! Try: sb compress this folder as tar.gz")
	_, _ = fmt.Fprintln(out, "Tip: switch providers later with: sb config set provider <name>")
	return nil
}

// offerProviderChoice checks for afm-bridge and prompts the user to choose
// between AFM and Ollama. Returns (provider, bridgePath, error).
// bridgePath is the resolved executable path when provider=="afm", empty otherwise.
// If afm-bridge is not installed or AFM is unavailable, returns ("ollama", "", nil).
func offerProviderChoice(in io.Reader, out io.Writer) (string, string, error) {
	bridgePath, err := lookPath("afm-bridge")
	if err != nil {
		// Bridge not in PATH — check the default install location.
		home, homeErr := os.UserHomeDir()
		if homeErr != nil || home == "" {
			return "ollama", "", nil
		}
		candidate := filepath.Join(home, ".shellbud", "bin", "afm-bridge")
		if _, statErr := osStat(candidate); statErr == nil {
			bridgePath = candidate
		} else {
			// No bridge found; proceed with Ollama.
			return "ollama", "", nil
		}
	}

	available, reason := checkAFMAvailability(bridgePath)
	if !available {
		if reason != "" {
			_, _ = fmt.Fprintf(out, "[!!] AFM not available: %s\n", reason)
		}
		return "ollama", "", nil
	}

	_, _ = fmt.Fprintln(out, "[ok] Apple Foundation Models available")
	_, _ = fmt.Fprintln(out, "\nChoose a provider:")
	_, _ = fmt.Fprintln(out, "  1. AFM (Apple Foundation Models — on-device, no setup needed)")
	_, _ = fmt.Fprintln(out, "  2. Ollama (local model server)")

	const maxAttempts = 3
	scanner := bufio.NewScanner(in)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, _ = fmt.Fprint(out, "\nSelect [1]: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		switch input {
		case "", "1":
			return "afm", bridgePath, nil
		case "2":
			return "ollama", "", nil
		default:
			_, _ = fmt.Fprintf(out, "Invalid selection: %s. Please enter 1 or 2.\n", input)
		}
	}
	return "", "", fmt.Errorf("no valid selection after %d attempts", maxAttempts)
}

// setupAFM saves config for the AFM provider and prints a success message.
// bridgePath is the resolved executable path (absolute or in PATH) to save in config.
func setupAFM(bridgePath string, out io.Writer) error {
	if bridgePath == "" {
		bridgePath = config.DefaultAFMCommand
	}
	cfg := &config.Config{
		Provider: "afm",
		Model:    config.DefaultAFMModel,
		Ollama:   config.Ollama{Host: config.DefaultOllamaHost},
		OpenAI:   config.OpenAI{Host: config.DefaultOpenAIHost},
		AFM:      config.AFM{Command: bridgePath},
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	_, _ = fmt.Fprintf(out, "\nConfig saved to %s\n", configPath())
	_, _ = fmt.Fprintln(out, "Ready! Using Apple Foundation Models. Try: sb list files here")
	_, _ = fmt.Fprintln(out, "Tip: switch providers later with: sb config set provider <name>")
	return nil
}

// afmAvailable runs the bridge with --check-availability and returns whether
// AFM is available. Returns (true, "") on success, (false, reason) otherwise.
func afmAvailable(bridgePath string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bridgePath, "--check-availability")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return false, fmt.Sprintf("bridge execution failed: %s", errText)
		}
		return false, "bridge execution failed"
	}

	var result struct {
		Available bool   `json:"available"`
		Reason    string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return false, fmt.Sprintf("could not parse bridge response: %s", errText)
		}
		return false, "could not parse bridge response"
	}
	return result.Available, result.Reason
}

func ensureOllamaInstalled(in io.Reader, out io.Writer) error {
	if _, err := lookPath("ollama"); err == nil {
		_, _ = fmt.Fprintln(out, "[ok] Ollama is installed")
		return nil
	}

	_, _ = fmt.Fprintln(out, "[!!] Ollama not found")

	switch platformOS() {
	case "darwin":
		if !executor.Confirm("Install Ollama via Homebrew?", true, in, out) {
			return fmt.Errorf("ollama is required. Install it manually from https://ollama.com")
		}
		_, _ = fmt.Fprintln(out, "Running: brew install ollama")
		cmd := execCommand("brew", "install", "ollama")
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
		cmd := execCommand("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install ollama: %w", err)
		}
	default:
		return fmt.Errorf("unsupported platform %s. Install Ollama manually from https://ollama.com", platformOS())
	}

	_, _ = fmt.Fprintln(out, "[ok] Ollama installed")
	return nil
}

func ensureOllamaRunning(host string, in io.Reader, out io.Writer) error {
	if reachabilityCheck(host) {
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
	cmd := execCommand("ollama", "serve")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama: %w", err)
	}

	for i := 0; i < 10; i++ {
		sleep(time.Second)
		if reachabilityCheck(host) {
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
