package api

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLinuxCommandsSelectPrivileges(t *testing.T) {
	shell := os.Getenv("SSHGUARD_TEST_SHELL")
	if shell == "" {
		shell = "sh"
	}
	if runtime.GOOS == "windows" {
		if shell == "sh" {
			shell = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "sh.exe")
		}
	}
	for _, uid := range []string{"0", "1000"} {
		for name, commands := range map[string]map[string]string{
			"install":   installCommands("https://guard.example.com", strings.Repeat("a", 64)),
			"update":    updateCommands("https://guard.example.com"),
			"uninstall": uninstallCommands("https://guard.example.com"),
		} {
			t.Run(name+"/uid="+uid, func(t *testing.T) {
				dir := t.TempDir()
				write := func(name, text string) string {
					t.Helper()
					path := filepath.Join(dir, name)
					if err := os.WriteFile(path, []byte("#!/bin/sh\n"+text), 0700); err != nil {
						t.Fatal(err)
					}
					return path
				}
				write("id", "printf '%s\\n' \"$TEST_UID\"\n")
				write("sudo", "printf 'sudo\\n' > \"$TEST_SUDO_LOG\"\nexec \"$@\"\n")
				write("curl", `
[ "${TEST_DOWNLOAD_FAIL:-0}" = 0 ] || exit 23
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then shift; cp "$TEST_INSTALLER" "$1"; exit; fi
  shift
done
exit 2
`)
				write("installer", "printf '%s\\n' \"$@\" > \"$TEST_ARGUMENTS\"\nexit 17\n")
				argsLog, sudoLog := filepath.Join(dir, "arguments"), filepath.Join(dir, "sudo-log")
				run := func(fail string) error {
					cmd := exec.Command(shell, "-c", `fixture=$(CDPATH='' cd "$TEST_ROOT" && pwd -P) || exit
export PATH="$fixture:$PATH" TMPDIR="$fixture"
export TEST_INSTALLER="$fixture/installer" TEST_ARGUMENTS="$fixture/arguments" TEST_SUDO_LOG="$fixture/sudo-log"
`+commands["linux"])
					cmd.Env = append(os.Environ(), "TEST_ROOT="+dir, "TEST_UID="+uid, "TEST_DOWNLOAD_FAIL="+fail)
					return cmd.Run()
				}
				var exitError *exec.ExitError
				if err := run("0"); !errors.As(err, &exitError) || exitError.ExitCode() != 17 {
					t.Fatalf("installer exit status not preserved: %v", err)
				}
				_, sudoErr := os.Stat(sudoLog)
				if (sudoErr == nil) != (uid != "0") {
					t.Fatalf("incorrect privilege selection: %v", sudoErr)
				}
				arguments, err := os.ReadFile(argsLog)
				if err != nil || (name == "install" && !strings.Contains(string(arguments), strings.Repeat("a", 64))) ||
					(name == "update" && !strings.Contains(string(arguments), "--update")) {
					t.Fatalf("installer arguments lost: %s, %v", arguments, err)
				}
				if err := os.Remove(argsLog); err != nil {
					t.Fatal(err)
				}
				if err := run("1"); !errors.As(err, &exitError) || exitError.ExitCode() != 23 {
					t.Fatalf("download failure not preserved: %v", err)
				}
				if _, err := os.Stat(argsLog); !os.IsNotExist(err) {
					t.Fatal("installer executed after a failed download")
				}
				if files, _ := filepath.Glob(filepath.Join(dir, "tmp.*")); len(files) != 0 {
					t.Fatal("temporary installer was not removed", files)
				}
			})
		}
	}
}
