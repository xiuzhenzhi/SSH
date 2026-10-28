package main

import (
	"context"
	"golang.org/x/sys/windows/svc"
	"testing"
	"time"
)

func TestWindowsServiceStopAndShutdown(t *testing.T) {
	for _, command := range []svc.Cmd{svc.Stop, svc.Shutdown} {
		handler := serviceHandler{run: func(ctx context.Context) { <-ctx.Done() }}
		changes := make(chan svc.ChangeRequest)
		states := make(chan svc.Status, 8)
		done := make(chan struct{})
		go func() {
			specific, code := handler.Execute(nil, changes, states)
			if specific || code != 0 {
				t.Error("unexpected service exit")
			}
			close(done)
		}()
		if s := <-states; s.State != svc.StartPending {
			t.Fatal("missing start pending")
		}
		running := <-states
		if running.State != svc.Running || running.Accepts&(svc.AcceptStop|svc.AcceptShutdown) != (svc.AcceptStop|svc.AcceptShutdown) {
			t.Fatal("invalid service capabilities")
		}
		changes <- svc.ChangeRequest{Cmd: svc.Interrogate}
		if s := <-states; s.State != svc.Running {
			t.Fatal("interrogate failed")
		}
		changes <- svc.ChangeRequest{Cmd: command}
		if s := <-states; s.State != svc.StopPending {
			t.Fatal("missing stop pending")
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Windows SCM stop hung")
		}
	}
}
