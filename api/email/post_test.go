package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/mhale/smtpd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const targetEmailAddress = "target@test.com"
const sourceEmailAddress = "source@test.com"
const sourceEmailPassword = "test password"

const emailSender = "Test sender"
const emailSubject = "Test subject"
const emailBody = "Test body"
const successRedirectUrl = "http://localhost/success"

var expectedMessageReceived = fmt.Sprintf(
	"To: %s\r\nSubject: %s\r\n\r\n%s\r\n\r\nSent by %s",
	targetEmailAddress,
	emailSubject,
	emailBody,
	emailSender,
)
var expectedErrorRedirectUrl = fmt.Sprintf(
	"mailto:%s?subject=%s&body=%s",
	targetEmailAddress,
	url.PathEscape(emailSubject),
	url.PathEscape(emailBody),
)

func TestSendEmail(t *testing.T) {
	emailsReceived := 0
	smtpHandler := func(_ net.Addr, from string, to []string, data []byte) error {
		emailsReceived++
		assert.Equal(t, sourceEmailAddress, from)
		assert.Equal(t, []string{targetEmailAddress}, to)
		assert.Contains(t, string(data), expectedMessageReceived)
		return nil
	}

	smtpServer, smtpServerPort := setupSmtpServer(t, smtpHandler, nil)
	testHttpServer, shutdownWaitGroup, triggerShutdown := setupHttpServer(context.Background(), smtpServerPort)

	requestPostEmail(t, testHttpServer.URL)
	assert.Equal(t, emailsReceived, 1)

	teardownHttpServer(testHttpServer, shutdownWaitGroup, triggerShutdown)
	teardownSmtpServer(smtpServer)
}

func TestRedirectIfOk(t *testing.T) {
	smtpServer, smtpServerPort := setupSmtpServer(t, nil, nil)
	testHttpServer, shutdownWaitGroup, triggerShutdown := setupHttpServer(context.Background(), smtpServerPort)

	response := requestPostEmail(t, testHttpServer.URL)
	assert.Equal(t, http.StatusFound, response.StatusCode)
	assert.Equal(t, successRedirectUrl, response.Header.Get("Location"))

	teardownHttpServer(testHttpServer, shutdownWaitGroup, triggerShutdown)
	teardownSmtpServer(smtpServer)
}

func TestRedirectIfErrorInSmtpServer(t *testing.T) {
	smtpHandler := func(_ net.Addr, _ string, _ []string, _ []byte) error {
		return errors.New("Error in SMTP server")
	}

	smtpServer, smtpServerPort := setupSmtpServer(t, smtpHandler, nil)
	testHttpServer, shutdownWaitGroup, triggerShutdown := setupHttpServer(context.Background(), smtpServerPort)

	response := requestPostEmail(t, testHttpServer.URL)
	assert.Equal(t, http.StatusSeeOther, response.StatusCode)
	assert.Equal(t, expectedErrorRedirectUrl, response.Header.Get("Location"))

	teardownHttpServer(testHttpServer, shutdownWaitGroup, triggerShutdown)
	teardownSmtpServer(smtpServer)
}

func TestRedirectIfErrorConnectingToSmtpServer(t *testing.T) {
	testHttpServer, shutdownWaitGroup, triggerShutdown := setupHttpServer(context.Background(), 1234)

	response := requestPostEmail(t, testHttpServer.URL)
	assert.Equal(t, http.StatusSeeOther, response.StatusCode)
	assert.Equal(t, expectedErrorRedirectUrl, response.Header.Get("Location"))

	teardownHttpServer(testHttpServer, shutdownWaitGroup, triggerShutdown)
}

func TestReuseSmtpConnection(t *testing.T) {
	connSetupCount := 0
	smtpAuthHandler := func(_ net.Addr, _ string, _ []byte, _ []byte, _ []byte) (bool, error) {
		connSetupCount++
		return true, nil
	}

	smtpServer, smtpServerPort := setupSmtpServer(t, nil, smtpAuthHandler)
	testHttpServer, shutdownWaitGroup, triggerShutdown := setupHttpServer(context.Background(), smtpServerPort)

	requestPostEmail(t, testHttpServer.URL)
	requestPostEmail(t, testHttpServer.URL)
	assert.Equal(t, 1, connSetupCount)

	teardownHttpServer(testHttpServer, shutdownWaitGroup, triggerShutdown)
	teardownSmtpServer(smtpServer)
}

