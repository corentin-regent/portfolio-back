package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
)

func run(appContext context.Context, getEnv func(string) string) {
	shutdownWaitGroup := &sync.WaitGroup{}
	handler := NewHandler(appContext, shutdownWaitGroup, getEnv)
	server := &http.Server{
		Addr:    ":" + getEnv("HTTP_SERVER_PORT"),
		Handler: handler,
	}
	go serve(server)
	waitUntilShutdown(server, shutdownWaitGroup)
}

func serve(server *http.Server) {
	log.Println("[INFO] HTTP server listening")
	err := server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		log.Printf("[ERROR] HTTP server crashed: %s\n", err)
	}
}

func waitUntilShutdown(server *http.Server, shutdownWaitGroup *sync.WaitGroup) {
	shutdownWaitGroup.Wait()
	log.Println("[INFO] Shutting down HTTP server")
	err := server.Shutdown(context.Background())
	if err != nil {
		log.Printf("[ERROR] HTTP server shutdown failed: %s\n", err)
	}
}

func main() {
	appContext, _ := signal.NotifyContext(context.Background(), os.Interrupt)
	run(appContext, os.Getenv)
}
