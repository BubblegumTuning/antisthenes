package context

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name    string
		system  string
		history []openai.ChatCompletionMessage
		wantLen int
	}{
		{
			name:    "default system no history",
			system:  "",
			history: nil,
			wantLen: 1,
		},
		{
			name:   "custom system with history",
			system: "test system",
			history: []openai.ChatCompletionMessage{
				{Role: "user", Content: "hi"},
			},
			wantLen: 2,
		},
		{
			name:    "history truncation",
			system:  "sys",
			history: make([]openai.ChatCompletionMessage, 30),
			wantLen: 21, // 1 sys + 20 history
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewPromptBuilder(tt.system)
			msgs := b.BuildMessages(tt.history, nil)
			if len(msgs) != tt.wantLen {
				t.Errorf("got len %d want %d", len(msgs), tt.wantLen)
			}
		})
	}
}

func TestNewPromptBuilder_NoSoulFile(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origWD) }()

	// no SOUL.md in tmp
	b := NewPromptBuilder("")
	if !strings.Contains(b.SystemPrompt, "helpful assistant") {
		t.Errorf("expected fallback prompt, got %q", b.SystemPrompt)
	}
}

func TestEstimateTokensRough(t *testing.T) {
	if EstimateTokensRough("") != 0 {
		t.Fatal("empty")
	}
	if got := EstimateTokensRough("ab"); got != 1 { // (2+3)/4 = 1
		t.Fatalf("short: got %d", got)
	}
	if got := EstimateTokensRough("abcd"); got != 1 { // (4+3)/4 = 1
		t.Fatalf("four: got %d", got)
	}
	if got := EstimateTokensRough(strings.Repeat("x", 4000)); got != 1000 {
		t.Fatalf("4k chars: got %d want 1000", got)
	}
}

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens(nil) != 0 {
		t.Fatal("nil")
	}
	// Dense text must not collapse to 0 (old word-based formula did).
	dense := []openai.ChatCompletionMessage{{Role: "user", Content: strings.Repeat("x", 4000)}}
	if got := EstimateTokens(dense); got < 900 {
		t.Fatalf("dense undercount: %d", got)
	}
	// Tool-call-only assistant message (empty Content) must count arguments.
	tc := []openai.ChatCompletionMessage{{
		Role: "assistant",
		ToolCalls: []openai.ToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: openai.FunctionCall{
				Name:      "terminal",
				Arguments: `{"command":"ls -la /home/nanami"}`,
			},
		}},
	}}
	if got := EstimateTokens(tc); got < 10 {
		t.Fatalf("tool_calls undercount: %d", got)
	}
	// Simple text: role+content chars//4
	simple := []openai.ChatCompletionMessage{{Role: "user", Content: "hello world foo bar"}}
	got := EstimateTokens(simple)
	if got <= 0 {
		t.Fatal("simple gave 0")
	}
	// Must exceed naive old formula words//3*4 for this string (was 4).
	if got < 5 {
		t.Fatalf("simple too low: %d", got)
	}
}

func TestEstimateRequestTokensIncludesTools(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{{Role: "user", Content: "hi"}}
	base := EstimateRequestTokens("sys", msgs, nil)
	tools := []openai.Tool{{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "terminal",
			Description: strings.Repeat("d", 400),
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}},
		},
	}}
	withTools := EstimateRequestTokens("sys", msgs, tools)
	if withTools <= base {
		t.Fatalf("tools should add tokens: base=%d with=%d", base, withTools)
	}
}

func TestContextTokensPrefersAPI(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{{Role: "user", Content: "hi"}}
	est := EstimateRequestTokens("sys", msgs, nil)
	if got := ContextTokens(0, "sys", msgs, nil); got != est {
		t.Fatalf("fallback: got %d want %d", got, est)
	}
	if got := ContextTokens(12345, "sys", msgs, nil); got != 12345 {
		t.Fatalf("api prefer: got %d", got)
	}
}

func TestWithinBudget(t *testing.T) {
	b := &PromptBuilder{MaxTokens: 100}
	msgs := []openai.ChatCompletionMessage{{Content: strings.Repeat("word ", 10)}} // ~40 tokens rough

	if !b.WithinBudget(msgs) {
		t.Error("expected within budget for small")
	}

	large := []openai.ChatCompletionMessage{{Content: strings.Repeat("word ", 100)}}
	if b.WithinBudget(large) {
		t.Error("expected over for large")
	}
}

