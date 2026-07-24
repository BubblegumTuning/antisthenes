package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanami/antisthenes/config"

	openai "github.com/sashabaranov/go-openai"
)

// CompressionConfig controls when automatic compression triggers.
type CompressionConfig struct {
	ThresholdPercent int // e.g. 75 means trigger at 75% of MaxTokens
}

// DefaultCompression returns a reasonable default config.
func DefaultCompression() CompressionConfig {
	return CompressionConfig{ThresholdPercent: 75}
}

// ShouldCompress checks if we are over the threshold.
func (b *PromptBuilder) ShouldCompress(messages []openai.ChatCompletionMessage) bool {
	return b.ShouldAutoCompress(messages)
}

// DumpWorkSummary writes a summary of current work to configured WorkDir (from Config) for later reload.
func DumpWorkSummary(sessionID string, summary string) (string, error) {
	wd := config.Load().WorkDir
	if wd == "" {
		wd = config.DefaultWorkDir()
	}
	if err := os.MkdirAll(wd, 0o700); err != nil {
		return "", err
	}
	filename := filepath.Join(wd, fmt.Sprintf("antisthenes-work-%s-%d.md", sessionID, time.Now().Unix()))
	content := fmt.Sprintf("# Work Summary - %s\n\n%s\n", time.Now().Format(time.RFC3339), summary)
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", err
	}
	return filename, nil
}

// LoadWorkSummary reads a previously dumped summary.
func LoadWorkSummary(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListWorkSummaries lists available dump files in configured WorkDir (from Config).
func ListWorkSummaries() ([]string, error) {
	wd := config.Load().WorkDir
	if wd == "" {
		wd = config.DefaultWorkDir()
	}
	entries, err := os.ReadDir(wd)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "antisthenes-work-") && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, filepath.Join(wd, e.Name()))
		}
	}
	return files, nil
}

// CompressHistory performs a simple compression: keeps the most recent messages
// and prepends a summary placeholder when over threshold.
func CompressHistory(messages []openai.ChatCompletionMessage, maxKeep int) []openai.ChatCompletionMessage {
	if len(messages) <= maxKeep {
		return messages
	}

	summary := openai.ChatCompletionMessage{
		Role:    "system",
		Content: "[Compressed history - earlier messages summarized and removed]",
	}

	kept := messages[len(messages)-maxKeep:]
	return append([]openai.ChatCompletionMessage{summary}, kept...)
}
