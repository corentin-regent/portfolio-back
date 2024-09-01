package middleware

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"
)

func Timeout(handler http.Handler, getEnv func(string) string) http.Handler {
	timeoutInMilliseconds, err := strconv.Atoi(getEnv("TIMEOUT_REQUEST_PROCESSING"))
	if err != nil {
		log.Printf("[ERROR] Invalid request processing timeout: %s\n", err)
		return handler
	}
	timeout := time.Duration(timeoutInMilliseconds) * time.Millisecond

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		contextWithTimeout, freeContext := context.WithTimeout(request.Context(), timeout)
		request = request.WithContext(contextWithTimeout)
		handler.ServeHTTP(response, request)
		freeContext()
	})
}
