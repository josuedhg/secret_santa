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
		log.Fatalf("Error al inicializar la base de datos: %v", err)
	}

	event := store.GetEvent()

	// Si no hay códigos de invitación, genera 5 iniciales
	if len(store.ListInviteCodes()) == 0 {
		_, _ = store.GenerateInviteCodes(5)
	}

	// Cargar archivos estáticos embebidos
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("Error al cargar archivos estáticos: %v", err)
	}

	server, err := handlers.NewServer(cfg, store, templateFS, http.FS(staticSub))
	if err != nil {
		log.Fatalf("Error al inicializar servidor: %v", err)
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

	// Mensaje de bienvenida en consola
	fmt.Println("==================================================================")
	fmt.Println("🎁 APLICACIÓN DE INTERCAMBIO (Sin contraseñas ni correos)")
	fmt.Println("==================================================================")
	fmt.Printf(" Servidor activo en:        %s\n", cfg.BaseURL)
	fmt.Printf(" Datos guardados en:        %s\n", cfg.DataPath)
	fmt.Printf(" Clave del Organizador:     %s\n", event.AdminKey)
	fmt.Printf(" Enlace directo organizador: %s/admin?key=%s\n", cfg.BaseURL, event.AdminKey)
	fmt.Println("==================================================================")

	// Cierre elegante
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error en el servidor: %v", err)
		}
	}()

	<-stop
	fmt.Println("\nDeteniendo el servidor...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error al detener servidor: %v", err)
	}
	fmt.Println("Servidor detenido. ¡Feliz Intercambio! 🎄")
}
