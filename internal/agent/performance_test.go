package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkParseSSH(b *testing.B) {
	for _, sample := range []struct{ name, line string }{
		{"login", "sshd[123]: Accepted publickey for DOMAIN\\John Doe from 2001:db8::1 port 2222 ssh2: RSA SHA256:xxx"},
		{"failure", "sshd[123]: Failed password for invalid user admin from 192.0.2.1 port 54321 ssh2"},
		{"other", "sshd[123]: Connection closed by authenticating user root 192.0.2.1 port 54321 [preauth]"},
	} {
		b.Run(sample.name, func(b *testing.B) {
			at := time.Now().UTC()
			b.ReportAllocs()
			for b.Loop() {
				_, _ = parseSSH(sample.line, "stable-source-cursor", at)
			}
		})
	}
}

func BenchmarkSpoolCounts(b *testing.B) {
	s, err := openSpool(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer s.db.Close()
	if err = s.append(make([]SSHEvent, queueLimit), cursor{Position: "full"}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = s.counts()
	}
}

func BenchmarkSpoolPosition(b *testing.B) {
	s, err := openSpool(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer s.db.Close()
	if err = s.append(nil, cursor{Kind: "file:auth.log", FileID: "file-identity", Offset: 12345, At: time.Now().UTC()}); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = s.position()
	}
}

func BenchmarkIdleFilePoll(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "auth.log")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		b.Fatal(err)
	}
	s, err := openSpool(filepath.Join(dir, "state"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.db.Close()
	ctx := context.Background()
	if _, err = collectFile(ctx, s, path); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err = collectFile(ctx, s, path); err != nil {
			b.Fatal(err)
		}
	}
}
