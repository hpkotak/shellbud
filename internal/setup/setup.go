// Package setup handles first-run onboarding: detecting, installing, and
// configuring Ollama. All actions require explicit user consent.
package setup

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hpkotak/shellbud/internal/config"
	"github.com/hpkotak/shellbud/internal/platform"
	"github.com/ollama/ollama/api"
)

var scanner *bufio.Scanner

func init() {
	scanner = bufio.NewScanner(os.Stdin)
}

// Run executes the interactive setup flow.
func Run() error {
	fmt.Println("ShellBud Setup")
	fmt.Println("==============")
	fmt.Printf("Platform: %s\n\n", platform.OS())

	// Step 1: Check if Ollama is installed
	if err := ensureOllamaInstalled(); err != nil {
		return err
	}

	// Step 2: Check if Ollama is running
	host := "http://localhost:11434"
	if err := ensureOllamaRunning(host); err != nil {
		return err
	}

	// Step 3: Get available models, offer to pull if none
	client, err := ollamaClient(host)
	if err != nil {
		return err
	}

	model, err := selectModel(client)
	if err != nil {
		return err
	}

	// Step 4: Save config
	cfg := &config.Config{
		Provider: "ollama",
		Model:    model,
		Ollama:   config.Ollama{Host: host},
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nConfig saved to %s\n", config.Path())
	fmt.Println("Ready! Try: sb compress this folder as tar.gz")
	return nil
}

func ensureOllamaInstalled() error {
	if _, err := exec.LookPath("ollama"); err == nil {
		fmt.Println("[ok] Ollama is installed")
		return nil
	}

	fmt.Println("[!!] Ollama not found")

	switch platform.OS() {
	case "darwin":
		fmt.Print("Install Ollama via Homebrew? [Y/n]: ")
		if !readYesNo(true) {
			return fmt.Errorf("ollama is required. Install it manually from https://ollama.com")
		}
		fmt.Println("Running: brew install ollama")
		cmd := exec.Command("brew", "install", "ollama")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install ollama: %w", err)
		}
	case "linux":
		fmt.Print("Install Ollama via install script? [Y/n]: ")
		if !readYesNo(true) {
			return fmt.Errorf("ollama is required. Install it manually from https://ollama.com")
		}
		fmt.Println("Running: curl -fsSL https://ollama.com/install.sh | sh")
		cmd := exec.Command("sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install ollama: %w", err)
		}
	default:
		return fmt.Errorf("unsupported platform %s. Install Ollama manually from https://ollama.com", platform.OS())
	}

	fmt.Println("[ok] Ollama installed")
	return nil
}

func ensureOllamaRunning(host string) error {
	if isOllamaReachable(host) {
		fmt.Println("[ok] Ollama is running")
		return nil
	}

	fmt.Println("[!!] Ollama is not running")
	fmt.Print("Start Ollama? [Y/n]: ")
	if !readYesNo(true) {
		return fmt.Errorf("ollama must be running. Start it with: ollama serve")
	}

	fmt.Println("Starting Ollama in background...")
	cmd := exec.Command("ollama", "serve")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama: %w", err)
	}

	// Wait for it to come up
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		if isOllamaReachable(host) {
			fmt.Println("[ok] Ollama is running")
			return nil
		}
		fmt.Print(".")
	}

	return fmt.Errorf("ollama did not start within 10 seconds")
}

func selectModel(client *api.Client) (string, error) {
	ctx := context.Background()
	models, err := client.List(ctx)
	if err != nil {
		return "", fmt.Errorf("listing models: %w", err)
	}

	if len(models.Models) == 0 {
		return pullRecommendedModel(client)
	}

	fmt.Println("\nAvailable models:")
	for i, m := range models.Models {
		fmt.Printf("  %d. %s\n", i+1, m.Name)
	}
	fmt.Printf("\nSelect default model [1]: ")

	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	idx := 0 // default to first
	if input != "" {
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > len(models.Models) {
			return "", fmt.Errorf("invalid selection: %s", input)
		}
		idx = n - 1
	}

	selected := models.Models[idx].Name
	fmt.Printf("[ok] Selected: %s\n", selected)
	return selected, nil
}

func pullRecommendedModel(client *api.Client) (string, error) {
	fmt.Println("\nNo models found. Pull a recommended model?")
	fmt.Println("  1. llama3.2:3b   (fast, ~2GB)")
	fmt.Println("  2. codellama:7b  (better for code, ~4GB)")
	fmt.Println("  3. Skip")
	fmt.Print("\nSelect [1]: ")

	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

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

	fmt.Printf("Pulling %s (this may take a few minutes)...\n", model)
	ctx := context.Background()
	err := client.Pull(ctx, &api.PullRequest{Model: model}, func(resp api.ProgressResponse) error {
		if resp.Total > 0 {
			pct := float64(resp.Completed) / float64(resp.Total) * 100
			fmt.Printf("\r  %.0f%% downloaded", pct)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("pulling model: %w", err)
	}
	fmt.Printf("\n[ok] %s ready\n", model)
	return model, nil
}

func ollamaClient(host string) (*api.Client, error) {
	base, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parsing host URL: %w", err)
	}
	return api.NewClient(base, http.DefaultClient), nil
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

func readYesNo(defaultYes bool) bool {
	scanner.Scan()
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	switch input {
	case "":
		return defaultYes
	case "y", "yes":
		return true
	default:
		return false
	}
}
