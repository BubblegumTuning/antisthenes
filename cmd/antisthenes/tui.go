package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
	"github.com/nanami/antisthenes/internal/cron"
	"github.com/nanami/antisthenes/internal/gateway"
	telegramgw "github.com/nanami/antisthenes/internal/gateway/telegram"
	"github.com/nanami/antisthenes/internal/memory"
	"github.com/nanami/antisthenes/internal/skills"
	"github.com/nanami/antisthenes/internal/tui"
)

func runTUI(cfg config.Config) {
	fmt.Printf("Antisthenes %s - Minimal AI Agent (TUI)\n", version)
	fmt.Println("Delegate agents available via delegate_task tool")

	idx, err := skills.NewSkillIndex(".")
	if err != nil {
		fmt.Println("Error loading skills:", err)
		os.Exit(1)
	}
	fmt.Println("Skills loaded:", len(idx.List()))

	store, err := memory.NewStore(cfg.DBPath)
	if err != nil {
		fmt.Println("Error opening store:", err)
		os.Exit(1)
	}
	defer store.Close()
	sessions, err := store.ListSessions(1)
	if err != nil {
		fmt.Println("Error listing sessions:", err)
		os.Exit(1)
	}
	var sid string
	if len(sessions) > 0 {
		sid = sessions[0]
		fmt.Println("Resuming session:", sid)
	} else {
		sid, err = store.CreateSession()
		if err != nil {
			fmt.Println("Error creating session:", err)
			os.Exit(1)
		}
		fmt.Println("New session:", sid)
	}

	// Hoist reg/loop early for cron callback (minimal wiring improvement).
	// Use shared helper (introduced in Phase 2 step 1).
	reg, loop := newDefaultRegistryAndLoop("", cfg.GetActiveEndpoint().Model, cfg.GetActiveEndpoint().BaseURL, cfg)

	// Phase 5 per DESIGN-TUI.md: Route cron/gateway/subagent output EXCLUSIVELY through the model (custom Msgs).
	// Cron disabled by default (tui.cron_enabled=false). No fmt from bg while Program runs. Use p.Send for delivery to Update.
	// Handle mid-stream by processing in Update (sets notification; shown in right slot).
	// Scheduler started conditionally; callback delivers via CronResultMsg.
	telegramReady := cfg.Gateway.TelegramEnabled && cfg.Gateway.TelegramToken != "" && cfg.Gateway.TelegramChatID != ""
	model := tui.NewModel(loop, store, sid, telegramReady)

	var tg *telegramgw.Adapter
	if telegramReady {
		tg = telegramgw.NewAdapter(cfg.Gateway.TelegramToken, cfg.Gateway.TelegramChatID)
		model.SetGatewayReply(func(chatID, text string) error {
			return tg.SendMessage(context.Background(), chatID, text)
		})
	}

	p := tea.NewProgram(&model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	model.SetNotify(func(msg tea.Msg) { p.Send(msg) })
	// basic mouse enabled for viewport scroll (wheel/click) per DESIGN-TUI.md phase 4

	var sched *cron.Scheduler
	if cfg.CronEnabled {
		sched = cron.NewScheduler(store)
		agent.RegisterCronTools(reg, sched)
		sched.RegisterAgent(func(command string) {
			go func(cmd string) {
				ctx := context.Background()
				msgs := []openai.ChatCompletionMessage{{Role: "user", Content: cmd}}
				final, err := loop.RunWithTools(ctx, msgs)
				var text string
				if err != nil {
					text = fmt.Sprintf("error: %v", err)
				} else {
					for j := len(final) - 1; j >= 0; j-- {
						if final[j].Role == "assistant" && final[j].Content != "" {
							text = strings.TrimSpace(final[j].Content)
							break
						}
					}
					if text == "" {
						text = "no assistant response"
					}
				}
				p.Send(tui.CronResultMsg{Text: "Cron: " + text})
			}(command)
		})
		defer sched.Stop()
		fmt.Println("Cron scheduler started (30s ticker; results routed via TUI model only).")
	} else {
		fmt.Println("Cron disabled (tui.cron_enabled = false per DESIGN-TUI.md). Right status slot reserved for notifications.")
	}

	gwCtx, gwCancel := context.WithCancel(context.Background())
	defer gwCancel()

	if cfg.Gateway.TelegramEnabled {
		if cfg.Gateway.TelegramToken == "" || cfg.Gateway.TelegramChatID == "" {
			fmt.Println("Gateway: telegram_enabled but telegram_token/telegram_chat_id missing; adapter not started.")
		} else {
			gw := gateway.NewGateway(nil)
			gw.RegisterAdapter(tg)
			opts := gateway.BridgeOptions{
				Notify: func(text string) {
					p.Send(tui.GatewayMsg{Text: text})
				},
				OnInbound: func(event gateway.MessageEvent) {
					p.Send(tui.GatewayInboundMsg{
						Platform: event.Platform,
						ChatID:   event.ChatID,
						UserID:   event.UserID,
						Text:     event.Text,
					})
				},
			}
			if err := gw.StartBridge(gwCtx, loop, opts); err != nil {
				fmt.Println("Gateway: failed to start telegram adapter:", err)
			} else {
				defer gw.StopAllAdapters()
				fmt.Println("Gateway: telegram adapter started (window 2; Alt+2 to switch).")
			}
		}
	} else {
		fmt.Println("Gateway disabled (gateway.telegram_enabled = false). Enable in config.json to use Telegram.")
	}

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running TUI:", err)
		os.Exit(1)
	}
}
