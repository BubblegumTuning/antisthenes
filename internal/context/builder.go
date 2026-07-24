package context

import (
	"encoding/json"
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

// EstimateTokensRough approximates tokens as ~4 characters per token (Hermes-style).
// Ceiling division so short texts (1–3 chars) never estimate as 0.
func EstimateTokensRough(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

// EstimateTokens returns a rough token count for conversation messages only.
// Prefer EstimateRequestTokens when system + tool schemas are known.
// Prefer API-reported usage when available (see agent.Loop.Usage).
func EstimateTokens(messages []openai.ChatCompletionMessage) int {
	return EstimateMessagesTokens(messages)
}

// EstimateMessagesTokens counts message payload roughly
// (Hermes estimate_messages_tokens_rough): roles, content, tool_calls, tool results.
// Images are a flat ~1500 tokens each rather than raw base64 size.
func EstimateMessagesTokens(messages []openai.ChatCompletionMessage) int {
	totalChars := 0
	imageTokens := 0
	const imageCost = 1500
	for _, m := range messages {
		totalChars += messageEstimateChars(m)
		imageTokens += messageImageTokens(m, imageCost)
	}
	return ((totalChars + 3) / 4) + imageTokens
}

// EstimateToolsTokens counts tool schema payload (Hermes _estimate_tools_tokens_rough).
func EstimateToolsTokens(tools []openai.Tool) int {
	if len(tools) == 0 {
		return 0
	}
	totalChars := 0
	for _, t := range tools {
		if t.Function == nil {
			continue
		}
		totalChars += len(t.Function.Name)
		totalChars += len(t.Function.Description)
		if t.Function.Parameters != nil {
			if b, err := json.Marshal(t.Function.Parameters); err == nil {
				totalChars += len(b)
			}
		}
	}
	return (totalChars + 3) / 4
}

// EstimateRequestTokens estimates a full chat-completions request:
// system prompt + messages + tool schemas (Hermes estimate_request_tokens_rough).
func EstimateRequestTokens(system string, messages []openai.ChatCompletionMessage, tools []openai.Tool) int {
	total := 0
	if system != "" {
		total += EstimateTokensRough(system)
	}
	total += EstimateMessagesTokens(messages)
	total += EstimateToolsTokens(tools)
	return total
}

// ContextTokens prefers API-reported last prompt tokens when positive; otherwise
// falls back to a Hermes-style full-request estimate.
func ContextTokens(apiLastPrompt int, system string, messages []openai.ChatCompletionMessage, tools []openai.Tool) int {
	if apiLastPrompt > 0 {
		return apiLastPrompt
	}
	return EstimateRequestTokens(system, messages, tools)
}

func messageEstimateChars(m openai.ChatCompletionMessage) int {
	n := len(m.Role) + len(m.Content) + len(m.Name) + len(m.ToolCallID)
	for _, tc := range m.ToolCalls {
		n += len(tc.ID) + len(string(tc.Type)) + len(tc.Function.Name) + len(tc.Function.Arguments)
	}
	for _, p := range m.MultiContent {
		n += len(p.Type) + len(p.Text)
		// Image URL / base64 stripped; counted via messageImageTokens.
		if p.ImageURL != nil {
			n += len(p.ImageURL.URL)
			// If looks like data URL, do not count raw base64 as tokens-via-chars.
			if strings.HasPrefix(p.ImageURL.URL, "data:") {
				n -= len(p.ImageURL.URL)
				n += len("[image]")
			}
		}
	}
	return n
}

func messageImageTokens(m openai.ChatCompletionMessage, costPerImage int) int {
	count := 0
	for _, p := range m.MultiContent {
		if p.Type == "image_url" || p.ImageURL != nil {
			count++
		}
	}
	return count * costPerImage
}

// EstimateContext estimates tokens for the builder's system prompt + messages + tools.
func (b *PromptBuilder) EstimateContext(messages []openai.ChatCompletionMessage, tools []openai.Tool) int {
	sys := ""
	if b != nil {
		sys = b.SystemPrompt
	}
	return EstimateRequestTokens(sys, messages, tools)
}

// WithinBudget checks if messages (plus optional tools) are under the soft token limit.
func (b *PromptBuilder) WithinBudget(messages []openai.ChatCompletionMessage) bool {
	return b.WithinBudgetWithTools(messages, nil)
}

// WithinBudgetWithTools checks full request estimate against MaxTokens.
func (b *PromptBuilder) WithinBudgetWithTools(messages []openai.ChatCompletionMessage, tools []openai.Tool) bool {
	if b == nil || b.MaxTokens == 0 {
		return true
	}
	return b.EstimateContext(messages, tools) <= b.MaxTokens
}

// ShouldAutoCompress returns true when message estimate exceeds 75% of the budget.
func (b *PromptBuilder) ShouldAutoCompress(messages []openai.ChatCompletionMessage) bool {
	return b.ShouldAutoCompressWithTools(messages, nil)
}

// ShouldAutoCompressWithTools uses full request estimate (system + messages + tools).
func (b *PromptBuilder) ShouldAutoCompressWithTools(messages []openai.ChatCompletionMessage, tools []openai.Tool) bool {
	if b == nil || b.MaxTokens == 0 {
		return false
	}
	used := b.EstimateContext(messages, tools)
	threshold := int(float64(b.MaxTokens) * 0.75)
	return used > threshold
}
