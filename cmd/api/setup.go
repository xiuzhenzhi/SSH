package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"sshguard/internal/api"
	"time"
)

func serveSetup(ctx context.Context, c api.Config) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(c.SetupFile), 0700); err != nil {
		return false, err
	}
	setup := api.NewSetup(c)
	server := &http.Server{Addr: c.Listen, Handler: setup.Router(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	errors := make(chan error, 1)
	go func() {
		if c.TLSCertFile != "" {
			errors <- server.ListenAndServeTLS(c.TLSCertFile, c.TLSKeyFile)
		} else {
			errors <- server.ListenAndServe()
		}
	}()
	completed := false
	select {
	case err := <-errors:
		return false, err
	case <-ctx.Done():
	case <-setup.Done():
		completed = true
	}
	stop, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return completed, server.Shutdown(stop)
}
