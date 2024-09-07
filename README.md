# Backend for my portfolio website

[![CI/CD](https://github.com/corentin-regent/portfolio-back/actions/workflows/cicd.yml/badge.svg)](https://github.com/corentin-regent/portfolio-back/actions/workflows/cicd.yml)

This is a Work In Progress

## Environment variables

| Name                       | Description                                                                         | Example                                            |
| -------------------------- | ----------------------------------------------------------------------------------- | -------------------------------------------------- |
| CORS_ALLOWED_ORIGINS       | Comma-separated list of front-end domains from which the client browser can request | corentin-regent.github.io,corentin-regent.is-a.dev |
| SMTP_CLIENT_DOMAIN         | Host name with which the SMTP client introduces itself before submitting emails     | localhost                                          |
| SMTP_SERVER_DOMAIN         | Domain of the SMTP server that collects emails                                      | smtp.gmail.com                                     |
| SMTP_SERVER_PORT           | Port on which the SMTP server listens to for incoming emails                        | 587                                                |
| SOURCE_EMAIL_ADDRESS       | Email address from which the emails are sent                                        | source@example.com                                 |
| SOURCE_EMAIL_PASSWORD      | Plain password for the source email address                                         | password                                           |
| TARGET_EMAIL_ADDRESS       | Email address to which the emails are sent                                          | target@gmail.com                                   |
| TIMEOUT_REQUEST_PROCESSING | Delay after which request processing should abort, in milliseconds                  | 5000                                               |
