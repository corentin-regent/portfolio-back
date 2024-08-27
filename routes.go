package main

import (
	"context"
	"net/http"
	"sync"

	"portfolio-back/api"
)

func InstallRoutes(
	serveMux *http.ServeMux,
	appContext context.Context,
	shutdownWaitGroup *sync.WaitGroup,
	getEnv func(string) string,
) {
	serveMux.HandleFunc("/email", api.HandleEmail(appContext, shutdownWaitGroup, getEnv))
}
