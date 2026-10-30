package server

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sshguard/internal/api"
	"sshguard/internal/storage"
	"syscall"
	"time"
)

// Run starts the central server command.
func Run() {
	check := flag.Bool("check", false, "check config, migrate database, bootstrap admin, then exit")
	flag.Parse()
	c, e := api.LoadConfig()
	if e != nil {
		log.Fatal(e)
	}
	if c.TLSCertFile != "" {
		if _, e := tls.LoadX509KeyPair(c.TLSCertFile, c.TLSKeyFile); e != nil {
			log.Fatal("Cannot load configured TLS certificate and key: ", e)
		}
	}
	if c.SetupPending {
		if *check {
			log.Print("Web initialization ready; database and administrator will be configured in the browser")
			return
		}
		setupCtx, stopSetup := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		completed, err := serveSetup(setupCtx, c)
		stopSetup()
		if err != nil {
			log.Fatal("Initialization service failed: ", err)
		}
		if !completed {
			return
		}
		c, e = api.LoadConfig()
		if e != nil {
			log.Fatal(e)
		}
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
	if *check {
		log.Print("Configuration and database ready")
		return
	}
	app := api.NewApp(db, c)
	s := &http.Server{Addr: c.Listen, Handler: app.Router(), TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	waitWorkers := app.StartWorkers(stop)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			ctx, cancel := context.WithTimeout(stop, 30*time.Second)
			if e := storage.Cleanup(ctx, db); e != nil && stop.Err() == nil {
				log.Print("Authentication metadata cleanup failed")
			}
			cancel()
			select {
			case <-stop.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	go func() {
		log.Printf("SSHGuard API listening on %s", c.Listen)
		var e error
		if c.TLSCertFile != "" {
			e = s.ListenAndServeTLS(c.TLSCertFile, c.TLSKeyFile)
		} else {
			e = s.ListenAndServe()
		}
		if e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	<-stop.Done()
	app.Shutdown()
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.Shutdown(ctx)
	waitWorkers()
}
