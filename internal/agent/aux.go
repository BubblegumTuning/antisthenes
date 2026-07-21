package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

// HeuristicSessionTitle builds a short title from the first user message (no network).
func HeuristicSessionTitle(userText string) string {
	s := strings.TrimSpace(userText)
	if s == "" {
		return "New session"
	}
	// First line only.
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	s = strings.Join(strings.Fields(s), " ")
	const maxRunes = 48
	if utf8.RuneCountInString(s) > maxRunes {
		runes := []rune(s)
		s = string(runes[:maxRunes])
		if sp := strings.LastIndex(s, " "); sp > 16 {
			s = s[:sp]
		}
		s = strings.TrimRight(s, " ,;:-") + "…"
	}
	// Capitalize first letter lightly.
	if r, size := utf8.DecodeRuneInString(s); r != utf8.RuneError && unicode.IsLower(r) {
		s = string(unicode.ToUpper(r)) + s[size:]
	}
	return s
}

// SanitizeSessionTitle cleans model output into a single-line window label.
func SanitizeSessionTitle(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, `"'`)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	// Drop common model preambles.
	lower := strings.ToLower(s)
	for _, p := range []string{"title:", "session title:", "here is a title:"} {
		if strings.HasPrefix(lower, p) {
			s = strings.TrimSpace(s[len(p):])
			lower = strings.ToLower(s)
		}
	}
	s = HeuristicSessionTitle(s)
	if s == "" || s == "New session" {
		return ""
	}
	return s
}

// CompleteAux runs a single non-streaming chat completion against an aux model (no tools).
func CompleteAux(ctx context.Context, m config.AuxModel, system, user string) (string, error) {
	if strings.TrimSpace(m.BaseURL) == "" || strings.TrimSpace(m.Model) == "" {
		return "", fmt.Errorf("aux model %q missing base_url or model", m.Name)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// Bound cheap aux calls so title gen cannot hang the async cmd forever.
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	client := newOpenAIClient(m.APIKey, m.BaseURL)
	msgs := []openai.ChatCompletionMessage{}
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: system})
	}
	msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: user})
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       m.Model,
		Messages:    msgs,
		MaxTokens:   64,
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("aux model returned no choices")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// GenerateSessionTitle prefers an aux model with role "title", else heuristic.
func GenerateSessionTitle(ctx context.Context, cfg config.Config, userText string) string {
	fallback := HeuristicSessionTitle(userText)
	aux, ok := cfg.ResolveAuxModel("title")
	if !ok {
		return fallback
	}
	system := "You name chat sessions. Reply with ONLY a concise title (3-8 words). No quotes, no punctuation at the end, no explanation."
	user := "First user message:\n" + userText
	out, err := CompleteAux(ctx, aux, system, user)
	if err != nil {
		return fallback
	}
	if t := SanitizeSessionTitle(out); t != "" {
		return t
	}
	return fallback
}

// RegisterAuxExecutors registers each aux model name into the ExecutorRegistry
// so delegates can target them via ExecutorName.
func RegisterAuxExecutors(cfg config.Config) {
	for _, m := range cfg.AuxModels {
		name := strings.TrimSpace(m.Name)
		if name == "" || strings.TrimSpace(m.Model) == "" || strings.TrimSpace(m.BaseURL) == "" {
			continue
		}
		RegisterExecutor(name, m.Model, m.BaseURL, m.APIKey)
	}
}
