package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mylab-api-go/internal/config"
	"mylab-api-go/internal/db"
	"mylab-api-go/internal/routes"
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