func TestCancellation(t *testing.T) {
	smtpRequestReceived := make(chan struct{})
	unlockSmtpServer := make(chan struct{})
	smtpHandler := func(_ net.Addr, _ string, _ []string, _ []byte) error {
		smtpRequestReceived <- struct{}{}
		<-unlockSmtpServer
		return nil
	}

	httpHandlerContext, triggerCancellation := context.WithCancel(context.Background())
	smtpServer, smtpServerPort := setupSmtpServer(t, smtpHandler, nil)
	testHttpServer, shutdownWaitGroup, triggerShutdown := setupHttpServer(httpHandlerContext, smtpServerPort)

	var response *http.Response
	httpRequestCompleted := make(chan struct{})
	go func() {
		response = requestPostEmail(t, testHttpServer.URL)
		httpRequestCompleted <- struct{}{}
	}()
	<-smtpRequestReceived

	triggerCancellation()
	unlockSmtpServer <- struct{}{}
	<-httpRequestCompleted

	assert.Equal(t, http.StatusSeeOther, response.StatusCode)
	assert.Equal(t, expectedErrorRedirectUrl, response.Header.Get("Location"))

	teardownHttpServer(testHttpServer, shutdownWaitGroup, triggerShutdown)
	teardownSmtpServer(smtpServer)
}

func setupSmtpServer(t *testing.T, handler smtpd.Handler, authHandler smtpd.AuthHandler) (*smtpd.Server, int) {
	smtpServer := newSmtpServer(t, handler, authHandler)
	smtpListener, smtpServerPort := newSmtpServerListener()
	go serveSmtp(smtpServer, smtpListener)
	return smtpServer, smtpServerPort
}

func setupHttpServer(appContext context.Context, smtpServerPort int) (*httptest.Server, *sync.WaitGroup, func()) {
	httpServerContext, triggerShutdown := context.WithCancel(appContext)
	shutdownWaitGroup := &sync.WaitGroup{}
	handleEmail := HandlePostEmail(httpServerContext, shutdownWaitGroup, mockGetEnvWithServerPort(smtpServerPort))
	httpEmailHandler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		request = request.WithContext(appContext)
		handleEmail(response, request)
	})
	testHttpServer := httptest.NewServer(httpEmailHandler)
	return testHttpServer, shutdownWaitGroup, triggerShutdown
}

func newSmtpServer(t *testing.T, handler smtpd.Handler, authHandler smtpd.AuthHandler) *smtpd.Server {
	if authHandler == nil {
		authHandler = defaultSmtpAuthHandlerfunc(t)
	}
	server := &smtpd.Server{
		Handler:     handler,
		AuthHandler: authHandler,
		TLSRequired: true,
	}
	err := server.ConfigureTLS("../../smtp_test_server.crt", "../../smtp_test_server.key")
	if err != nil {
		log.Panicf("Failed to configure TLS for SMTP server: %s\n", err)
	}
	return server
}

func defaultSmtpAuthHandlerfunc(t *testing.T) smtpd.AuthHandler {
	return func(_ net.Addr, mechanism string, username []byte, password []byte, _ []byte) (bool, error) {
		assert.Equal(t, "PLAIN", mechanism)
		assert.Equal(t, sourceEmailAddress, string(username))
		assert.Equal(t, sourceEmailPassword, string(password))
		return true, nil
	}
}

func newSmtpServerListener() (net.Listener, int) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Panicf("Failed to start TCP listener for SMTP server: %s\n", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	return listener, port
}

func serveSmtp(server *smtpd.Server, listener net.Listener) {
	err := server.Serve(listener)
	if !errors.Is(err, smtpd.ErrServerClosed) {
		log.Panicf("SMTP server crashed: %s\n", err)
	}
}

func mockGetEnvWithServerPort(smtpServerPort int) func(string) string {
	return func(key string) string {
		switch key {
		case "SMTP_CLIENT_DOMAIN":
			return "localhost"
		case "SMTP_SERVER_DOMAIN":
			return "localhost"
		case "SMTP_SERVER_PORT":
			return fmt.Sprint(smtpServerPort)
		case "TARGET_EMAIL_ADDRESS":
			return targetEmailAddress
		case "SOURCE_EMAIL_ADDRESS":
			return sourceEmailAddress
		case "SOURCE_EMAIL_PASSWORD":
			return sourceEmailPassword
		case "TEST_ONLY_SKIP_TLS_VERIFY":
			return "dummy string just in case"
		default:
			return ""
		}
	}
}

func teardownHttpServer(testHttpServer *httptest.Server, shutdownWaitGroup *sync.WaitGroup, triggerShutdown func()) {
	triggerShutdown()
	shutdownWaitGroup.Wait()
	testHttpServer.Close()
}

func teardownSmtpServer(smtpServer *smtpd.Server) {
	smtpServer.Shutdown(context.Background())
}

func requestPostEmail(t *testing.T, url string) *http.Response {
	httpClient := newHttpClientNoRedirects()
	response, err := httpClient.Post(url, "application/json", newPostBody())
	require.Nil(t, err, "Failed to POST email: %s\n", err)
	return response
}

func newHttpClientNoRedirects() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func newPostBody() io.Reader {
	requestBody := &requestBody{
		Sender:             emailSender,
		Subject:            emailSubject,
		Body:               emailBody,
		SuccessRedirectUrl: successRedirectUrl,
	}
	dumpedRequestBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Panicf("Failed to serialize POST request body: %s\n", err)
	}
	return bytes.NewBuffer(dumpedRequestBody)
}
