package main

import (
    "context"
    "errors"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    cfg, err := LoadConfig()
    if err != nil {
        if errors.Is(err, flag.ErrHelp) {
            os.Exit(0)
        }
        log.Fatalf("failed to load config: %v", err)
    }

    srv := NewServer(cfg)
    localIP := GetLocalIP()

    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

    go func() {
        fmt.Println("server is running on", localIP)
        if err:= srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Fatalf("failed to start server: %v", err)
        }
    }()

    <-stop
    fmt.Println("server is shutting down")

    ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("error when server shutting down: %v", err)
    }

    fmt.Println("Server shutdown successfully")
}
