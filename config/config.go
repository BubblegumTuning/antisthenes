package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Endpoint represents a model endpoint configuration.
type Endpoint struct {
	Name    string `json:"name"`
	Model   string `json:"model"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
}

// XAITokens stores OAuth tokens for xAI.
type XAITokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// GatewayConfig holds settings for messaging adapters (Telegram, etc.)
type GatewayConfig struct {
	TelegramEnabled bool   `json:"telegram_enabled"`
	TelegramToken   string `json:"telegram_token,omitempty"`
	TelegramChatID  string `json:"telegram_chat_id,omitempty"`
}

// Config holds settings for Antisthenes.
type Config struct {
	AgentName      string        `json:"agent_name"`
	ActiveEndpoint string        `json:"active_endpoint"`
	Endpoints      []Endpoint    `json:"endpoints"`
	XAI            XAITokens     `json:"xai_oauth"`
	ShowThinking   bool          `json:"show_thinking"`
	MaxTokens      int           `json:"max_tokens"`
	Gateway        GatewayConfig `json:"gateway"`
	DebugLogging   bool          `json:"debug_logging"`
	DBPath         string        `json:"db_path"`
	WorkDir        string        `json:"work_dir"`
	EditHeight     int           `json:"edit_height"`
	// Phase 4 per DESIGN-TUI.md: configurable auto-scroll, tool dump flag, chat colors
	AutoScroll        bool `json:"auto_scroll"`
	ShowFullToolDumps bool `json:"show_full_tool_dumps"`
	// MarkdownEnabled renders inline markdown in assistant chat (bold, italic, code, links).
	MarkdownEnabled bool `json:"markdown_enabled"`
	// ClearWithoutConfirm skips the /clear and /new confirmation modal when true.
	ClearWithoutConfirm bool `json:"clear_without_confirm"`
	// ApprovalsWithoutConfirm skips TUI approval modals per tool when true (see config.example.json).
	ApprovalsWithoutConfirm map[string]bool `json:"approvals_without_confirm"`
	// cron_enabled: per DESIGN-TUI.md phase 5 (Integration & correctness). Default false during TUI rebuild to avoid integration surface; right slot reserved for notifications when enabled.
	CronEnabled bool `json:"cron_enabled"`
	// NmapEnabled registers nmap_scan on the tool registry (default true when omitted from config.json).
	NmapEnabled bool `json:"nmap_enabled"`
	// NetworkStatusEnabled registers network_status on the tool registry (default false when omitted).
	NetworkStatusEnabled bool `json:"network_status_enabled"`
	// Phase 7 per DESIGN-TUI.md: in-memory input history (Up/Down in edit box).
	InputHistoryEnabled bool      `json:"input_history_enabled"`
	InputHistorySize    int       `json:"input_history_size"`
	Colors              TUIColors `json:"colors"`
}

// DefaultConfig returns a configuration with both local and xAI endpoints.
func DefaultConfig() Config {
	return Config{
		ActiveEndpoint: "local",
		AgentName:      "Antisthenes",
		Endpoints: []Endpoint{
			{
				Name:    "local",
				Model:   "Qwen3.6-MTP-27B-UD-Q4_K_XL.gguf",
				BaseURL: "http://192.168.88.24:8001/v1",
			},
			{
				Name:    "xai",
				Model:   "grok-3",
				BaseURL: "https://api.x.ai/v1",
			},
		},
		ShowThinking: true,
		MaxTokens:    160000,
		Gateway: GatewayConfig{
			TelegramEnabled: false,
		},
		DBPath:     filepath.Join(os.TempDir(), "antisthenes-data", "antisthenes-demo.db"),
		WorkDir:    filepath.Join(os.TempDir(), "antisthenes-data"),
		EditHeight: 3,
		// Phase 4 defaults per DESIGN-TUI.md (auto_scroll true, show_full false, colors for renderChat)
		AutoScroll:        true,
		ShowFullToolDumps: false,
		MarkdownEnabled:   true,
		// cron_enabled default false per DESIGN-TUI.md phase 5 (Integration & correctness). Disabled during TUI rebuild; right notification slot reserved.
		CronEnabled:         false,
		NmapEnabled:         true,
		InputHistoryEnabled: true,
		InputHistorySize:    50,
		Colors:              DefaultTUIColors(),
	}
}

// NmapOn reports whether nmap_scan is registered (default true when config key omitted).
func (c Config) NmapOn() bool {
	return c.NmapEnabled
}

// NetworkStatusOn reports whether network_status is registered (default false when config key omitted).
func (c Config) NetworkStatusOn() bool {
	return c.NetworkStatusEnabled
}

// InputHistoryOn reports whether Up/Down input history is enabled (DESIGN-TUI.md phase 7).
func (c Config) InputHistoryOn() bool {
	return c.InputHistoryEnabled
}

// InputHistoryMax returns the capped history size (default 50).
func (c Config) InputHistoryMax() int {
	if c.InputHistorySize <= 0 {
		return 50
	}
	return c.InputHistorySize
}

// Load reads config from config.json or returns defaults.
func Load() Config {
	data, err := os.ReadFile("config.json")
	if err != nil {
		cfg := DefaultConfig()
		_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0700)
		_ = os.MkdirAll(cfg.WorkDir, 0700)
		return cfg
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		cfg = DefaultConfig()
		_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0700)
		_ = os.MkdirAll(cfg.WorkDir, 0700)
		return cfg
	}
	if cfg.DBPath == "" || cfg.WorkDir == "" {
		d := DefaultConfig()
		if cfg.DBPath == "" {
			cfg.DBPath = d.DBPath
		}
		if cfg.WorkDir == "" {
			cfg.WorkDir = d.WorkDir
		}
	}
	if cfg.AgentName == "" {
		cfg.AgentName = DefaultConfig().AgentName
	}
	if cfg.EditHeight <= 0 {
		cfg.EditHeight = 3
	}
	// Phase 4: default auto_scroll to true (absent json key yields false in Go; force per DESIGN)
	// cron_enabled: absent key or false is correct default per DESIGN-TUI.md phase 5 (Integration & correctness). No force; Default sets false. Enable via config.json "cron_enabled": true to route cron output exclusively through TUI model msgs (right status slot).
	if !cfg.AutoScroll {
		cfg.AutoScroll = true
	}
	// ShowFullToolDumps defaults to false (zero value OK).
	applyInputHistoryDefaults(&cfg, data)
	applyMarkdownDefaults(&cfg, data)
	applyNmapDefaults(&cfg, data)
	cfg.Colors.ApplyDefaults()
	_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0700)
	_ = os.MkdirAll(cfg.WorkDir, 0700)
	return cfg
}

// applyNmapDefaults sets nmap_enabled true when the key is omitted from config.json.
func applyNmapDefaults(cfg *Config, raw []byte) {
	if len(raw) > 0 && !strings.Contains(string(raw), `"nmap_enabled"`) {
		cfg.NmapEnabled = true
	}
}

// applyMarkdownDefaults sets markdown_enabled true when the key is omitted from config.json.
func applyMarkdownDefaults(cfg *Config, raw []byte) {
	if len(raw) > 0 && !strings.Contains(string(raw), `"markdown_enabled"`) {
		cfg.MarkdownEnabled = true
	}
}

// applyInputHistoryDefaults sets phase-7 input history defaults; omitted json keys default enabled.
func applyInputHistoryDefaults(cfg *Config, raw []byte) {
	if cfg.InputHistorySize <= 0 {
		cfg.InputHistorySize = 50
	}
	if len(raw) > 0 && !strings.Contains(string(raw), `"input_history_enabled"`) {
		cfg.InputHistoryEnabled = true
	}
}

// Save writes the config to config.json.
func Save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("config.json", data, 0600)
}

// GetActiveEndpoint returns the currently active endpoint.
func (c Config) GetActiveEndpoint() Endpoint {
	for _, ep := range c.Endpoints {
		if ep.Name == c.ActiveEndpoint {
			return ep
		}
	}
	if len(c.Endpoints) > 0 {
		return c.Endpoints[0]
	}
	return Endpoint{}
}
