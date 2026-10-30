package agent

import (
	"math/rand/v2"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var previousLoginPattern = regexp.MustCompile(`(?:^|\s)(Accepted|Failed) ([A-Za-z0-9@._/-]+) for (?:invalid user )?(.+?) from ([0-9A-Fa-f:.]+) port ([0-9]+)(?:\s|$)`)

func previousParseSSH(message, identity string, at time.Time) (SSHEvent, bool) {
	m := previousLoginPattern.FindStringSubmatch(message)
	if m == nil {
		return SSHEvent{}, false
	}
	ip := net.ParseIP(m[4])
	port, err := strconv.Atoi(m[5])
	if ip == nil || err != nil || port < 1 || port > 65535 || len([]rune(m[3])) > 128 || len(m[2]) > 32 || strings.ContainsAny(m[3], "\r\n\x00") {
		return SSHEvent{}, false
	}
	event := SSHEvent{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(identity)).String(), Username: m[3], IP: ip.String(), Port: port, Method: m[2], At: at.UTC(), Type: "ssh.login", Outcome: "success"}
	if m[1] == "Failed" {
		event.Type, event.Outcome = "ssh.login_failed", "failure"
	}
	return event, true
}

func checkParserCompatibility(t *testing.T, message string) {
	t.Helper()
	at := time.Unix(1793360000, 0).UTC()
	got, ok := parseSSH(message, "stable-event-identity", at)
	want, matched := previousParseSSH(message, "stable-event-identity", at)
	if ok != matched || got != want {
		t.Fatalf("parser changed for %q: got %+v/%v, want %+v/%v", message, got, ok, want, matched)
	}
}

func TestSSHParserCompatibility(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	prefixes := []string{"", "sshd[123]: ", "prefix\t", "prefix\f", "x", "\n", "\r", "\v"}
	states := []string{"Accepted", "Failed", "failed", "NotAccepted"}
	methods := []string{"publickey", "password", "keyboard-interactive/pam", "gssapi-with-mic", "bad+method", "", strings.Repeat("x", 33)}
	users := []string{"root", "invalid user admin", "invalid user ", "invalid user", "DOMAIN\\John Doe", "用户", "root from somewhere", "root from 192.0.2.4 port 10 x", "\x00root", "root\nother", "root\ruser", strings.Repeat("中", 129), ""}
	ips := []string{"192.0.2.1", "2001:db8::1", "::ffff:192.0.2.1", "999.1.1.1", "01.2.3.4", "invalid", ""}
	ports := []string{"1", "22", "65535", "65536", "0", "-1", "99999999999999999999999", "22x", ""}
	suffixes := []string{"", " ssh2", "\nAccepted password for root from 192.0.2.1 port 22", "\tkey", "x", "\r", "\v"}
	for i := 0; i < 20000; i++ {
		message := prefixes[rng.IntN(len(prefixes))] + states[rng.IntN(len(states))] + " " + methods[rng.IntN(len(methods))] + " for " + users[rng.IntN(len(users))] + " from " + ips[rng.IntN(len(ips))] + " port " + ports[rng.IntN(len(ports))] + suffixes[rng.IntN(len(suffixes))]
		checkParserCompatibility(t, message)
	}
}

func FuzzSSHParserCompatibility(f *testing.F) {
	for _, message := range []string{
		"sshd[1]: Accepted publickey for root from 2001:db8::1 port 2222 ssh2",
		"Failed password for invalid user admin from 192.0.2.1 port 22 ssh2",
		"Accepted password for invalid user  from ::1 port 1",
		"Accepted password for root\nFailed password for user from ::1 port 22",
	} {
		f.Add(message)
	}
	f.Fuzz(func(t *testing.T, message string) {
		if len(message) <= 65536 {
			checkParserCompatibility(t, message)
		}
	})
}

func TestUpdateValidatorCompatibility(t *testing.T) {
	versionPattern := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$`)
	idPattern := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
	for _, value := range []string{"0.3.12", "1.0.0-alpha.1", "01.002.3--", "1.2.3-", "1..3", "1.2.3.4", "1.2.3\n", "1.2.3-中", "", "12345678-1234-1234-1234-123456789abc", strings.Repeat("-", 36), strings.Repeat("a", 36), strings.Repeat("A", 36)} {
		if validUpdateVersion(value) != (len(value) <= 32 && versionPattern.MatchString(value)) || validUpdateID(value) != idPattern.MatchString(value) {
			t.Fatal("update validation changed", value)
		}
	}
}
