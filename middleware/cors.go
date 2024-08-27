package middleware

import (
	"net/http"
	"strings"

	"github.com/rs/cors"
)

func Cors(handler http.Handler, getEnv func(string) string) http.Handler {
	corsConfig := cors.Options{
		AllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS"), " "),
		AllowedMethods: []string{http.MethodPost},
	}
	return cors.New(corsConfig).Handler(handler)
}
