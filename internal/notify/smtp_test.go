package notify

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSMTPTLSAndSTARTTLS(t *testing.T) {
	certServer := httptest.NewTLSServer(nil)
	certificate := certServer.TLS.Certificates[0]
	pool := x509.NewCertPool()
	pool.AddCert(certServer.Certificate())
	certServer.Close()
	for _, mode := range []string{"tls", "starttls", "plaintext"} {
		t.Run(mode, func(t *testing.T) {
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer listener.Close()
			config := &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
			if mode == "tls" {
				listener = tls.NewListener(listener, config)
			}
			delivered := make(chan string, 1)
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, e := listener.Accept()
				if e != nil {
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(conn)
				io.WriteString(conn, "220 local.fixture ESMTP\r\n")
				for {
					line, e := reader.ReadString('\n')
					if e != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						if mode == "starttls" {
							io.WriteString(conn, "250-local.fixture\r\n250 STARTTLS\r\n")
						} else {
							io.WriteString(conn, "250 local.fixture\r\n")
						}
					case strings.HasPrefix(line, "HELO"):
						io.WriteString(conn, "250 local.fixture\r\n")
					case strings.HasPrefix(line, "STARTTLS"):
						if mode == "plaintext" {
							io.WriteString(conn, "502 TLS unavailable\r\n")
							return
						}
						io.WriteString(conn, "220 begin TLS\r\n")
						secure := tls.Server(conn, config)
						if secure.Handshake() != nil {
							return
						}
						conn = secure
						reader = bufio.NewReader(conn)
					case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
						io.WriteString(conn, "250 accepted\r\n")
					case strings.HasPrefix(line, "DATA"):
						io.WriteString(conn, "354 enter message\r\n")
						var text strings.Builder
						for {
							line, e = reader.ReadString('\n')
							if e != nil {
								return
							}
							if line == ".\r\n" {
								break
							}
							text.WriteString(line)
						}
						delivered <- text.String()
						io.WriteString(conn, "250 queued\r\n")
						return // Lost QUIT response must not duplicate accepted mail.
					default:
						return
					}
				}
			}()
			host, port, _ := net.SplitHostPort(listener.Addr().String())
			number, _ := strconv.Atoi(port)
			tlsMode := mode
			if mode == "plaintext" {
				tlsMode = "starttls"
			}
			sender := Sender{AllowedHosts: []string{"127.0.0.1"}, RootCAs: pool}
			e = sender.Send(context.Background(), "smtp", Config{Host: host, Port: number, TLSMode: tlsMode, From: "guard@example.com", To: []string{"operator@example.com"}}, Message{ID: "stable-job-id", Title: "登录提醒", Text: "root 登录成功🔒", At: time.Now()})
			if mode == "plaintext" {
				if e == nil {
					t.Fatal("SMTP without STARTTLS accepted")
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				select {
				case text := <-delivered:
					if !strings.Contains(text, "stable-job-id") || !strings.Contains(text, "Content-Transfer-Encoding: base64") {
						t.Fatal("missing stable ID or safe encoding")
					}
					parts := strings.SplitN(text, "\r\n\r\n", 2)
					if len(parts) != 2 {
						t.Fatal("invalid MIME")
					}
					decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(parts[1], "\r\n", ""))
					if err != nil || !strings.Contains(string(decoded), "root 登录成功🔒") {
						t.Fatal("message encoding", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("SMTP DATA not delivered")
				}
			}
			listener.Close()
			<-done
		})
	}
}
