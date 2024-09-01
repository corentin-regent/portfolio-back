package email

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"sync"
)

var errCancelled = errors.New("POST /email request was cancelled")

type smtpServer struct {
	Host string
	Port string
}

func (server *smtpServer) Name() string {
	return server.Host + ":" + server.Port
}

type requestBody struct {
	Sender             string
	Subject            string
	Body               string
	SuccessRedirectUrl string
}

func HandlePostEmail(
	appContext context.Context,
	shutdownWaitGroup *sync.WaitGroup,
	getEnv func(string) string,
) http.HandlerFunc {

	smtpClientDomain := getEnv("SMTP_CLIENT_DOMAIN")
	smtpServerDomain := getEnv("SMTP_SERVER_DOMAIN")
	smtpServerPort := getEnv("SMTP_SERVER_PORT")
	targetEmailAddress := getEnv("TARGET_EMAIL_ADDRESS")
	sourceEmailAddress := getEnv("SOURCE_EMAIL_ADDRESS")
	sourceEmailPassword := getEnv("SOURCE_EMAIL_PASSWORD")
	skipTlsVerify := getEnv("TEST_ONLY_SKIP_TLS_VERIFY") == "dummy string just in case"

	smtpMessageMutex := sync.Mutex{}

	buildMessage := func(email *requestBody) string {
		return fmt.Sprintf(
			"To: %s\r\nSubject: %s\r\n\r\n%s\r\n\r\nSent by %s",
			targetEmailAddress,
			email.Subject,
			email.Body,
			email.Sender,
		)
	}

	setupSmtpClient := func() (client *smtp.Client, err error) {
		server := &smtpServer{
			Host: smtpServerDomain,
			Port: smtpServerPort,
		}
		tlsConfig := &tls.Config{
			InsecureSkipVerify: skipTlsVerify,
			ServerName:         server.Host,
		}
		auth := smtp.PlainAuth("", sourceEmailAddress, sourceEmailPassword, server.Host)

		log.Println("[DEBUG] Establishing TCP connection with SMTP server")
		conn, err := net.Dial("tcp", server.Name())
		if err != nil {
			return
		}
		log.Println("[DEBUG] Creating SMTP client")
		client, err = smtp.NewClient(conn, server.Host)
		if err != nil {
			return
		}
		log.Println("[DEBUG] Sending HELLO to SMTP server")
		if err = client.Hello(smtpClientDomain); err != nil {
			return
		}
		log.Println("[DEBUG] Negotiating TLS encryption for SMTP communication")
		if err = client.StartTLS(tlsConfig); err != nil {
			return
		}
		log.Println("[DEBUG] Authenticating to the SMTP server")
		err = client.Auth(auth)
		return
	}

	cancelEmail := func(client *smtp.Client) (err error) {
		log.Println("[DEBUG] Aborting SMTP email")
		err = client.Reset()
		if err != nil {
			return
		}
		return errCancelled
	}

	sendEmail := func(request *http.Request, client *smtp.Client, email *requestBody) (err error) {
		doneChannel := make(chan struct{})
		smtpMessageMutex.Lock()
		defer smtpMessageMutex.Unlock()

		log.Println("[DEBUG] Setting SMTP email sender")
		go func() {
			err = client.Mail(sourceEmailAddress)
			doneChannel <- struct{}{}
		}()
		select {
		case <-doneChannel:
			if err != nil {
				return
			}
		case <-request.Context().Done():
			return cancelEmail(client)
		}

		log.Println("[DEBUG] Setting SMTP email receiver")
		go func() {
			err = client.Rcpt(targetEmailAddress)
			doneChannel <- struct{}{}
		}()
		select {
		case <-doneChannel:
			if err != nil {
				return
			}
		case <-request.Context().Done():
			return cancelEmail(client)
		}

		log.Println("[DEBUG] Starting SMTP email body")
		var messageWriter io.WriteCloser
		go func() {
			messageWriter, err = client.Data()
			doneChannel <- struct{}{}
		}()
		select {
		case <-doneChannel:
			if err != nil {
				return
			}
		case <-request.Context().Done():
			return cancelEmail(client)
		}

		log.Println("[DEBUG] Writing SMTP email body")
		go func() {
			_, err = messageWriter.Write([]byte(buildMessage(email)))
			doneChannel <- struct{}{}
		}()
		select {
		case <-doneChannel:
			if err != nil {
				return
			}
		case <-request.Context().Done():
			return cancelEmail(client)
		}

		log.Println("[DEBUG] Sending SMTP email")
		go func() {
			err = messageWriter.Close()
			doneChannel <- struct{}{}
		}()
		select {
		case <-doneChannel:
			return
		case <-request.Context().Done():
			return cancelEmail(client)
		}
	}

	failPostEmail := func(response http.ResponseWriter, request *http.Request, email *requestBody, err error) {
		log.Printf("[ERROR] POST /email failed for sender %q: %s\n", email.Sender, err)
		failureRedirectUrl := fmt.Sprintf(
			"mailto:%s?subject=%s&body=%s",
			targetEmailAddress,
			url.PathEscape(email.Subject),
			url.PathEscape(email.Body),
		)
		http.Redirect(response, request, failureRedirectUrl, http.StatusSeeOther)
	}

	var (
		initSmtp     sync.Once
		smtpClient   *smtp.Client
		smtpSetupErr error
	)

	shutdownWaitGroup.Add(1)
	listenForShutdown := func() {
		<-appContext.Done()
		if smtpClient != nil {
			log.Println("[INFO] Shutting down SMTP client")
			err := smtpClient.Quit()
			if err != nil {
				log.Printf("[ERROR] SMTP client shutdown failed: %s\n", err)
			}
		}
		shutdownWaitGroup.Done()
	}

	go listenForShutdown()
	return func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		var email *requestBody
		err := decoder.Decode(&email)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
		}

		initSmtp.Do(func() {
			log.Println("[INFO] Setting up SMTP client")
			smtpClient, smtpSetupErr = setupSmtpClient()
			if smtpSetupErr == nil {
				log.Println("[INFO] SMTP client is ready")
			} else {
				log.Printf("[ERROR] SMTP client setup failed: %s\n", smtpSetupErr)
			}
		})
		if smtpSetupErr != nil {
			failPostEmail(response, request, email, smtpSetupErr)
			return
		}

		err = sendEmail(request, smtpClient, email)
		if err == nil {
			http.Redirect(response, request, email.SuccessRedirectUrl, http.StatusFound)
		} else {
			failPostEmail(response, request, email, err)
		}
	}
}
