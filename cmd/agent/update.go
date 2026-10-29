package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

var updateVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$`)
var updateIDPattern = regexp.MustCompile(`^[a-f0-9-]{36}$`)

type updateTask struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

func updateCommand(cfg Config, task updateTask) (string, []string, error) {
	if !validEndpoint(cfg.Endpoint) || !updateIDPattern.MatchString(task.ID) || !updateVersionPattern.MatchString(task.Version) {
		return "", nil, fmt.Errorf("invalid fixed update request")
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(os.Getenv("ProgramFiles"), "SSHGuardAgent", "update-agent.ps1"), "-Endpoint", cfg.Endpoint, "-Update"}, nil
	}
	return "/bin/sh", []string{"/usr/local/lib/sshguard/update-agent.sh", cfg.Endpoint, "--update"}, nil
}

// Invoked by a privileged local scheduler, never by the collector. The platform
// can only request the fixed locally-installed updater; it cannot supply code.
func pollUpdate(ctx context.Context, cfg Config) error {
	var response struct {
		Data struct {
			Update *updateTask `json:"update"`
		} `json:"data"`
	}
	if e := request(ctx, cfg.Endpoint, "/api/v1/agent/update/poll", cfg.Token, struct{}{}, &response); e != nil {
		return e
	}
	task := response.Data.Update
	if task == nil {
		return nil
	}
	exe, args, e := updateCommand(cfg, *task)
	if e != nil {
		return e
	}
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	command := exec.CommandContext(runCtx, exe, args...)
	command.Env = append(os.Environ(), "SSHGUARD_EXPECTED_VERSION="+task.Version)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	e = command.Run()
	status := "success"
	if e != nil {
		status = "failed"
	}
	reportCtx, done := context.WithTimeout(context.Background(), 45*time.Second)
	defer done()
	reportErr := request(reportCtx, cfg.Endpoint, "/api/v1/agent/update/result", cfg.Token, map[string]string{"id": task.ID, "status": status}, nil)
	if e != nil {
		return fmt.Errorf("fixed Agent update failed; inspect updater logs")
	}
	return reportErr
}
