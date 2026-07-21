package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestToolRegistry_ListDir(t *testing.T) {
	r := NewToolRegistry()
	result, err := r.Call("list_dir", map[string]any{"path": "."})
	if err != nil {
		t.Fatalf("list_dir failed: %v", err)
	}
	if result == "" {
		t.Error("list_dir returned empty result")
	}
}

func TestToolRegistry_ListDir_ExecutePath(t *testing.T) {
	r := NewToolRegistry()
	result, err := r.Execute("list_dir", `{"path": "."}`)
	if err != nil {
		t.Fatalf("Execute list_dir failed: %v", err)
	}
	if result == "" {
		t.Error("Execute(list_dir) returned empty result")
	}
	if !strings.Contains(result, "loop.go") && !strings.Contains(result, "tools.go") {
		t.Error("list_dir result missing expected source files")
	}
}

func TestToolRegistry_CreateDir(t *testing.T) {
	r := NewToolRegistry()
	dir := "test_create_dir_" + strings.ReplaceAll(t.Name(), "/", "_") + "/sub/nested"
	defer os.RemoveAll("test_create_dir_" + strings.ReplaceAll(t.Name(), "/", "_"))

	result, err := r.Call("create_dir", map[string]any{"path": dir})
	if err != nil {
		t.Fatalf("create_dir failed: %v", err)
	}
	if !strings.Contains(result, "Directory created") && !strings.Contains(result, "created") {
		t.Errorf("unexpected create_dir result: %s", result)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("directory was not created")
	}
}

func TestToolRegistry_CreateDir_ExecutePath(t *testing.T) {
	r := NewToolRegistry()
	dir := "test_create_dir_exec_" + strings.ReplaceAll(t.Name(), "/", "_") + "/nested"
	defer os.RemoveAll("test_create_dir_exec_" + strings.ReplaceAll(t.Name(), "/", "_"))

	result, err := r.Execute("create_dir", `{"path":"`+dir+`"}`)
	if err != nil {
		t.Fatalf("Execute create_dir failed: %v", err)
	}
	if !strings.Contains(result, "Directory created") && !strings.Contains(result, "created") {
		t.Errorf("unexpected Execute result: %s", result)
	}
}

func TestToolRegistry_WriteFileCreatesParents(t *testing.T) {
	r := NewToolRegistry()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a", "b", "c.txt")
	content := "test content from regression test"

	result, err := r.Call("write_file", map[string]any{"path": path, "content": content})
	if err != nil {
		t.Fatalf("write_file with parents failed: %v", err)
	}
	if !strings.Contains(result, "File written") {
		t.Errorf("unexpected write_file result: %s", result)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != content {
		t.Errorf("file not written correctly or parents not created")
	}
}

func TestToolRegistry_ToOpenAIToolsIncludesCreateDir(t *testing.T) {
	r := NewToolRegistry()
	if !toolListed(r.ToOpenAITools(), "create_dir") {
		t.Error("create_dir not present in ToOpenAITools()")
	}
}

func TestToolRegistry_AllToolsHaveParametersSchema(t *testing.T) {
	r := NewToolRegistry()
	// Assert every tool present on the base registry has a Parameters object schema.
	for _, tool := range r.ToOpenAITools() {
		if tool.Function == nil {
			t.Fatal("nil function")
		}
		if tool.Function.Parameters == nil {
			t.Errorf("tool %q missing Parameters schema (MCP/OpenAI clients need inputSchema)", tool.Function.Name)
		}
	}
	// mcp_call / mcp_list_tools schemas live in toolSchemas even before registration
	for _, name := range []string{"mcp_call", "mcp_list_tools"} {
		if schema, ok := toolSchemas[name]; !ok || schema.Parameters == nil {
			t.Errorf("%s schema missing Parameters", name)
		}
	}
}

func TestToolRegistry_ToOpenAIToolsIncludesMCPCallWhenRegistered(t *testing.T) {
	r := NewToolRegistry()
	if toolListed(r.ToOpenAITools(), "mcp_call") {
		t.Error("mcp_call should not appear before RegisterMCPCallTool")
	}

	r.Register("mcp_call", func(map[string]any) (string, error) { return "ok", nil })
	if !toolListed(r.ToOpenAITools(), "mcp_call") {
		t.Error("mcp_call not present after registration")
	}
}

func toolListed(tools []openai.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Function != nil && tool.Function.Name == name {
			return true
		}
	}
	return false
}

func TestExecuteToolCalls(t *testing.T) {
	r := NewToolRegistry()

	toolCalls := []openai.ToolCall{
		{
			ID: "call_test_1",
			Function: openai.FunctionCall{
				Name:      "echo",
				Arguments: `{"message":"from test"}`,
			},
		},
	}

	msgs := ExecuteToolCalls(r, toolCalls)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "tool" {
		t.Errorf("expected role 'tool', got %s", msgs[0].Role)
	}
	if msgs[0].ToolCallID != "call_test_1" {
		t.Errorf("expected ToolCallID 'call_test_1', got %s", msgs[0].ToolCallID)
	}
	if !strings.Contains(msgs[0].Content, "from test") {
		t.Errorf("result did not contain input: %s", msgs[0].Content)
	}

	badCalls := []openai.ToolCall{
		{
			ID: "call_bad",
			Function: openai.FunctionCall{
				Name:      "nonexistent_tool",
				Arguments: `{}`,
			},
		},
	}
	errMsgs := ExecuteToolCalls(r, badCalls)
	if len(errMsgs) != 1 {
		t.Fatalf("expected 1 error msg, got %d", len(errMsgs))
	}
	if !strings.Contains(errMsgs[0].Content, "error:") {
		t.Errorf("expected error in content for bad tool: %s", errMsgs[0].Content)
	}
}
