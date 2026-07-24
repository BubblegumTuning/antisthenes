package agent

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"

	ctxbuilder "github.com/nanami/antisthenes/internal/context"
	openai "github.com/sashabaranov/go-openai"
)

// Loop is the minimal streaming agent loop (v0).
type Loop struct {
	client   *openai.Client
	model    string
	registry *ToolRegistry
	builder  *ctxbuilder.PromptBuilder

	usageMu sync.Mutex
	usage   TokenUsage
}

// NewLoop creates a new agent loop. baseURL can be empty for default OpenAI.
func NewLoop(apiKey, model, baseURL string) *Loop {
	client := newOpenAIClient(apiKey, baseURL)

	return &Loop{
		client:   client,
		model:    model,
		registry: NewToolRegistry(),
		builder:  ctxbuilder.NewPromptBuilder(""),
	}
}

// NewLoopWithRegistry creates a loop using a pre-configured ToolRegistry
// (useful for registering extra tools like mcp_call).
func NewLoopWithRegistry(apiKey, model, baseURL string, reg *ToolRegistry) *Loop {
	client := newOpenAIClient(apiKey, baseURL)

	return &Loop{
		client:   client,
		model:    model,
		registry: reg,
		builder:  ctxbuilder.NewPromptBuilder(""),
	}
}

// SetApprovalHandler wires interactive approval for policy-gated tools (bash, create_dir, etc.).
func (l *Loop) SetApprovalHandler(h ApprovalHandler) {
	l.registry.SetApprovalHandler(h)
}

// Registry exposes the loop's tool registry (for /tools introspection in the TUI).
func (l *Loop) Registry() *ToolRegistry {
	if l == nil {
		return nil
	}
	return l.registry
}

// Builder exposes the prompt builder (system prompt + budget helpers).
func (l *Loop) Builder() *ctxbuilder.PromptBuilder {
	if l == nil {
		return nil
	}
	return l.builder
}

// RunStream executes one turn with streaming, runs tools, and returns the full
// updated message history (same contract as RunWithTools).
func (l *Loop) RunStream(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) ([]openai.ChatCompletionMessage, error) {
	// Use PromptBuilder for system prompt + truncation, matching RunWithTools path.
	// This was bypassed in RunStream, causing no system prompt on LLM calls during tool recursion.
	toolsToUse := l.registry.ToOpenAITools()
	promptMsgs := l.builder.BuildMessages(messages, toolsToUse)

	req := openai.ChatCompletionRequest{
		Model:    l.model,
		Messages: promptMsgs,
		Tools:    toolsToUse,
		Stream:   true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}

	stream, err := l.openStream(ctx, req)
	if err != nil {
		return messages, err
	}
	defer stream.Close()

	var msg openai.ChatCompletionMessage
	msg.Role = "assistant"
	var lastUsage *openai.Usage
	sys := ""
	if l.builder != nil {
		sys = l.builder.SystemPrompt
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return messages, err
		}
		if resp.Usage != nil {
			u := *resp.Usage
			lastUsage = &u
		}
		if len(resp.Choices) == 0 {
			continue
		}

		delta := resp.Choices[0].Delta

		if delta.Content != "" {
			// fmt.Print(delta.Content) // disabled for TUI compatibility
			msg.Content += delta.Content
		}

		// Accumulate tool calls robustly (map by ID handles split deltas reliably)
		// Falls back to last entry when ID is empty on continuation chunks (common in some providers)
		if msg.ToolCalls == nil {
			msg.ToolCalls = []openai.ToolCall{}
		}
		toolCallMap := make(map[string]*openai.ToolCall)
		for i := range msg.ToolCalls {
			if msg.ToolCalls[i].ID != "" {
				toolCallMap[msg.ToolCalls[i].ID] = &msg.ToolCalls[i]
			}
		}

		// (assistant with tool_calls, then tool results). Fixes malformed state for LLM.

		for _, tc := range delta.ToolCalls {
			if tc.Function.Name == "" && tc.Function.Arguments == "" {
				continue
			}
			var target *openai.ToolCall
			if tc.ID != "" {
				if t, ok := toolCallMap[tc.ID]; ok {
					target = t
				} else {
					msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
						ID:       tc.ID,
						Type:     tc.Type,
						Function: openai.FunctionCall{},
					})
					target = &msg.ToolCalls[len(msg.ToolCalls)-1]
					toolCallMap[tc.ID] = target
				}
			} else if len(msg.ToolCalls) > 0 {
				// continuation chunk without ID: append to most recent
				target = &msg.ToolCalls[len(msg.ToolCalls)-1]
			}
			if target != nil {
				target.Function.Name += tc.Function.Name
				target.Function.Arguments += tc.Function.Arguments
			}
		}
	}

	if lastUsage != nil && (lastUsage.TotalTokens > 0 || lastUsage.PromptTokens > 0 || lastUsage.CompletionTokens > 0) {
		l.recordAPIUsage(*lastUsage)
	} else {
		compHint := ctxbuilder.EstimateTokensRough(msg.Content)
		for _, tc := range msg.ToolCalls {
			compHint += ctxbuilder.EstimateTokensRough(tc.Function.Name) + ctxbuilder.EstimateTokensRough(tc.Function.Arguments)
		}
		l.recordEstimatedUsage(sys, promptMsgs, toolsToUse, compHint)
	}

	// Execute tools if present (full streaming + tool support)
	// Normalize arguments to valid JSON to survive any concat edge cases from streaming
	if len(msg.ToolCalls) > 0 {
		for i := range msg.ToolCalls {
			args := strings.TrimSpace(msg.ToolCalls[i].Function.Arguments)
			if args == "" {
				args = "{}"
			} else if !json.Valid([]byte(args)) {
				// last-ditch: wrap if it looks like a bare object fragment
				if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
					// already object-like, trust it
				} else {
					args = "{}"
				}
			}
			msg.ToolCalls[i].Function.Arguments = args
		}

		// Append the assistant message containing ToolCalls *before* tool results and recurse.
		// Critical fix for RunStream: ensures recursion receives correct OpenAI history sequence
		// (assistant(tool_calls), tool, final).
		// Previously missing this append + no PromptBuilder caused malformed conversation state,
		// model repeating tool calls, erratic behaviour, or prolonged thinking (TUI spinner never clears) in normal mode.
		// Now consistent with RunWithTools (append before tools) and uses builder for system prompt.
		messages = append(messages, msg)

		toolMsgs := ExecuteToolCalls(l.registry, msg.ToolCalls)
		messages = append(messages, toolMsgs...)

		// Recurse for final answer after tool results
		return l.RunStream(ctx, messages, nil)
	}

	messages = append(messages, msg)
	return messages, nil
}