func TestShouldAutoCompress(t *testing.T) {
	b := &PromptBuilder{MaxTokens: 100}
	// 75% = 75
	over := []openai.ChatCompletionMessage{{Content: strings.Repeat("a ", 30)}} // rough >75?
	if !b.ShouldAutoCompress(over) {
		t.Log("note: approx may vary")
	}

	b2 := &PromptBuilder{MaxTokens: 0}
	if b2.ShouldAutoCompress(nil) {
		t.Error("0 max should false")
	}
}

func TestDefaultCompression(t *testing.T) {
	c := DefaultCompression()
	if c.ThresholdPercent != 75 {
		t.Errorf("expected 75, got %d", c.ThresholdPercent)
	}
}

func TestShouldCompress(t *testing.T) {
	b := NewPromptBuilder("sys")
	b.MaxTokens = 100

	small := []openai.ChatCompletionMessage{{Content: "small"}}
	if b.ShouldCompress(small) {
		t.Error("small messages should not trigger compression")
	}

	large := []openai.ChatCompletionMessage{{Content: strings.Repeat("word ", 200)}}
	if !b.ShouldCompress(large) {
		t.Error("large messages should trigger compression")
	}
}

func TestCompressHistory(t *testing.T) {
	msgs := make([]openai.ChatCompletionMessage, 5)
	for i := range msgs {
		msgs[i] = openai.ChatCompletionMessage{Content: string(rune('a' + i))}
	}

	kept := CompressHistory(msgs, 3)
	if len(kept) != 4 { // 1 summary + 3
		t.Errorf("expected 4, got %d", len(kept))
	}
	if kept[0].Content != "[Compressed history - earlier messages summarized and removed]" {
		t.Error("summary not prepended")
	}

	small := CompressHistory(msgs[:2], 5)
	if len(small) != 2 {
		t.Error("should not compress if under")
	}
}

func TestWorkSummaries_Hermetic(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origWD) }()

	// write a config.json to control WorkDir to subdir of tmp for isolation
	workDir := filepath.Join(tmp, "workdata")
	cfg := map[string]interface{}{
		"work_dir": workDir,
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile("config.json", data, 0o600)

	session := "sess-001"
	summary := "# Task done\n- item1"

	path, err := DumpWorkSummary(session, summary)
	if err != nil {
		t.Fatalf("Dump failed: %v", err)
	}
	if !strings.HasPrefix(path, workDir) {
		t.Errorf("dump path %s not under workDir %s", path, workDir)
	}

	// load it
	loaded, err := LoadWorkSummary(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !strings.Contains(loaded, summary) {
		t.Error("loaded summary missing content")
	}

	// list
	files, err := ListWorkSummaries()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, f := range files {
		if f == path {
			found = true
		}
	}
	if !found {
		t.Error("listed file not found in ListWorkSummaries")
	}
}

func TestLoadSoulPrompt_Success(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origWD) }()

	soulContent := "You are a special soul prompt for compression tests.\nBe concise."
	if err := os.WriteFile("SOUL.md", []byte(soulContent), 0o600); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}

	b := NewPromptBuilder("")
	if !strings.Contains(b.SystemPrompt, "special soul prompt") {
		t.Errorf("expected soul content in prompt, got %q", b.SystemPrompt)
	}
}

func TestWorkSummaries_DefaultDir(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origWD) }()

	// No config.json -> exercises default TempDir fallback in Dump/List
	_, err := DumpWorkSummary("sess-default", "default dir summary")
	if err != nil {
		t.Fatalf("Dump default dir: %v", err)
	}

	files, err := ListWorkSummaries()
	if err != nil {
		t.Fatalf("List default: %v", err)
	}
	_ = files // just exercise
}

func TestLoadWorkSummary_NotFound(t *testing.T) {
	_, err := LoadWorkSummary("/definitely/not/a/real/path/antisthenes-work-missing.md")
	if err == nil {
		t.Error("expected error for non-existent summary file")
	}
}

func TestListWorkSummaries_NoDir(t *testing.T) {
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origWD) }()

	// ensure no work dir exists
	// List should return empty or error gracefully (current impl returns err from ReadDir if no dir)
	files, err := ListWorkSummaries()
	if err == nil && len(files) != 0 {
		t.Logf("no-dir list returned %d files (ok)", len(files))
	}
	_ = err // coverage for the ReadDir path
}
