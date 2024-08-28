package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"sync"
)

type smtpServer struct {
	Host string
	Port string
}

func (server *smtpServer) Name() string {
	return server.Host + ":" + server.Port
}

type postEmailData struct {
	Sender             string
	Subject            string
	Body               string
	SuccessRedirectUrl string
}

func HandleEmail(
	appContext context.Context,
	shutdownWaitGroup *sync.WaitGroup,
	getEnv func(string) string,
) func(http.ResponseWriter, *http.Request) {

	smtpClientDomain := getEnv("SMTP_CLIENT_DOMAIN")
	smtpServerDomain := getEnv("SMTP_SERVER_DOMAIN")
	smtpServerPort := getEnv("SMTP_SERVER_PORT")
	targetEmailAddress := getEnv("TARGET_EMAIL_ADDRESS")
	sourceEmailAddress := getEnv("SOURCE_EMAIL_ADDRESS")
	sourceEmailPassword := getEnv("SOURCE_EMAIL_PASSWORD")

	var (
		initSmtp     sync.Once
		smtpClient   *smtp.Client
		smtpSetupErr error
	)

	buildMessage := func(email *postEmailData) string {
		return fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n\r\nSent by %s", targetEmailAddress, email.Subject, email.Body, email.Sender)
	}

	setupSmtpClient := func() (client *smtp.Client, err error) {
		server := smtpServer{
			Host: smtpServerDomain,
			Port: smtpServerPort,
		}
		tlsConfig := &tls.Config{
			ServerName: server.Host,
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

	sendEmail := func(client *smtp.Client, email *postEmailData) (err error) {
		log.Println("[DEBUG] Setting SMTP email sender")
		if err = client.Mail(sourceEmailAddress); err != nil {
			return
		}
		log.Println("[DEBUG] Setting SMTP email receiver")
		if err = client.Rcpt(targetEmailAddress); err != nil {
			return
		}
		log.Println("[DEBUG] Starting SMTP email body")
		messageWriter, err := client.Data()
		if err != nil {
			return
		}
		log.Println("[DEBUG] Writing SMTP email body")
		_, err = messageWriter.Write([]byte(buildMessage(email)))
		if err != nil {
			return
		}
		log.Println("[DEBUG] Sending SMTP email")
		err = messageWriter.Close()
		return
	}

	failPostEmail := func(response http.ResponseWriter, request *http.Request, email postEmailData, err error) {
		log.Printf("[ERROR] POST /email failed: %s\n", err)
		failureRedirectUrl := fmt.Sprintf(
			"mailto:%s?subject=%s&body=%s",
			targetEmailAddress,
			url.PathEscape(email.Subject),
			url.PathEscape(email.Body),
		)
		http.Redirect(response, request, failureRedirectUrl, http.StatusSeeOther)
	}

	handlePostEmail := func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(request.Body)
		var email postEmailData
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

		err = sendEmail(smtpClient, &email)
		if err == nil {
			http.Redirect(response, request, email.SuccessRedirectUrl, http.StatusFound)
		} else {
			failPostEmail(response, request, email, err)
		}
	}

	shutdownWaitGroup.Add(1)
	listenForShutdown := func() {
		<-appContext.Done()
		if smtpClient != nil {
			log.Println("[INFO] Shutting down SMTP client")
			err := smtpClient.Quit()
			if err != nil {
				log.Printf("[ERROR] SMTP client shutdown failed: %s\n", err.Error())
			}
		}
		shutdownWaitGroup.Done()
	}

	go listenForShutdown()
	return func(response http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPost:
			handlePostEmail(response, request)
		default:
			http.Error(response, fmt.Sprintf("unexpected method %q", request.Method), http.StatusBadRequest)
		}
	}
}
