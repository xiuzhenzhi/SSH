package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJournalHelperProcess(t *testing.T) {
	if os.Getenv("SSHGUARD_JOURNAL_HELPER") != "1" {
		return
	}
	chunk := bytes.Repeat([]byte("diagnostic "), 8192)
	for i := 0; i < 128; i++ {
		_, _ = os.Stderr.Write(chunk)
	}
	_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), 70<<10))
	time.Sleep(time.Hour)
}

func TestOversizedJournalStopsProducer(t *testing.T) {
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec \"$SSHGUARD_JOURNAL_TEST_EXE\" -test.run=^TestJournalHelperProcess$ -- \"$@\"\n"
	if err = os.WriteFile(filepath.Join(dir, "journalctl"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SSHGUARD_JOURNAL_HELPER", "1")
	t.Setenv("SSHGUARD_JOURNAL_TEST_EXE", exe)
	s, err := openSpool(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = collectJournal(ctx, s)
	if err == nil || !strings.Contains(err.Error(), "exceeds read limit") || ctx.Err() != nil {
		t.Fatalf("oversized producer was not stopped promptly: %v", err)
	}
}
