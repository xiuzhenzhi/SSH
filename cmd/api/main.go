package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sshguard/internal/api"
	"sshguard/internal/storage"
	"syscall"
	"time"
)

func main() {
	c, e := api.LoadConfig()
	if e != nil {
		log.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	db, e := storage.Open(ctx, c.Driver, c.DatabaseURL)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	if e = storage.Migrate(ctx, db); e != nil {
		log.Fatal(e)
	}
	if e = api.Bootstrap(ctx, db, c); e != nil {
		log.Fatal(e)
	}
	cancel()
	s := &http.Server{Addr: c.Listen, Handler: api.New(db, c), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	go func() {
		log.Printf("SSHGuard API listening on %s", c.Listen)
		if e := s.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	<-stop.Done()
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.Shutdown(ctx)
}
