package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Client is a minimal MCP client over stdio (subprocess).
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	nextID int
}

// NewClient launches an MCP server as a subprocess and returns a connected client.
func NewClient(command string, args ...string) (*Client, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// Discard server stderr so banners/logs cannot block the pipe.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		nextID: 1,
	}

	// Perform initialize handshake
	if _, err := c.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "antisthenes",
			"version": "client",
		},
	}); err != nil {
		c.Close()
		return nil, fmt.Errorf("initialize failed: %w", err)
	}
	// MCP lifecycle: client notifies server it is ready (no response).
	if err := c.notify("notifications/initialized", map[string]any{}); err != nil {
		c.Close()
		return nil, fmt.Errorf("initialized notification failed: %w", err)
	}
	return c, nil
}

// ListTools returns the list of tools from the remote MCP server.
func (c *Client) ListTools() ([]map[string]any, error) {
	resp, err := c.call("tools/list", nil)
	if err != nil {
		return nil, err
	}
	result, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response")
	}
	tools, _ := result["tools"].([]any)
	var out []map[string]any
	for _, t := range tools {
		if m, ok := t.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// CallTool invokes a tool on the remote server.
func (c *Client) CallTool(name string, args map[string]any) (string, error) {
	params := map[string]any{
		"name":      name,
		"arguments": args,
	}
	resp, err := c.call("tools/call", params)
	if err != nil {
		return "", err
	}
	result, ok := resp.(map[string]any)
	if !ok {
		return "", fmt.Errorf("unexpected response type")
	}
	content, _ := result["content"].([]any)
	if len(content) > 0 {
		if item, ok := content[0].(map[string]any); ok {
			if text, ok := item["text"].(string); ok {
				return text, nil
			}
		}
	}
	return "", fmt.Errorf("no text content in response")
}

func (c *Client) call(method string, params any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}

	data, _ := json.Marshal(req)
	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		return nil, err
	}

	line, err := c.stdout.ReadString('\n')
	if err != nil {
		return nil, err
	}

	var resp struct {
		ID     any           `json:"id"`
		Result any           `json:"result"`
		Error  *JSONRPCError `json:"error"`
	}
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

// notify sends a JSON-RPC notification (no id, no response expected).
func (c *Client) notify(method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

// Close terminates the MCP server process.
func (c *Client) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}
