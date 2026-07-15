package agent

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/nanami/antisthenes/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

// DelegateConfig allows configuring sub-agent behavior.
type DelegateConfig struct {
	Model        string
	BaseURL      string
	APIKey       string
	ExecutorName string // optional named executor from registry (MVP falls back to default)
}

// DefaultDelegateConfig returns sensible defaults matching main loop.
func DefaultDelegateConfig() DelegateConfig {
	return DelegateConfig{
		Model:   "Qwen3.6-MTP-27B-UD-Q4_K_XL.gguf",
		BaseURL: "http://192.168.88.24:8001/v1",
	}
}

// SubAgentResult holds the result of a delegated task.
type SubAgentResult struct {
	TaskID string
	Result string
	Error  error
}

// DelegateTask runs a sub-task using an isolated Loop + memory store.
func DelegateTask(goal string) SubAgentResult {
	return DelegateTaskWithConfig(goal, DefaultDelegateConfig())
}

// DelegateTaskWithConfig runs a sub-task with custom configuration.
// If ExecutorName is set, it resolves via the ExecutorRegistry (MVP uses default model).
func DelegateTaskWithConfig(goal string, cfg DelegateConfig) SubAgentResult {
	// Resolve executor if requested (MVP: falls back to default model)
	if cfg.ExecutorName != "" {
		exec := GetExecutor(cfg.ExecutorName)
		if exec.Model != "" {
			cfg.Model = exec.Model
			cfg.BaseURL = exec.BaseURL
			cfg.APIKey = exec.APIKey
		}
	}

	tmpFile, err := os.CreateTemp("", "antisthenes-delegate-*.db")
	if err != nil {
		return SubAgentResult{
			TaskID: fmt.Sprintf("task-%d", time.Now().UnixNano()),
			Error:  err,
		}
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()

	store, err := memory.NewStore(dbPath)
	if err != nil {
		return SubAgentResult{
			TaskID: fmt.Sprintf("task-%d", time.Now().UnixNano()),
			Error:  err,
		}
	}
	defer store.Close()
	defer os.Remove(dbPath)

	_, err = store.CreateSession()
	if err != nil {
		return SubAgentResult{
			TaskID: fmt.Sprintf("task-%d", time.Now().UnixNano()),
			Error:  err,
		}
	}

	loop := NewLoop(cfg.APIKey, cfg.Model, cfg.BaseURL)
	messages := []openai.ChatCompletionMessage{{Role: "user", Content: goal}}

	updated, err := loop.RunWithTools(context.Background(), messages)
	if err != nil {
		return SubAgentResult{
			TaskID: fmt.Sprintf("task-%d", time.Now().UnixNano()),
			Error:  err,
		}
	}

	result := ""
	for i := len(updated) - 1; i >= 0; i-- {
		if updated[i].Role == "assistant" && updated[i].Content != "" {
			result = updated[i].Content
			break
		}
	}
	if result == "" {
		result = "sub-agent completed (no final assistant message)"
	}

	return SubAgentResult{
		TaskID: fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Result: result,
	}
}

// DelegateMultiple runs several tasks in parallel.
func DelegateMultiple(goals []string) []SubAgentResult {
	var wg sync.WaitGroup
	results := make([]SubAgentResult, len(goals))

	for i, g := range goals {
		wg.Add(1)
		go func(idx int, goal string) {
			defer wg.Done()
			results[idx] = DelegateTask(goal)
		}(i, g)
	}
	wg.Wait()
	return results
}
