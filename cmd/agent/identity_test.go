package main

import "testing"

func TestQueueCannotMoveToAnotherServer(t *testing.T) {
	s, e := openSpool(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.db.Close()
	cfg := Config{Endpoint: "https://guard.example.com", ServerID: "original-server", Token: "old"}
	if e = s.bindIdentity(cfg); e != nil {
		t.Fatal(e)
	}
	cfg.Token = "rotated"
	if e = s.bindIdentity(cfg); e != nil {
		t.Fatal("credential rotation rejected", e)
	}
	cfg.ServerID = "another-server"
	if e = s.bindIdentity(cfg); e == nil {
		t.Fatal("queue could be attributed to another server")
	}
	cfg.ServerID = "original-server"
	cfg.Endpoint = "https://another.example.com"
	if e = s.bindIdentity(cfg); e == nil {
		t.Fatal("queue could move to another platform")
	}
}
