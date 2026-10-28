package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

type SSHEvent struct {
	ID       string    `json:"event_id"`
	Type     string    `json:"event_type"`
	Username string    `json:"username"`
	IP       string    `json:"source_ip"`
	Port     int       `json:"source_port"`
	Method   string    `json:"auth_method"`
	Outcome  string    `json:"outcome"`
	At       time.Time `json:"occurred_at"`
}

var loginPattern = regexp.MustCompile(`(?:^|\s)(Accepted|Failed) ([A-Za-z0-9@._/-]+) for (?:invalid user )?(.+?) from ([0-9A-Fa-f:.]+) port ([0-9]+)(?:\s|$)`)

func parseSSH(message, identity string, at time.Time) (SSHEvent, bool) {
	m := loginPattern.FindStringSubmatch(message)
	if m == nil {
		return SSHEvent{}, false
	}
	ip := net.ParseIP(m[4])
	port, e := strconv.Atoi(m[5])
	if ip == nil || e != nil || port < 1 || port > 65535 || len([]rune(m[3])) > 128 || len(m[2]) > 32 || strings.ContainsAny(m[3], "\r\n\x00") {
		return SSHEvent{}, false
	}
	event := SSHEvent{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(identity)).String(), Username: m[3], IP: ip.String(), Port: port, Method: m[2], At: at.UTC(), Type: "ssh.login", Outcome: "success"}
	if m[1] == "Failed" {
		event.Type = "ssh.login_failed"
		event.Outcome = "failure"
	}
	return event, true
}

const queueLimit = 10000

var errQueueFull = errors.New("local queue is full; collection paused without advancing its cursor")

type cursor struct {
	Kind     string    `json:"kind"`
	Position string    `json:"position"`
	FileID   string    `json:"file_id,omitempty"`
	Offset   int64     `json:"offset,omitempty"`
	At       time.Time `json:"at"`
}
type spool struct{ db *bolt.DB }

func (s *spool) bindIdentity(cfg Config) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte("meta"))
		identity := cfg.Endpoint + "|" + cfg.ServerID
		previous := meta.Get([]byte("identity"))
		if previous != nil && string(previous) != identity {
			return errors.New("state belongs to a different server; archive it before reenrolling this host")
		}
		if previous == nil {
			if tx.Bucket([]byte("queue")).Stats().KeyN > 0 {
				return errors.New("existing queue has no server identity; operator recovery required")
			}
			return meta.Put([]byte("identity"), []byte(identity))
		}
		return nil
	})
}

func openSpool(dir string) (*spool, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	db, e := bolt.Open(filepath.Join(dir, "state.db"), 0600, &bolt.Options{Timeout: time.Second})
	if e != nil {
		return nil, e
	}
	e = db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"queue", "meta", "rejected"} {
			if _, e := tx.CreateBucketIfNotExists([]byte(name)); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		db.Close()
		return nil, e
	}
	return &spool{db}, nil
}
func (s *spool) position() (cursor, error) {
	var c cursor
	e := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("meta")).Get([]byte("cursor"))
		if b == nil {
			return nil
		}
		return json.Unmarshal(b, &c)
	})
	return c, e
}

// Queue entries and source cursor are committed together and fsynced by bbolt.
func (s *spool) append(events []SSHEvent, c cursor) error {
	if len(events) == 0 {
		previous, e := s.position()
		if e != nil {
			return e
		}
		if previous == c {
			return nil
		}
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		q := tx.Bucket([]byte("queue"))
		if q.Stats().KeyN+len(events) > queueLimit {
			return errQueueFull
		}
		for _, event := range events {
			seq, e := q.NextSequence()
			if e != nil {
				return e
			}
			key := make([]byte, 8)
			binary.BigEndian.PutUint64(key, seq)
			b, e := json.Marshal(event)
			if e != nil {
				return e
			}
			if e = q.Put(key, b); e != nil {
				return e
			}
		}
		b, e := json.Marshal(c)
		if e != nil {
			return e
		}
		return tx.Bucket([]byte("meta")).Put([]byte("cursor"), b)
	})
}

type queued struct {
	key   []byte
	event SSHEvent
}

func (s *spool) batch() ([]queued, error) {
	items := []queued{}
	e := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("queue")).Cursor()
		for k, v := c.First(); k != nil && len(items) < 100; k, v = c.Next() {
			var event SSHEvent
			if e := json.Unmarshal(v, &event); e != nil {
				return fmt.Errorf("queue entry corrupt: %w", e)
			}
			items = append(items, queued{append([]byte(nil), k...), event})
		}
		return nil
	})
	return items, e
}
func (s *spool) acknowledge(items []queued, reject bool) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		q := tx.Bucket([]byte("queue"))
		dead := tx.Bucket([]byte("rejected"))
		for _, item := range items {
			if reject {
				if dead.Stats().KeyN >= 1000 {
					return errors.New("rejected queue full; operator intervention required")
				}
				if value := q.Get(item.key); value != nil {
					if e := dead.Put(item.key, value); e != nil {
						return e
					}
				}
			}
			if e := q.Delete(item.key); e != nil {
				return e
			}
		}
		return nil
	})
}
func (s *spool) counts() (int, int) {
	q, d := 0, 0
	_ = s.db.View(func(tx *bolt.Tx) error {
		q = tx.Bucket([]byte("queue")).Stats().KeyN
		d = tx.Bucket([]byte("rejected")).Stats().KeyN
		return nil
	})
	return q, d
}
