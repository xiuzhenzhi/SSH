package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func defaultConfigPath() string { return "/etc/sshguard/agent.json" }
func configureService(action, path string) error {
	return fmt.Errorf("use install-agent.sh for systemd or OpenRC service installation")
}
func runPlatform(run func(context.Context)) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	run(ctx)
	return nil
}
