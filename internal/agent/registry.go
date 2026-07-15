package agent

import (
	"encoding/json"
	"fmt"
	"sort"

	openai "github.com/sashabaranov/go-openai"
)

// ToolFunc is the signature for executable tools.
type ToolFunc func(args map[string]any) (string, error)

// ApprovalRequest identifies which tool needs confirmation and what to show the user.
type ApprovalRequest struct {
	Tool    string
	Command string
}

// ApprovalHandler is invoked when a tool needs interactive user approval (TUI popup).
// Return approved=false to deny; level controls how long the approval lasts.
type ApprovalHandler func(req ApprovalRequest) (approved bool, level ApprovalLevel)

// ToolRegistry holds available tools for the agent.
type ToolRegistry struct {
	tools           map[string]ToolFunc
	policy          *Policy
	approvalHandler ApprovalHandler
	jobs            *jobManager
}

// SetApprovalHandler registers a callback for interactive approval (used by the TUI).
func (r *ToolRegistry) SetApprovalHandler(h ApprovalHandler) {
	r.approvalHandler = h
}

// requestInteractiveApproval always prompts the handler when set (package installs, etc.).
func (r *ToolRegistry) requestInteractiveApproval(tool, command string) (ok bool, userDenied bool) {
	if r.approvalHandler == nil {
		return false, false
	}
	approved, level := r.approvalHandler(ApprovalRequest{Tool: tool, Command: command})
	if !approved {
		return false, true
	}
	r.policy.Approve(command, level)
	return true, false
}

// resolveApproval checks policy and optionally invokes the approval handler.
// Returns ok=true when the command may proceed (already approved or user approved now).
func (r *ToolRegistry) resolveApproval(tool, command string) (ok bool, userDenied bool) {
	if !r.policy.NeedsApproval(command) {
		return true, false
	}
	if r.approvalHandler == nil {
		return false, false
	}
	approved, level := r.approvalHandler(ApprovalRequest{Tool: tool, Command: command})
	if !approved {
		return false, true
	}
	r.policy.Approve(command, level)
	return true, false
}

func (r *ToolRegistry) Register(name string, fn ToolFunc) {
	r.tools[name] = fn
}

// Call executes a registered tool by name with the given arguments map.
// This is the primary entry point for external systems (MCP, gateway, etc.).
func (r *ToolRegistry) Call(name string, args map[string]any) (string, error) {
	fn, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return fn(args)
}

func (r *ToolRegistry) Execute(name string, argsJSON string) (string, error) {
	fn, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid tool args: %w", err)
	}
	return fn(args)
}

// ToOpenAITools returns OpenAI tool definitions for every tool registered on this registry.
// Schemas come from toolSchemas; dynamically registered tools (e.g. mcp_call) appear when registered.
func (r *ToolRegistry) ToOpenAITools() []openai.Tool {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]openai.Tool, 0, len(names))
	for _, name := range names {
		schema, ok := toolSchemas[name]
		if !ok {
			schema = openai.FunctionDefinition{Description: name}
		}
		fn := schema
		fn.Name = name
		tools = append(tools, openai.Tool{
			Type:     openai.ToolTypeFunction,
			Function: &fn,
		})
	}
	return tools
}
