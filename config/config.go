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

// IterativeSettings controls /iterative (and related) worker seed guidance.
// Zero values mean “use defaults” via the accessor methods on Config.
type IterativeSettings struct {
	// ContextRemindPercent: when context usage exceeds this %, write a concise progress summary to the work log (default 55).
	ContextRemindPercent int `json:"context_remind_percent"`
	// ContextSummaryPercent: when context usage exceeds this %, force a fuller summary/refresh before more edits (default 60).
	ContextSummaryPercent int `json:"context_summary_percent"`
	// MaxIterations: hard cap on Execute phases across multi-cycle PER (default 40).
	MaxIterations int `json:"max_iterations"`
}

// Default iterative worker guidance (historical hardcodes from DESIGN.md).
const (
	DefaultIterativeContextRemindPercent  = 55
	DefaultIterativeContextSummaryPercent = 60
	DefaultIterativeMaxIterations         = 40
)

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
	// AuxModels are secondary OpenAI-compatible endpoints for cheap/async work (titles, etc.).
	AuxModels  []AuxModel `json:"aux_models,omitempty"`
	EditHeight int        `json:"edit_height"`
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
	// Phase 7 per DESIGN-TUI.md: input history (Up/Down in edit box); file-backed under WorkDir.
	InputHistoryEnabled bool `json:"input_history_enabled"`
	InputHistorySize    int  `json:"input_history_size"`
	// InputHistoryPath optional override; empty → WorkDir/input_history.json (bash-style file).
	InputHistoryPath string    `json:"input_history_path,omitempty"`
	Colors           TUIColors `json:"colors"`
	// Iterative: /iterative worker context thresholds and max iterations (see IterativeSettings).
	Iterative IterativeSettings `json:"iterative"`
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
		DBPath:     DefaultDBPath(),
		WorkDir:    DefaultWorkDir(),
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
		Iterative: IterativeSettings{
			ContextRemindPercent:  DefaultIterativeContextRemindPercent,
			ContextSummaryPercent: DefaultIterativeContextSummaryPercent,
			MaxIterations:         DefaultIterativeMaxIterations,
		},
	}
}

// IterativeContextRemindPercent returns the remind threshold (1–100), default 55.
func (c Config) IterativeContextRemindPercent() int {
	v := c.Iterative.ContextRemindPercent
	if v <= 0 || v > 100 {
		return DefaultIterativeContextRemindPercent
	}
	return v
}

// IterativeContextSummaryPercent returns the force-summary threshold (1–100), default 60.
// Always at least the remind percent so the seed stays coherent.
func (c Config) IterativeContextSummaryPercent() int {
	remind := c.IterativeContextRemindPercent()
	v := c.Iterative.ContextSummaryPercent
	if v <= 0 || v > 100 {
		v = DefaultIterativeContextSummaryPercent
	}
	if v < remind {
		return remind
	}
	return v
}

// IterativeMaxIterations returns the Execute-phase cap for multi-cycle PER (at least 1), default 40.
func (c Config) IterativeMaxIterations() int {
	v := c.Iterative.MaxIterations
	if v <= 0 {
		return DefaultIterativeMaxIterations
	}
	return v
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

// InputHistoryFile returns the path for persisted Up/Down history.
// Explicit input_history_path wins; otherwise WorkDir/input_history.json.
func (c Config) InputHistoryFile() string {
	if p := strings.TrimSpace(c.InputHistoryPath); p != "" {
		return p
	}
	dir := strings.TrimSpace(c.WorkDir)
	if dir == "" {
		dir = DefaultWorkDir()
	}
	return filepath.Join(dir, "input_history.json")
}

// Load reads config from config.json or returns defaults.
// Path resolution: env (ANTISTHENES_DB_PATH / ANTISTHENES_DB, ANTISTHENES_WORK_DIR,
// ANTISTHENES_DATA_DIR) overrides config.json; empty config paths use durable defaults
// under ~/.antisthenes (see DefaultDataDir).
func Load() Config {
	data, err := os.ReadFile("config.json")
	if err != nil {
		cfg := DefaultConfig()
		applyPathDefaults(&cfg)
		ensureDataDirs(cfg)
		return cfg
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		cfg = DefaultConfig()
		applyPathDefaults(&cfg)
		ensureDataDirs(cfg)
		return cfg
	}
	applyPathDefaults(&cfg)
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
	ensureDataDirs(cfg)
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
