package agent

import (
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestSpoolResetsForNewIdentity(t *testing.T) {
	for _, name := range []string{"server", "platform", "registration", "legacy-registration", "legacy"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			s, err := openSpool(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if s != nil {
					s.db.Close()
				}
			}()
			cfg := Config{Endpoint: "https://guard.example.com", ServerID: "original-server", EnrollmentID: "original-registration"}
			if name == "legacy-registration" {
				cfg.EnrollmentID = ""
			}
			if name != "legacy" {
				if err = s.bindIdentity(cfg); err != nil {
					t.Fatal(err)
				}
			}
			oldCursor := cursor{Kind: "file", Position: "old", Offset: 123, At: time.Now().UTC()}
			if err = s.append([]SSHEvent{{ID: "old-rejected"}, {ID: "old-queued"}}, oldCursor); err != nil {
				t.Fatal(err)
			}
			items, err := s.batch()
			if err != nil {
				t.Fatal(err)
			}
			if err = s.acknowledge(items[:1], true); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "server":
				cfg.ServerID = "another-server"
			case "platform":
				cfg.Endpoint = "https://another.example.com"
			case "registration", "legacy-registration":
				cfg.EnrollmentID = "new-registration"
			}
			if err = s.bindIdentity(cfg); err != nil {
				t.Fatal("new identity did not recover old state", err)
			}
			if q, rejected := s.counts(); q != 0 || rejected != 0 {
				t.Fatal("old events survived identity reset", q, rejected)
			}
			if position, err := s.position(); err != nil || position != (cursor{}) {
				t.Fatal("old collector position survived identity reset", position, err)
			}
			newCursor := cursor{Position: "new", Offset: 456}
			if err = s.append([]SSHEvent{{ID: "new-event"}}, newCursor); err != nil {
				t.Fatal(err)
			}
			if err = s.db.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = openSpool(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.bindIdentity(cfg); err != nil {
				t.Fatal(err)
			}
			items, err = s.batch()
			if err != nil || len(items) != 1 || items[0].event.ID != "new-event" {
				t.Fatal("restart discarded new events or transferred old events", items, err)
			}
			if position, err := s.position(); err != nil || position != newCursor {
				t.Fatal("new collector position did not survive restart", position, err)
			}
		})
	}
}

func TestLegacyRegistrationPreservesOwnedState(t *testing.T) {
	s, err := openSpool(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	cfg := Config{Endpoint: "https://guard.example.com", ServerID: "same-server"}
	if err = s.bindIdentity(cfg); err != nil {
		t.Fatal(err)
	}
	position := cursor{Position: "saved"}
	if err = s.append([]SSHEvent{{ID: "pending"}}, position); err != nil {
		t.Fatal(err)
	}
	if err = s.bindIdentity(cfg); err != nil {
		t.Fatal("upgrading a config without a registration marker failed", err)
	}
	if q, _ := s.counts(); q != 1 {
		t.Fatal("legacy config lost its owned queue")
	}
	if err = s.bindIdentity(Config{}); err == nil {
		t.Fatal("accepted an empty identity")
	}
	if q, _ := s.counts(); q != 1 {
		t.Fatal("invalid config cleared existing state")
	}
	if current, err := s.position(); err != nil || current != position {
		t.Fatal("existing position lost", current, err)
	}
}

func TestUnboundLegacyCursorAndRejectedEventsAreReset(t *testing.T) {
	for _, bucket := range []string{"meta", "rejected"} {
		t.Run(bucket, func(t *testing.T) {
			s, err := openSpool(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.db.Close()
			err = s.db.Update(func(tx *bolt.Tx) error {
				return tx.Bucket([]byte(bucket)).Put([]byte("cursor"), []byte("old state"))
			})
			if err != nil {
				t.Fatal(err)
			}
			if err = s.bindIdentity(Config{Endpoint: "https://guard.example.com", ServerID: "new-server"}); err != nil {
				t.Fatal(err)
			}
			if _, rejected := s.counts(); rejected != 0 {
				t.Fatal("unowned rejected events retained")
			}
			if position, err := s.position(); err != nil || position != (cursor{}) {
				t.Fatal("unowned cursor retained", position, err)
			}
		})
	}
}