// RunAgent runs one user turn: streaming with retries, falling back to non-streaming
// when the local server drops the connection mid-request (common with llama.cpp).
func (l *Loop) RunAgent(ctx context.Context, messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, error) {
	history := append([]openai.ChatCompletionMessage(nil), messages...)
	updated, err := l.RunStream(ctx, history, nil)
	if err == nil {
		return updated, nil
	}
	if !isTransientNetErr(err) {
		return messages, err
	}
	updated, fbErr := l.RunWithTools(ctx, append([]openai.ChatCompletionMessage(nil), messages...))
	if fbErr != nil {
		return messages, err
	}
	return updated, nil
}

// RunWithTools runs the agent, automatically executing tool calls when returned.
func (l *Loop) RunWithTools(ctx context.Context, messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, error) {
	tools := l.registry.ToOpenAITools()
	// Use the context builder
	promptMsgs := l.builder.BuildMessages(messages, tools)

	resp, err := l.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    l.model,
		Messages: promptMsgs,
		Tools:    tools,
	})
	if err != nil {
		return messages, err
	}

	sys := ""
	if l.builder != nil {
		sys = l.builder.SystemPrompt
	}
	if resp.Usage.TotalTokens > 0 || resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
		l.recordAPIUsage(resp.Usage)
	} else {
		compHint := 0
		if len(resp.Choices) > 0 {
			compHint = ctxbuilder.EstimateTokensRough(resp.Choices[0].Message.Content)
			for _, tc := range resp.Choices[0].Message.ToolCalls {
				compHint += ctxbuilder.EstimateTokensRough(tc.Function.Name) + ctxbuilder.EstimateTokensRough(tc.Function.Arguments)
			}
		}
		l.recordEstimatedUsage(sys, promptMsgs, tools, compHint)
	}

	choice := resp.Choices[0]
	messages = append(messages, choice.Message)

	if len(choice.Message.ToolCalls) > 0 {
		toolMsgs := ExecuteToolCalls(l.registry, choice.Message.ToolCalls)
		messages = append(messages, toolMsgs...)
		// Recurse for final answer after tool results
		return l.RunWithTools(ctx, messages)
	}

	return messages, nil
}
