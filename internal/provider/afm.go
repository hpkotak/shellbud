package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	afmStdoutLimitBytes = 1 << 20 // 1 MiB
	afmStderrLimitBytes = 16 << 10
)

// AFMProvider implements Provider via an external AFM bridge executable.
//
// Bridge contract:
// - stdin:  {"model":"...","messages":[{"role":"...","content":"..."}]}
// - stdout: {"content":"assistant response"}
type AFMProvider struct {
	model   string
	command string
}

// NewAFM creates an AFMProvider that shells out to command for each request.
func NewAFM(model, command string) (*AFMProvider, error) {
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("model cannot be empty")
	}
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("afm command cannot be empty")
	}
	return &AFMProvider{model: model, command: command}, nil
}

func (a *AFMProvider) Name() string { return "afm" }

func (a *AFMProvider) Capabilities() Capabilities {
	return Capabilities{
		JSONMode:     true,
		Usage:        false,
		FinishReason: false,
	}
}

func (a *AFMProvider) Available(_ context.Context) error {
	if filepath.IsAbs(a.command) {
		info, err := os.Stat(a.command)
		if err != nil {
			return fmt.Errorf("afm command %q not found: %w", a.command, err)
		}
		if info.IsDir() {
			return fmt.Errorf("afm command %q is a directory", a.command)
		}
		if info.Mode()&0o111 == 0 {
			return fmt.Errorf("afm command %q is not executable", a.command)
		}
		return nil
	}

	if _, err := exec.LookPath(a.command); err != nil {
		return fmt.Errorf("afm command %q not found in PATH: %w", a.command, err)
	}
	return nil
}

func (a *AFMProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	type afmMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type afmRequest struct {
		Model      string       `json:"model"`
		Messages   []afmMessage `json:"messages"`
		ExpectJSON bool         `json:"expect_json,omitempty"`
	}

	apiMessages := make([]afmMessage, len(req.Messages))
	for i, m := range req.Messages {
		apiMessages[i] = afmMessage(m)
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = a.model
	}

	reqBody, err := json.Marshal(afmRequest{
		Model:      model,
		Messages:   apiMessages,
		ExpectJSON: req.ExpectJSON,
	})
	if err != nil {
		return ChatResponse{}, fmt.Errorf("encoding afm request: %w", err)
	}

	cmd := exec.CommandContext(ctx, a.command)
	cmd.Stdin = bytes.NewReader(reqBody)

	stdout := newLimitedBuffer(afmStdoutLimitBytes)
	stderr := newLimitedBuffer(afmStderrLimitBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			return ChatResponse{}, fmt.Errorf("afm bridge execution failed: %w", err)
		}
		return ChatResponse{}, fmt.Errorf("afm bridge execution failed: %w: %s", err, errText)
	}

	if stdout.overflow || stderr.overflow {
		return ChatResponse{}, fmt.Errorf("afm bridge output exceeded limit (%d bytes stdout, %d bytes stderr)",
			afmStdoutLimitBytes, afmStderrLimitBytes)
	}

	var decoded struct {
		Content      string `json:"content"`
		FinishReason string `json:"finish_reason,omitempty"`
		Usage        struct {
			InputTokens  int `json:"input_tokens,omitempty"`
			OutputTokens int `json:"output_tokens,omitempty"`
			TotalTokens  int `json:"total_tokens,omitempty"`
		} `json:"usage,omitempty"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		return ChatResponse{}, fmt.Errorf("decoding afm response: %w", err)
	}

	result := strings.TrimSpace(decoded.Content)
	if result == "" {
		return ChatResponse{}, fmt.Errorf("empty response from model")
	}

	usage := Usage{
		InputTokens:  decoded.Usage.InputTokens,
		OutputTokens: decoded.Usage.OutputTokens,
		TotalTokens:  decoded.Usage.TotalTokens,
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	return ChatResponse{
		Text:         result,
		Raw:          result,
		Structured:   isStructuredJSON(req.ExpectJSON, result),
		FinishReason: decoded.FinishReason,
		Usage:        usage,
	}, nil
}

type limitedBuffer struct {
	max      int
	buf      bytes.Buffer
	overflow bool
}

func newLimitedBuffer(max int) limitedBuffer {
	return limitedBuffer{max: max}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.max - b.buf.Len()
	if remaining <= 0 {
		b.overflow = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.overflow = true
		_, _ = b.buf.Write(p[:remaining])
		return len(p), nil
	}
	n, err := b.buf.Write(p)
	if err != nil {
		return n, err
	}
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

var _ io.Writer = (*limitedBuffer)(nil)
