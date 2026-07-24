package agent

import (
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestRecordAPIUsageAccumulates(t *testing.T) {
	l := &Loop{}
	l.recordAPIUsage(openai.Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120})
	l.recordAPIUsage(openai.Usage{PromptTokens: 150, CompletionTokens: 10, TotalTokens: 160})
	u := l.Usage()
	if !u.FromAPI {
		t.Fatal("FromAPI")
	}
	if u.LastPromptTokens != 150 || u.LastCompletionTokens != 10 || u.LastTotalTokens != 160 {
		t.Fatalf("last: %+v", u)
	}
	if u.SessionPromptTokens != 250 || u.SessionCompletion != 30 || u.SessionTotalTokens != 280 {
		t.Fatalf("session: %+v", u)
	}
	if u.APICalls != 2 {
		t.Fatalf("calls %d", u.APICalls)
	}
}

func TestRecordEstimatedUsage(t *testing.T) {
	l := &Loop{}
	msgs := []openai.ChatCompletionMessage{{Role: "user", Content: strings.Repeat("x", 400)}}
	l.recordEstimatedUsage("system prompt here", msgs, nil, 50)
	u := l.Usage()
	if u.FromAPI {
		t.Fatal("expected estimate")
	}
	if u.LastPromptTokens <= 0 {
		t.Fatal("prompt")
	}
	if u.LastCompletionTokens != 50 {
		t.Fatalf("completion %d", u.LastCompletionTokens)
	}
	if u.SessionTotalTokens != u.LastTotalTokens {
		t.Fatalf("session %+v", u)
	}
}

func TestRecordAPIUsageIgnoresEmpty(t *testing.T) {
	l := &Loop{}
	l.recordAPIUsage(openai.Usage{})
	if l.Usage().APICalls != 0 {
		t.Fatal("empty should not count")
	}
}
