package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"html-preview/internal/api"
	"html-preview/internal/config"
	"html-preview/internal/db"
	"html-preview/internal/item"
	"html-preview/internal/lifecycle"
	"html-preview/internal/screenshot"
)

func main() {
	cfg := config.Load()
	lifecycleInterval := lifecycle.ParseInterval(os.Getenv("LIFECYCLE_INTERVAL"))

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	conn, err := db.Open(cfg.DBPath())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer conn.Close()

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("embed web assets: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lc := lifecycle.New(conn, cfg.DataDir)
	go lc.Run(ctx, lifecycleInterval)

	itemsSvc := &item.Service{DB: conn, DataDir: cfg.DataDir}
	ss := screenshot.New(itemsSvc, cfg.ChromePath)
	ss.Start(ctx)

	handler := api.NewRouter(conn, cfg, web, ss)
	addr := cfg.Addr()
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("listening on %s (data: %s, lifecycle: %s)", addr, filepath.Clean(cfg.DataDir), lifecycleInterval)
	log.Printf("local:  http://127.0.0.1:%d/", cfg.Port)
	if lans := cfg.LANURLs(); len(lans) > 0 {
		for _, u := range lans {
			log.Printf("lan:    %s/", u)
		}
	} else {
		log.Printf("lan:    (no non-loopback IPv4 address found; ensure HOST=0.0.0.0)")
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}
