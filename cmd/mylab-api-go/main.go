package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mylab-api-go/internal/config"
	"mylab-api-go/internal/db"
	"mylab-api-go/internal/routes"
	routesauth "mylab-api-go/internal/routes/auth"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Database optional untuk startup, tapi dibutuhkan untuk endpoint yang akses DB.
	var dbConn *sql.DB
	if cfg.DatabaseURL != "" {
		opened, err := db.Open(cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("database error: %v", err)
		}
		dbConn = opened
		defer func() { _ = dbConn.Close() }()
	}

	// Laravel-like auth session store (server-side state for JWT sessions).
	// Default: file store under storage/sessions.
	// For Docker: mount a volume to persist storage/sessions.
	switch strings.ToLower(strings.TrimSpace(cfg.AuthSessionDriver)) {
	case "", "file":
		store, err := routesauth.NewFileSessionStore(cfg.AuthSessionFiles)
		if err != nil {
			log.Fatalf("auth session store (file) error: %v", err)
		}
		routesauth.SetSessionStore(store)
	case "database", "db", "postgres", "postgresql":
		if dbConn == nil {
			log.Fatalf("auth session store driver=%q requires DATABASE_URL", cfg.AuthSessionDriver)
		}
		store, err := routesauth.NewPostgresSessionStore(dbConn, cfg.AuthSessionTable)
		if err != nil {
			log.Fatalf("auth session store (postgres) error: %v", err)
		}
		routesauth.SetSessionStore(store)
	case "none", "disabled", "off":
		// keep nil store; auth works purely JWT + in-memory token revocation.
	default:
		log.Fatalf("auth session store driver not supported: %q", cfg.AuthSessionDriver)
	}

	srv := routes.New(cfg.HTTPAddr, cfg.LogLevel, dbConn)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("signal received: %s", sig)
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server stopped with error: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
