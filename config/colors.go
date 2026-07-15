package config

// TUIColors holds lipgloss-compatible color tokens for the TUI (numbered 0–255, hex, or ANSI names).
// See DESIGN-TUI.md "Colors & Theming". Empty fields are filled from DefaultTUIColors() at load time.
type TUIColors struct {
	User              string `json:"user"`
	Assistant         string `json:"assistant"`
	AssistantThinking string `json:"assistant_thinking"`
	ToolCall          string `json:"tool_call"`
	ToolResult        string `json:"tool_result"`
	InputBorder       string `json:"input_border"`
	ThinkingBorder    string `json:"thinking_border"`
	Status            string `json:"status"`
	Title             string `json:"title"`
	Error             string `json:"error"`
	Nudge             string `json:"nudge"`
	Compression       string `json:"compression"`
	ModalBorder       string `json:"modal_border"`
	WindowActive      string `json:"window_active"`
	WindowInactive    string `json:"window_inactive"`
	WindowEmpty       string `json:"window_empty"`
	Dim               string `json:"dim"`
	EmptyChat         string `json:"empty_chat"`

	// Thinking is deprecated; use assistant_thinking. Kept for older config.json files.
	Thinking string `json:"thinking,omitempty"`
}

// DefaultTUIColors returns the redesigned amber/green terminal palette (DESIGN-TUI.md).
func DefaultTUIColors() TUIColors {
	return TUIColors{
		User:              "214",
		Assistant:         "82",
		AssistantThinking: "245",
		ToolCall:          "220",
		ToolResult:        "245",
		InputBorder:       "214",
		ThinkingBorder:    "33",
		Status:            "245",
		Title:             "220",
		Error:             "196",
		Nudge:             "245",
		Compression:       "214",
		ModalBorder:       "196",
		WindowActive:      "220",
		WindowInactive:    "245",
		WindowEmpty:       "238",
		Dim:               "241",
		EmptyChat:         "245",
	}
}

// ApplyDefaults fills any empty color fields from DefaultTUIColors().
func (c *TUIColors) ApplyDefaults() {
	if c == nil {
		return
	}
	d := DefaultTUIColors()
	if c.User == "" {
		c.User = d.User
	}
	if c.Assistant == "" {
		c.Assistant = d.Assistant
	}
	if c.AssistantThinking == "" {
		if c.Thinking != "" {
			c.AssistantThinking = c.Thinking
		} else {
			c.AssistantThinking = d.AssistantThinking
		}
	}
	if c.ToolCall == "" {
		c.ToolCall = d.ToolCall
	}
	if c.ToolResult == "" {
		c.ToolResult = d.ToolResult
	}
	if c.InputBorder == "" {
		c.InputBorder = d.InputBorder
	}
	if c.ThinkingBorder == "" {
		c.ThinkingBorder = d.ThinkingBorder
	}
	if c.Status == "" {
		c.Status = d.Status
	}
	if c.Title == "" {
		c.Title = d.Title
	}
	if c.Error == "" {
		c.Error = d.Error
	}
	if c.Nudge == "" {
		c.Nudge = d.Nudge
	}
	if c.Compression == "" {
		c.Compression = d.Compression
	}
	if c.ModalBorder == "" {
		c.ModalBorder = d.ModalBorder
	}
	if c.WindowActive == "" {
		c.WindowActive = d.WindowActive
	}
	if c.WindowInactive == "" {
		c.WindowInactive = d.WindowInactive
	}
	if c.WindowEmpty == "" {
		c.WindowEmpty = d.WindowEmpty
	}
	if c.Dim == "" {
		c.Dim = d.Dim
	}
	if c.EmptyChat == "" {
		c.EmptyChat = d.EmptyChat
	}
}
