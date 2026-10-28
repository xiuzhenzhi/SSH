package api

import (
	"strings"
	"testing"
)

func TestInstallCommandsPlatforms(t *testing.T) {
	token := strings.Repeat("a", 64)
	commands := installCommands("https://guard.example.com", token)
	for _, os := range []string{"windows", "ubuntu", "debian", "alpine"} {
		if !strings.Contains(commands[os], token) {
			t.Fatalf("%s missing enrollment", os)
		}
	}
	if strings.Contains(commands["alpine"], "sudo") || strings.Contains(commands["alpine"], "bash") {
		t.Fatal("Alpine must not require sudo or bash")
	}
	if !strings.Contains(commands["windows"], "powershell.exe") || !strings.Contains(commands["windows"], "install-agent.ps1") {
		t.Fatal("Windows installer unavailable")
	}
	if powershellQuote("a'b") != "'a''b'" {
		t.Fatal("PowerShell quote escaping")
	}
}
func TestUpdateDoesNotEnroll(t *testing.T) {
	commands := updateCommands("https://guard.example.com")
	for _, platform := range []string{"ubuntu", "debian", "alpine"} {
		if !strings.Contains(commands[platform], "'--update'") {
			t.Fatal("update flag missing", platform)
		}
	}
	if strings.Contains(commands["windows"], "-EnrollmentToken") || !strings.Contains(commands["windows"], "-Update") {
		t.Fatal("Windows update would enroll")
	}
}
