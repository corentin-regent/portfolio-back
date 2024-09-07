package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

func run(appContext context.Context, getEnv func(string) string) {
	shutdownWaitGroup := &sync.WaitGroup{}
	handler := NewHandler(appContext, shutdownWaitGroup, getEnv)
	go serve(appContext, handler)
	shutdownWaitGroup.Wait()
}

func serve(appContext context.Context, handler http.Handler) {
	log.Println("[INFO] HTTP server listening")
	lambda.StartWithOptions(
		httpadapter.NewV2(handler).ProxyWithContext,
		lambda.WithContext(appContext),
		lambda.WithEnableSIGTERM(func() {
			log.Println("[INFO] Shutting down HTTP server")
		}),
	)
}

func main() {
	run(context.Background(), os.Getenv)
}
