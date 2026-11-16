package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"secret_santa/config"
	"secret_santa/handlers"
	"secret_santa/storage"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

func main() {
	cfg := config.LoadConfig()

	store, err := storage.NewStore(cfg.DataPath, cfg.AdminKey)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	event := store.GetEvent()

	// If no invite codes exist yet, generate 5 initial ones for convenience
	if len(store.ListInviteCodes()) == 0 {
		_, _ = store.GenerateInviteCodes(5)
	}

	// Prepare embedded static filesystem
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("Failed to load static files: %v", err)
	}

	server, err := handlers.NewServer(cfg, store, templateFS, http.FS(staticSub))
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server startup banner
	fmt.Println("==================================================================")
	fmt.Println("🎅 SECRET SANTA APPLICATION")
	fmt.Println("==================================================================")
	fmt.Printf(" Server running at:     %s\n", cfg.BaseURL)
	fmt.Printf(" Data stored at:        %s\n", cfg.DataPath)
	fmt.Printf(" Admin Passkey:         %s\n", event.AdminKey)
	fmt.Printf(" Direct Admin Link:     %s/admin?key=%s\n", cfg.BaseURL, event.AdminKey)
	if cfg.HasSMTP() {
		fmt.Printf(" Email Notifications:   Enabled (SMTP: %s:%d)\n", cfg.SMTPHost, cfg.SMTPPort)
	} else {
		fmt.Println(" Email Notifications:   Mock/Console mode (SMTP not configured)")
	}
	fmt.Println("==================================================================")

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	fmt.Println("Server stopped. Goodbye! 🎄")
}
