package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTimeout = 2

func TestResponseTimedOut(t *testing.T) {
	responseTimedOut := 0
	handler := func(response http.ResponseWriter, request *http.Request) {
		select {
		case <-time.After(time.Duration(999*testTimeout) * time.Millisecond):
		case <-request.Context().Done():
			responseTimedOut++
		}
	}
	testHttpServer := setupHttpServer(handler)

	response, err := http.Get(testHttpServer.URL)
	require.Nil(t, err, "Request failed: %s\n", err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, 1, responseTimedOut)

	testHttpServer.Close()
}

func TestResponseNotTimedOut(t *testing.T) {
	responseTimedOut := 0
	handler := func(response http.ResponseWriter, request *http.Request) {
		select {
		case <-time.After(time.Duration(testTimeout/2) * time.Millisecond):
		case <-request.Context().Done():
			responseTimedOut++
		}
	}
	testHttpServer := setupHttpServer(handler)

	response, err := http.Get(testHttpServer.URL)
	require.Nil(t, err, "Request failed: %s\n", err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, 0, responseTimedOut)

	testHttpServer.Close()
}

func setupHttpServer(handler http.HandlerFunc) *httptest.Server {
	mockGetEnv := func(key string) string {
		if key == "TIMEOUT_REQUEST_PROCESSING" {
			return strconv.Itoa(testTimeout)
		}
		return ""
	}
	handlerWithTimeout := Timeout(handler, mockGetEnv)
	return httptest.NewServer(handlerWithTimeout)
}
