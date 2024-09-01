package main

import (
	"context"
	"net/http"
	"sync"

	"portfolio-back/api/email"
)

func InstallRoutes(
	serveMux *http.ServeMux,
	appContext context.Context,
	shutdownWaitGroup *sync.WaitGroup,
	getEnv func(string) string,
) {
	serveMux.HandleFunc("POST /email", email.HandlePostEmail(appContext, shutdownWaitGroup, getEnv))
}
