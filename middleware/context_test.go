package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppCancelled(t *testing.T) {
	processingRequest := make(chan struct{})
	responseCancelled := 0
	handler := func(response http.ResponseWriter, request *http.Request) {
		processingRequest <- struct{}{}
		select {
		case <-time.After(time.Duration(1) * time.Millisecond):
		case <-request.Context().Done():
			responseCancelled++
		}
	}

	appContext, shutdownApp := context.WithCancel(context.Background())
	testHttpServer := setupHttpServerWithContext(handler, appContext)
	defer testHttpServer.Close()

	responseReceived := make(chan struct{})
	go func() {
		response, err := http.Get(testHttpServer.URL)
		require.Nil(t, err, "Request failed: %s\n", err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, 1, responseCancelled)
		responseReceived <- struct{}{}
	}()

	<-processingRequest
	shutdownApp()
	<-responseReceived
}

func TestAppNotCancelled(t *testing.T) {
	responseCancelled := 0
	handler := func(response http.ResponseWriter, request *http.Request) {
		select {
		case <-time.After(time.Duration(1) * time.Millisecond):
		case <-request.Context().Done():
			responseCancelled++
		}
	}

	testHttpServer := setupHttpServerWithContext(handler, context.Background())
	defer testHttpServer.Close()

	response, err := http.Get(testHttpServer.URL)
	require.Nil(t, err, "Request failed: %s\n", err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, 0, responseCancelled)
}

func setupHttpServerWithContext(handler http.HandlerFunc, appContext context.Context) *httptest.Server {
	handlerWithTimeout := Context(handler, appContext)
	return httptest.NewServer(handlerWithTimeout)
}
