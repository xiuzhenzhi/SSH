package main

import (
	"strings"
	"testing"
)

func TestFixedUpdateCommand(t *testing.T) {
	cfg := Config{Endpoint: "https://guard.example.com"}
	task := updateTask{ID: "12345678-1234-1234-1234-123456789abc", Version: "0.3.2"}
	executable, args, err := updateCommand(cfg, task)
	if err != nil || executable == "" {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "update-agent.") {
		t.Fatal("not a fixed local updater")
	}
	for _, invalid := range []string{";whoami", "$(id)", "https://evil.test/payload", "../payload", "0.3.2\ncommand"} {
		task.Version = invalid
		if _, _, err := updateCommand(cfg, task); err == nil {
			t.Fatal("accepted invalid version", invalid)
		}
	}
	task.Version = "0.3.2"
	cfg.Endpoint = "http://guard.example.com"
	if _, _, err := updateCommand(cfg, task); err == nil {
		t.Fatal("HTTP update allowed")
	}
}
