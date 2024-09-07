# See https://www.gaunt.dev/blog/2022/glibc-error-with-aws-sam-and-go/
build-PortfolioBack:
	CGO_ENABLED=0 go build
	mv portfolio-back $(ARTIFACTS_DIR)
