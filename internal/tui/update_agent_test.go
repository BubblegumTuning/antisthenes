package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestHandleResponseMsg_NoDuplicateUserMessages(t *testing.T) {
	m := Model{
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 10),
		textInput: textarea.New(),
		cfg:       config.Config{AgentName: "Test"},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "reply one"},
	}

	submitted := m
	(&submitted).submitUserMessage("second")
	resp := responseMsg{
		windowIndex: 0,
		messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "first"},
			{Role: "assistant", Content: "reply one"},
			{Role: "user", Content: "second"},
			{Role: "assistant", Content: "reply two"},
		},
	}
	(&submitted).handleResponseMsg(resp)

	userCount := 0
	for _, msg := range submitted.windows[0].Messages {
		if msg.Role == "user" {
			userCount++
		}
	}
	if userCount != 2 {
		t.Fatalf("expected 2 user messages, got %d: %+v", userCount, submitted.windows[0].Messages)
	}

	chat := submitted.renderChat()
	if strings.Count(chat, "You: second") != 1 {
		t.Fatalf("user message rendered more than once:\n%s", chat)
	}
}
