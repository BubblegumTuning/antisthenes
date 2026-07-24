package agent

import (
	ctxbuilder "github.com/nanami/antisthenes/internal/context"
	openai "github.com/sashabaranov/go-openai"
)

// TokenUsage holds last-call and cumulative session token accounting.
// Last* prefer API-reported values when the provider sends usage; otherwise
// they may be Hermes-style estimates (FromAPI=false).
type TokenUsage struct {
	LastPromptTokens     int
	LastCompletionTokens int
	LastTotalTokens      int
	SessionPromptTokens  int
	SessionCompletion    int
	SessionTotalTokens   int
	APICalls             int
	FromAPI              bool // true if Last* came from provider usage
}

// Usage returns a snapshot of token counters (thread-safe).
func (l *Loop) Usage() TokenUsage {
	if l == nil {
		return TokenUsage{}
	}
	l.usageMu.Lock()
	defer l.usageMu.Unlock()
	return l.usage
}

func (l *Loop) recordAPIUsage(u openai.Usage) {
	if l == nil {
		return
	}
	prompt := u.PromptTokens
	completion := u.CompletionTokens
	total := u.TotalTokens
	if total == 0 {
		total = prompt + completion
	}
	if prompt == 0 && completion == 0 && total == 0 {
		return
	}
	l.usageMu.Lock()
	defer l.usageMu.Unlock()
	l.usage.LastPromptTokens = prompt
	l.usage.LastCompletionTokens = completion
	l.usage.LastTotalTokens = total
	l.usage.SessionPromptTokens += prompt
	l.usage.SessionCompletion += completion
	l.usage.SessionTotalTokens += total
	l.usage.APICalls++
	l.usage.FromAPI = true
}

func (l *Loop) recordEstimatedUsage(system string, messages []openai.ChatCompletionMessage, tools []openai.Tool, completionHint int) {
	if l == nil {
		return
	}
	prompt := ctxbuilder.EstimateRequestTokens(system, messages, tools)
	if prompt <= 0 && completionHint <= 0 {
		return
	}
	completion := completionHint
	if completion < 0 {
		completion = 0
	}
	total := prompt + completion
	l.usageMu.Lock()
	defer l.usageMu.Unlock()
	l.usage.LastPromptTokens = prompt
	l.usage.LastCompletionTokens = completion
	l.usage.LastTotalTokens = total
	l.usage.SessionPromptTokens += prompt
	l.usage.SessionCompletion += completion
	l.usage.SessionTotalTokens += total
	l.usage.APICalls++
	l.usage.FromAPI = false
}
