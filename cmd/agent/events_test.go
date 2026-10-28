package main

import (
	"errors"
	"testing"
	"time"
)

func TestSSHParsing(t *testing.T) {
	for _, tc := range []struct{ line, user, ip, outcome string }{
		{"sshd[123]: Accepted publickey for root from 2001:db8::1 port 12345 ssh2: RSA SHA256:xxx", "root", "2001:db8::1", "success"},
		{"Failed password for invalid user guest from 192.0.2.1 port 22 ssh2", "guest", "192.0.2.1", "failure"},
		{"sshd: Accepted password for DOMAIN\\John Doe from 192.0.2.2 port 2222 ssh2", "DOMAIN\\John Doe", "192.0.2.2", "success"},
	} {
		e, ok := parseSSH(tc.line, "stable-cursor", time.Now())
		if !ok || e.Username != tc.user || e.IP != tc.ip || e.Outcome != tc.outcome {
			t.Fatalf("parse failed: %+v", e)
		}
		again, _ := parseSSH(tc.line, "stable-cursor", time.Now())
		if e.ID != again.ID {
			t.Fatal("unstable event ID")
		}
	}
	for _, line := range []string{"Invalid user guest from 192.0.2.1 port 22", "Accepted password for root from 999.1.1.1 port 22 ssh2", "Failed password for root from 1.1.1.1 port 99999 ssh2"} {
		if _, ok := parseSSH(line, "x", time.Now()); ok {
			t.Fatal("invalid or duplicate-prone log parsed", line)
		}
	}
}
func TestSpoolRestartAndAtomicCursor(t *testing.T) {
	dir := t.TempDir()
	s, e := openSpool(dir)
	if e != nil {
		t.Fatal(e)
	}
	event, _ := parseSSH("Accepted password for root from 192.0.2.1 port 22 ssh2", "line:1", time.Now())
	if e = s.append([]SSHEvent{event}, cursor{Position: "one"}); e != nil {
		t.Fatal(e)
	}
	s.db.Close()
	s, e = openSpool(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.db.Close()
	items, e := s.batch()
	if e != nil || len(items) != 1 || items[0].event.ID != event.ID {
		t.Fatal("queue lost across restart", e)
	}
	tooMany := make([]SSHEvent, queueLimit)
	if e = s.append(tooMany, cursor{Position: "two"}); !errors.Is(e, errQueueFull) {
		t.Fatal(e)
	}
	c, _ := s.position()
	if c.Position != "one" {
		t.Fatal("cursor advanced despite full queue")
	}
	if e = s.acknowledge(items, false); e != nil {
		t.Fatal(e)
	}
	n, _ := s.counts()
	if n != 0 {
		t.Fatal("acknowledged event retained")
	}
}
