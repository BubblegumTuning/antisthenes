package agent

import (
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// Executor represents a named execution target (deep-thinker, coder, etc.).
type Executor struct {
	Name    string
	Model   string
	BaseURL string
	APIKey  string
}

// ExecutorRegistry holds the known executors. MVP: all point to the default model.
type ExecutorRegistry struct {
	mu        sync.RWMutex
	executors map[string]Executor
	defaults  Executor // fallback
}

var registry = &ExecutorRegistry{
	executors: make(map[string]Executor),
}

// RegisterExecutor registers or updates an executor. In MVP all use the same model.
func RegisterExecutor(name string, model, baseURL, apiKey string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.executors[name] = Executor{
		Name:    name,
		Model:   model,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}
}

// GetExecutor returns the executor or falls back to default.
func GetExecutor(name string) Executor {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	if e, ok := registry.executors[name]; ok {
		return e
	}
	return registry.defaults
}

// SetDefaultExecutor sets the model used by all executors in MVP mode.
func SetDefaultExecutor(model, baseURL, apiKey string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.defaults = Executor{
		Name:    "default",
		Model:   model,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}
}

// ListExecutors returns all registered executor names.
func ListExecutors() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	names := make([]string, 0, len(registry.executors)+1)
	for n := range registry.executors {
		names = append(names, n)
	}
	names = append(names, "auto")
	return names
}

// MVPExecutorNames are the supervised-mode choices from skills/iterative_per.
var MVPExecutorNames = []string{"auto", "coder", "deep-thinker", "orchestrator"}

// EnsureMVPExecutors registers the standard named executors against the given
// default model endpoint. MVP: all names resolve to the same model.
func EnsureMVPExecutors(model, baseURL, apiKey string) {
	SetDefaultExecutor(model, baseURL, apiKey)
	for _, name := range MVPExecutorNames {
		if name == "auto" {
			continue
		}
		RegisterExecutor(name, model, baseURL, apiKey)
	}
}

// ExecuteToolCalls executes a list of tool calls using the registry and returns
// the corresponding "tool" role messages to append to the conversation.
// Normalization of arguments is expected to have been done by the caller (see RunStream).
// This extracts the duplicated execution logic from loop.go RunStream and RunWithTools.
func ExecuteToolCalls(registry *ToolRegistry, toolCalls []openai.ToolCall) []openai.ChatCompletionMessage {
	toolMessages := make([]openai.ChatCompletionMessage, 0, len(toolCalls))
	for _, tc := range toolCalls {
		result, err := registry.Execute(tc.Function.Name, tc.Function.Arguments)
		if err != nil {
			result = "error: " + err.Error()
		}
		if strings.TrimSpace(result) == "" {
			result = "(no output)"
		}
		toolMessages = append(toolMessages, openai.ChatCompletionMessage{
			Role:       "tool",
			ToolCallID: tc.ID,
			Content:    result,
		})
	}
	return toolMessages
}
