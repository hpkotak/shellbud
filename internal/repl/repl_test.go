package repl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hpkotak/shellbud/internal/provider"
	"github.com/hpkotak/shellbud/internal/shellenv"
)

// mockProvider returns canned responses in order.
type mockProvider struct {
	responses []string
	callCount int
	messages  [][]provider.Message // captured messages per call
}

func (m *mockProvider) Chat(_ context.Context, msgs []provider.Message) (string, error) {
	m.messages = append(m.messages, msgs)
	if m.callCount < len(m.responses) {
		resp := m.responses[m.callCount]
		m.callCount++
		return resp, nil
	}
	return "", fmt.Errorf("no more responses configured")
}

func (m *mockProvider) Name() string                      { return "mock" }
func (m *mockProvider) Available(_ context.Context) error { return nil }

// errProvider always returns an error.
type errProvider struct{}

func (e *errProvider) Chat(_ context.Context, _ []provider.Message) (string, error) {
	return "", fmt.Errorf("model unavailable")
}
func (e *errProvider) Name() string                      { return "err" }
func (e *errProvider) Available(_ context.Context) error { return nil }

type failingReader struct {
	err error
}

func (r failingReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func saveVars(t *testing.T) func() {
	t.Helper()
	origRunCapture := runCapture
	origGatherEnv := gatherEnv
	return func() {
		runCapture = origRunCapture
		gatherEnv = origGatherEnv
	}
}

func stubEnv() {
	gatherEnv = func() shellenv.Snapshot {
		return shellenv.Snapshot{
			OS:    "darwin",
			Shell: "/bin/zsh",
			Arch:  "arm64",
			CWD:   "/tmp/test",
			Env:   map[string]string{},
		}
	}
}

func TestTextOnlyResponse(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	mock := &mockProvider{
		responses: []string{`{"text":"The current directory has 3 files.","commands":[]}`},
	}

	input := "what files are here?\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "The current directory has 3 files.") {
		t.Errorf("output should contain response text, got:\n%s", output)
	}
	if !strings.Contains(output, "Bye!") {
		t.Errorf("output should contain Bye!, got:\n%s", output)
	}
}

func TestCommandRunFlow(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	ranCommand := ""
	runCapture = func(command string) (string, int, error) {
		ranCommand = command
		return "file1.go\nfile2.go\n", 0, nil
	}

	mock := &mockProvider{
		responses: []string{`{"text":"Try this.","commands":["ls -la"]}`},
	}

	input := "list files\nr\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if ranCommand != "ls -la" {
		t.Errorf("expected command 'ls -la', got %q", ranCommand)
	}

	output := out.String()
	if !strings.Contains(output, "[r]un") {
		t.Errorf("output should contain run/explain/skip prompt, got:\n%s", output)
	}
}

func TestCommandSkipFlow(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	ranCommand := false
	runCapture = func(command string) (string, int, error) {
		ranCommand = true
		return "", 0, nil
	}

	mock := &mockProvider{
		responses: []string{`{"text":"Try this.","commands":["ls -la"]}`},
	}

	input := "list files\ns\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if ranCommand {
		t.Error("command should not have been executed when skipped")
	}

	if !strings.Contains(out.String(), "Skipped") {
		t.Errorf("output should contain 'Skipped', got:\n%s", out.String())
	}
}

func TestCommandRunExecutionError(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	runCapture = func(command string) (string, int, error) {
		return "", 1, fmt.Errorf("boom")
	}

	mock := &mockProvider{
		responses: []string{`{"text":"Try this.","commands":["ls -la"]}`},
	}

	input := "list files\nr\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !strings.Contains(out.String(), "Execution error: boom") {
		t.Errorf("output should contain execution error, got:\n%s", out.String())
	}
}

func TestCommandExplainError(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	mock := &mockProvider{
		responses: []string{`{"text":"Try this.","commands":["ls -la"]}`},
	}

	input := "list files\ne\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !strings.Contains(out.String(), "Explain error:") {
		t.Errorf("output should contain explain error, got:\n%s", out.String())
	}
}

func TestCommandInvalidChoiceDefaultsSkip(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	ranCommand := false
	runCapture = func(command string) (string, int, error) {
		ranCommand = true
		return "", 0, nil
	}

	mock := &mockProvider{
		responses: []string{`{"text":"Try this.","commands":["ls -la"]}`},
	}

	input := "list files\nx\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if ranCommand {
		t.Error("command should not run for invalid choice")
	}
	if !strings.Contains(out.String(), "Skipped.") {
		t.Errorf("output should contain skipped message, got:\n%s", out.String())
	}
}

func TestInvalidJSONNoCommandPrompt(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	ranCommand := false
	runCapture = func(command string) (string, int, error) {
		ranCommand = true
		return "", 0, nil
	}

	mock := &mockProvider{
		responses: []string{"ls -la"},
	}

	input := "list files\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if ranCommand {
		t.Error("command should not run for invalid non-JSON response")
	}
	if strings.Contains(out.String(), "[r]un") {
		t.Errorf("output should not show run prompt for invalid response, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "not valid structured output") {
		t.Errorf("output should contain structured-output warning, got:\n%s", out.String())
	}
}

