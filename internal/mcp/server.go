package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nanami/antisthenes/internal/agent"
)

// Server implements a minimal MCP server over stdio (JSON-RPC 2.0).
type Server struct {
	registry *agent.ToolRegistry
	version  string
}

// NewServer creates an MCP server backed by the given tool registry.
// Version defaults to "dev" unless set via NewServerWithVersion.
func NewServer(registry *agent.ToolRegistry) *Server {
	return NewServerWithVersion(registry, "")
}

// NewServerWithVersion is like NewServer but sets serverInfo.version (e.g. "0.3.1").
func NewServerWithVersion(registry *agent.ToolRegistry, version string) *Server {
	v := strings.TrimSpace(version)
	if v == "" {
		v = "dev"
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return &Server{registry: registry, version: v}
}

// Run starts the stdio JSON-RPC loop. Blocks until stdin is closed.
func (s *Server) Run() error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(writer, nil, -32700, "Parse error")
			continue
		}

		resp := s.handleRequest(req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			fmt.Fprintln(writer, string(data))
		}
	}
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

func (s *Server) handleRequest(req JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]string{
					"name":    "antisthenes",
					"version": s.version,
				},
			},
		}

	case "notifications/initialized", "initialized":
		// Client lifecycle notification — no JSON-RPC response.
		return nil

	case "ping":
		if req.ID == nil {
			return nil
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}

	case "tools/list":
		tools := s.listTools()
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": tools},
		}

	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.errorResponse(req.ID, -32700, "invalid params")
		}

		result, err := s.callTool(params.Name, params.Arguments)
		if err != nil {
			return s.errorResponse(req.ID, -32603, err.Error())
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]string{{"type": "text", "text": result}},
			},
		}

	default:
		// Unknown notifications must not error (no id → no response).
		if req.ID == nil {
			return nil
		}
		return s.errorResponse(req.ID, -32601, "Method not found: "+req.Method)
	}
}

func (s *Server) listTools() []map[string]any {
	openaiTools := s.registry.ToOpenAITools()
	out := make([]map[string]any, 0, len(openaiTools))
	for _, t := range openaiTools {
		if t.Function == nil {
			continue
		}
		schema := t.Function.Parameters
		if schema == nil {
			schema = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
		out = append(out, map[string]any{
			"name":        t.Function.Name,
			"description": t.Function.Description,
			"inputSchema": schema,
		})
	}
	return out
}

func (s *Server) callTool(name string, args map[string]any) (string, error) {
	// Full execution via the ToolRegistry (policy, approval, and all side effects preserved)
	return s.registry.Call(name, args)
}

func (s *Server) writeError(w io.Writer, id any, code int, msg string) {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: msg},
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintln(w, string(data))
}

func (s *Server) errorResponse(id any, code int, msg string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: msg},
	}
}
