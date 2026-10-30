package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileCollectorCursorRotationAndPartialLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.log")
	write := func(path, text string) {
		t.Helper()
		f, e := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.WriteString(text); e != nil {
			t.Fatal(e)
		}
		f.Close()
	}
	line := time.Now().Format(time.RFC3339) + " host sshd[1]: Accepted publickey for root from 192.0.2.1 port 2222 ssh2"
	write(path, line+"\n") // Existing history is not replayed on first enrollment.
	s, e := openSpool(filepath.Join(dir, "state"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.db.Close()
	poll := func() {
		t.Helper()
		if _, e := collectFile(context.Background(), s, path); e != nil {
			t.Fatal(e)
		}
	}
	poll()
	if q, _ := s.counts(); q != 0 {
		t.Fatal("initial history replayed")
	}
	write(path, line)
	poll()
	if q, _ := s.counts(); q != 0 {
		t.Fatal("partial record committed")
	}
	write(path, "\n")
	poll()
	poll()
	if q, _ := s.counts(); q != 1 {
		t.Fatal("cursor replay/loss", q)
	}
	old := path + ".1"
	if e = os.Rename(path, old); e != nil {
		t.Fatal(e)
	}
	write(old, line+"\n")
	write(path, line+"\n")
	poll() // Finish the renamed file.
	poll() // Then read the new file from its beginning.
	if q, _ := s.counts(); q != 3 {
		t.Fatal("rotation lost events", q)
	}
	items, e := s.batch()
	if e != nil {
		t.Fatal(e)
	}
	ids := map[string]bool{}
	for _, item := range items {
		if ids[item.event.ID] {
			t.Fatal("different log records share ID")
		}
		ids[item.event.ID] = true
	}
}

func TestLogTimestampYearRollover(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 1, 0, time.Local)
	at, ok := logTimestamp("Dec 31 23:59:59 host sshd: test", now)
	if !ok || at.Year() != 2025 {
		t.Fatal(at, ok)
	}
}
