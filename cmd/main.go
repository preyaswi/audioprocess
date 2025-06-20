package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "audio-processor/pkg/api"
    "audio-processor/pkg/config"
    "audio-processor/pkg/pipeline"
    "audio-processor/pkg/storage"

    "github.com/gorilla/mux"
)

func main() {
    // Load configuration
    cfg := config.Load()

    // Initialize storage
    memStore := storage.NewMemoryStore()
    diskStore, err := storage.NewDiskStore(cfg.StoragePath)
    if err != nil {
        log.Fatalf("Failed to initialize disk storage: %v", err)
    }
    defer diskStore.Close()

    // Initialize pipeline
    pipelineManager := pipeline.NewManager(cfg.Pipeline, memStore, diskStore)
    
    // Start pipeline workers
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    if err := pipelineManager.Start(ctx); err != nil {
        log.Fatalf("Failed to start pipeline: %v", err)
    }

    // Initialize API handlers
    handlers := api.NewHandlers(pipelineManager, memStore)

    // Setup routes
    router := mux.NewRouter()
    router.HandleFunc("/upload", handlers.UploadHandler).Methods("POST")
    router.HandleFunc("/chunks/{id}", handlers.GetChunkHandler).Methods("GET")
    router.HandleFunc("/sessions/{user_id}", handlers.GetSessionChunksHandler).Methods("GET")
    router.HandleFunc("/ws", handlers.WebSocketHandler)

    // Start HTTP server
    srv := &http.Server{
        Addr:         cfg.Server.Address,
        Handler:      router,
        ReadTimeout:  cfg.Server.ReadTimeout,
        WriteTimeout: cfg.Server.WriteTimeout,
    }

    go func() {
        log.Printf("Server starting on %s", cfg.Server.Address)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed to start: %v", err)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down server...")

    // Graceful shutdown
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer shutdownCancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }

    log.Println("Server exited")
}