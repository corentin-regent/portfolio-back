package main

import (
	"context"
	"net/http"
	"sync"

	"portfolio-back/middleware"
)

func NewHandler(
	appContext context.Context,
	shutdownWaitGroup *sync.WaitGroup,
	getEnv func(string) string,
) http.Handler {
	serveMux := http.NewServeMux()
	InstallRoutes(serveMux, appContext, shutdownWaitGroup, getEnv)
	var handler http.Handler = middleware.Timeout(serveMux, getEnv)
	handler = middleware.Cors(handler, getEnv)
	return handler
}