func TestCommandExplainFlow(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	mock := &mockProvider{
		responses: []string{
			`{"text":"Try this.","commands":["find . -size +100M"]}`,
			`{"text":"This command searches for files larger than 100MB.","commands":["echo should-not-run"]}`,
		},
	}

	input := "find big files\ne\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if mock.callCount != 2 {
		t.Errorf("expected 2 LLM calls (query + explain), got %d", mock.callCount)
	}

	output := out.String()
	if !strings.Contains(output, "files larger than 100MB") {
		t.Errorf("output should contain explanation, got:\n%s", output)
	}
	if !strings.Contains(output, `"commands":["echo should-not-run"]`) {
		t.Errorf("explain output should display raw response, got:\n%s", output)
	}
}

func TestExitCommand(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	mock := &mockProvider{}

	input := "exit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !strings.Contains(out.String(), "Bye!") {
		t.Errorf("output should contain 'Bye!', got:\n%s", out.String())
	}
	if mock.callCount != 0 {
		t.Error("no LLM calls should be made when user immediately exits")
	}
}

func TestQuitCommand(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	mock := &mockProvider{}

	input := "quit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !strings.Contains(out.String(), "Bye!") {
		t.Error("'quit' should exit the REPL")
	}
}

func TestEOFExits(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	mock := &mockProvider{}

	// Empty input = immediate EOF
	input := ""
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestInputReadErrorReturns(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	mock := &mockProvider{}
	out := &bytes.Buffer{}
	readErr := errors.New("input stream failed")

	err := Run(mock, failingReader{err: readErr}, out)
	if err == nil {
		t.Fatal("Run() should return read error")
	}
	if !errors.Is(err, readErr) {
		t.Fatalf("Run() error = %v, want %v", err, readErr)
	}
	if !strings.Contains(out.String(), "Input error:") {
		t.Errorf("output should contain input error, got:\n%s", out.String())
	}
	if mock.callCount != 0 {
		t.Error("read error before input should not trigger LLM calls")
	}
}

func TestLLMErrorContinues(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	p := &errProvider{}

	input := "hello\nexit\n"
	out := &bytes.Buffer{}

	err := Run(p, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Error:") {
		t.Errorf("output should show error, got:\n%s", output)
	}
	if !strings.Contains(output, "Bye!") {
		t.Error("REPL should continue after error and still accept exit")
	}
}

func TestDestructiveDoubleConfirm(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	ranCommand := false
	runCapture = func(command string) (string, int, error) {
		ranCommand = true
		return "", 0, nil
	}

	mock := &mockProvider{
		responses: []string{`{"text":"Dangerous command.","commands":["rm -rf /tmp/old"]}`},
	}

	// User chooses run, then declines double confirm
	input := "delete old files\nr\nn\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if ranCommand {
		t.Error("destructive command should not run when double-confirm is declined")
	}

	output := out.String()
	if !strings.Contains(output, "Warning: destructive") {
		t.Errorf("output should show destructive warning, got:\n%s", output)
	}
}

func TestDestructiveConfirmed(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	ranCommand := ""
	runCapture = func(command string) (string, int, error) {
		ranCommand = command
		return "", 0, nil
	}

	mock := &mockProvider{
		responses: []string{`{"text":"Dangerous command.","commands":["rm -rf /tmp/old"]}`},
	}

	// User chooses run, then confirms
	input := "delete old files\nr\ny\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if ranCommand != "rm -rf /tmp/old" {
		t.Errorf("expected command 'rm -rf /tmp/old', got %q", ranCommand)
	}
}

func TestEmptyInputIgnored(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	mock := &mockProvider{}

	input := "\n\n\nexit\n"
	out := &bytes.Buffer{}

	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if mock.callCount != 0 {
		t.Error("empty lines should not trigger LLM calls")
	}
}

func TestHistoryTruncation(t *testing.T) {
	restore := saveVars(t)
	defer restore()
	stubEnv()

	// Create responses for maxHistoryMsgs+5 turns
	count := maxHistoryMsgs + 5
	responses := make([]string, count)
	for i := range responses {
		responses[i] = fmt.Sprintf("Response %d", i)
	}

	mock := &mockProvider{responses: responses}

	var inputLines []string
	for i := 0; i < count; i++ {
		inputLines = append(inputLines, fmt.Sprintf("message %d", i))
	}
	inputLines = append(inputLines, "exit")
	input := strings.Join(inputLines, "\n") + "\n"

	out := &bytes.Buffer{}
	err := Run(mock, strings.NewReader(input), out)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// The last call should have system + maxHistoryMsgs messages (not unbounded growth).
	lastCall := mock.messages[len(mock.messages)-1]
	// Messages = 1 system + history (capped at maxHistoryMsgs).
	// History has user+assistant pairs, so it grows by 2 per turn.
	// After truncation, at most maxHistoryMsgs + 1 (system).
	maxAllowed := maxHistoryMsgs + 1
	if len(lastCall) > maxAllowed {
		t.Errorf("last call had %d messages, expected at most %d", len(lastCall), maxAllowed)
	}
}
