package agent

import (
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestSpoolCacheMatchesCommittedState(t *testing.T) {
	s, err := openSpool(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	check := func(queued, rejected int) {
		t.Helper()
		if q, r := s.counts(); q != queued || r != rejected {
			t.Fatalf("cached counts = %d/%d, want %d/%d", q, r, queued, rejected)
		}
		if err := s.db.View(func(tx *bolt.Tx) error {
			if q, r := tx.Bucket([]byte("queue")).Stats().KeyN, tx.Bucket([]byte("rejected")).Stats().KeyN; q != queued || r != rejected {
				t.Errorf("durable counts = %d/%d, want %d/%d", q, r, queued, rejected)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.append(make([]SSHEvent, 999), cursor{Position: "initial"}); err != nil {
		t.Fatal(err)
	}
	for q, _ := s.counts(); q > 0; q, _ = s.counts() {
		items, err := s.batch()
		if err != nil {
			t.Fatal(err)
		}
		if err = s.acknowledge(items, true); err != nil {
			t.Fatal(err)
		}
	}
	check(0, 999)
	if err = s.append([]SSHEvent{{ID: "one"}, {ID: "two"}}, cursor{Position: "new"}); err != nil {
		t.Fatal(err)
	}
	items, err := s.batch()
	if err != nil || len(items) != 2 {
		t.Fatal("missing queued events", items, err)
	}
	if err = s.acknowledge(items, true); err == nil {
		t.Fatal("quarantine overflow accepted")
	}
	check(2, 999) // The first tentative quarantine must roll back with the second.
	if err = s.append(make([]SSHEvent, queueLimit), cursor{Position: "must not commit"}); err == nil {
		t.Fatal("queue overflow accepted")
	}
	check(2, 999)
	if position, err := s.position(); err != nil || position.Position != "new" {
		t.Fatal("failed write changed cached position", position, err)
	}
	if err = s.acknowledge(append(items, items...), false); err != nil {
		t.Fatal(err)
	}
	check(0, 999)
}
