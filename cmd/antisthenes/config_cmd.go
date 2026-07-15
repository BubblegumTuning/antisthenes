package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/nanami/antisthenes/config"
)

func configureModel() {
	reader := bufio.NewReader(os.Stdin)
	cfg := config.Load()
	if cfg.DebugLogging {
		_ = os.MkdirAll("log", 0o700)
	}

	fmt.Println("=== Configure Model Endpoint ===")
	fmt.Printf("Active endpoint: %s\n", cfg.ActiveEndpoint)
	for _, ep := range cfg.Endpoints {
		keyStatus := "no key"
		if ep.APIKey != "" {
			keyStatus = "has key"
		}
		fmt.Printf("  - %s (%s) — %s\n", ep.Name, ep.Model, keyStatus)
	}
	fmt.Println()

	fmt.Print("Switch to endpoint [local / xai] (leave blank to edit current): ")
	choice, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input:", err)
		return
	}
	choice = strings.TrimSpace(choice)

	var activeEp *config.Endpoint
	for i := range cfg.Endpoints {
		if cfg.Endpoints[i].Name == cfg.ActiveEndpoint {
			activeEp = &cfg.Endpoints[i]
			break
		}
	}
	if activeEp == nil && len(cfg.Endpoints) > 0 {
		activeEp = &cfg.Endpoints[0]
	}

	if choice == "local" || choice == "xai" {
		oldKey := ""
		if activeEp != nil {
			oldKey = activeEp.APIKey
		}

		cfg.ActiveEndpoint = choice

		for i := range cfg.Endpoints {
			if cfg.Endpoints[i].Name == choice {
				activeEp = &cfg.Endpoints[i]
				break
			}
		}

		if activeEp.APIKey == "" && oldKey != "" {
			fmt.Print("Keep previous API key / credentials? [Y/n]: ")
			keep, _ := reader.ReadString('\n')
			keep = strings.TrimSpace(strings.ToLower(keep))
			if keep != "n" && keep != "no" {
				activeEp.APIKey = oldKey
				fmt.Println("Previous credentials kept.")
			}
		}
	}

	// Allow changing agent name
	fmt.Printf("Current agent name: %s\n", cfg.AgentName)
	fmt.Print("New agent name (blank = keep): ")
	newName, _ := reader.ReadString('\n')
	newName = strings.TrimSpace(newName)
	if newName != "" {
		cfg.AgentName = newName
	}

	if activeEp != nil {
		fmt.Printf("\nEditing endpoint: %s\n", activeEp.Name)
		fmt.Printf("Current model: %s\n", activeEp.Model)
		fmt.Print("New model (blank = keep): ")
		newModel, _ := reader.ReadString('\n')
		newModel = strings.TrimSpace(newModel)
		if newModel != "" {
			activeEp.Model = newModel
		}

		fmt.Printf("Current Base URL: %s\n", activeEp.BaseURL)
		fmt.Print("New Base URL (blank = keep): ")
		newURL, _ := reader.ReadString('\n')
		newURL = strings.TrimSpace(newURL)
		if newURL != "" {
			activeEp.BaseURL = newURL
		}

		fmt.Print("API Key (blank = keep current): ")
		newKey, _ := reader.ReadString('\n')
		newKey = strings.TrimSpace(newKey)
		if newKey != "" {
			activeEp.APIKey = newKey
		}
	}

	if err := config.Save(cfg); err != nil {
		fmt.Println("Error saving config:", err)
		return
	}

	fmt.Println("\nEndpoint configuration saved.")
}
