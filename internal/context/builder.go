package context

import (
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// PromptBuilder handles construction of messages sent to the LLM.
type PromptBuilder struct {
	SystemPrompt string
	MaxHistory   int
	MaxTokens    int
	UseCache     bool
}

// NewPromptBuilder creates a new builder.
// If system is empty, it will try to load SOUL.md from the current directory.
func NewPromptBuilder(system string) *PromptBuilder {
	if system == "" {
		system = loadSoulPrompt()
	}
	if system == "" {
		system = "You are a helpful assistant with access to tools."
	}
	return &PromptBuilder{
		SystemPrompt: system,
		MaxHistory:   20,
		MaxTokens:    160000,
		UseCache:     true,
	}
}

// loadSoulPrompt attempts to read SOUL.md from the current directory.
func loadSoulPrompt() string {
	data, err := os.ReadFile("SOUL.md")
	if err != nil {
		return ""
	}
	return string(data)
}

// BuildMessages constructs the final message list.
func (b *PromptBuilder) BuildMessages(history []openai.ChatCompletionMessage, tools []openai.Tool) []openai.ChatCompletionMessage {
	msgs := make([]openai.ChatCompletionMessage, 0, len(history)+1)

	sys := openai.ChatCompletionMessage{
		Role:    "system",
		Content: b.SystemPrompt,
	}

	msgs = append(msgs, sys)

	start := 0
	if len(history) > b.MaxHistory {
		start = len(history) - b.MaxHistory
	}
	msgs = append(msgs, history[start:]...)

	return msgs
}

// EstimateTokens returns a rough token count.
func EstimateTokens(messages []openai.ChatCompletionMessage) int {
	total := 0
	for _, m := range messages {
		total += len(strings.Fields(m.Content)) / 3 * 4
	}
	return total
}

// WithinBudget checks if messages are under the soft token limit.
func (b *PromptBuilder) WithinBudget(messages []openai.ChatCompletionMessage) bool {
	return EstimateTokens(messages) <= b.MaxTokens
}

// ShouldAutoCompress returns true when usage exceeds 75% of the budget.
func (b *PromptBuilder) ShouldAutoCompress(messages []openai.ChatCompletionMessage) bool {
	if b.MaxTokens == 0 {
		return false
	}
	used := EstimateTokens(messages)
	threshold := int(float64(b.MaxTokens) * 0.75)
	return used > threshold
}
