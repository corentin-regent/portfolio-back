package middleware

import (
	"context"
	"net/http"
)

func Context(handler http.Handler, appContext context.Context) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		request = request.WithContext(appContext)
		handler.ServeHTTP(response, request)
	})
}
