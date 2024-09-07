package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lambdaServerUrl = "http://localhost:3000"

const targetEmailAddress = "target@test.com"
const emailSubject = "Test subject"
const emailBody = "Test body"

var expectedErrorRedirectUrl = fmt.Sprintf(
	"mailto:%s?subject=%s&body=%s",
	targetEmailAddress,
	url.PathEscape(emailSubject),
	url.PathEscape(emailBody),
)

func TestLambdaIntegration(t *testing.T) {
	lamdbaServer := setupLambdaServer()
	defer teardownLambdaServer(lamdbaServer)
	testPostEmailIntegration(t)
}

// Running Lambda integration without SMTP server is good enough.
// As the HTTP server runs in a docker container managed by AWS SAM,
// I couldn't manage to make it communicate with an SMTP server on localhost.
func testPostEmailIntegration(t *testing.T) {
	response := requestPostEmail(t)
	assert.Equal(t, http.StatusSeeOther, response.StatusCode)
	assert.Equal(t, expectedErrorRedirectUrl, response.Header.Get("Location"))
}

func setupLambdaServer() *exec.Cmd {
	buildLambdaServerCmd := exec.Command("sam", "build", "-t", "integration-template.yml")
	err := buildLambdaServerCmd.Run()
	if err != nil {
		log.Panicf("Failed to build Lambda server: %s\n", err)
	}

	runLambdaServerCmd := exec.Command("sam", "local", "start-api", "--docker-network", "host")
	err = runLambdaServerCmd.Start()
	if err != nil {
		log.Panicf("Failed to start local Lambda API: %s\n", err)
	}

	readyCheckInterval := time.NewTicker(time.Second)
	for range readyCheckInterval.C {
		_, err := http.Get(lambdaServerUrl)
		if err == nil {
			break
		}
	}
	return runLambdaServerCmd
}

func teardownLambdaServer(lambdaServerCmd *exec.Cmd) {
	err := lambdaServerCmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		log.Panicf("Failed to kill Lambda server: %s\n", err)
	}
	lambdaServerCmd.Wait()
}

func requestPostEmail(t *testing.T) *http.Response {
	httpClient := newHttpClientNoRedirects()
	response, err := httpClient.Post(lambdaServerUrl+"/api/email", "application/json", newPostBody())
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

type requestBody struct {
	Sender             string
	Subject            string
	Body               string
	SuccessRedirectUrl string
}

func newPostBody() io.Reader {
	requestBody := &requestBody{
		Subject: emailSubject,
		Body:    emailBody,
	}
	dumpedRequestBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Panicf("Failed to serialize POST request body: %s\n", err)
	}
	return bytes.NewBuffer(dumpedRequestBody)
}
