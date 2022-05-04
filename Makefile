GOCMD=go
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GORUN=$(GOCMD) run
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOTOOL=$(GOCMD) tool
GOMOD=$(GOCMD) mod
GOPATH?=`$(GOCMD) env GOPATH`

BINARY=imap

TESTS=./...
COVERAGE_FILE=coverage.out

BUILD=build/

BUILD_DATE=`date`
BUILD_COMMIT=`git rev-parse HEAD`

all: download test build
.PHONY: all download test build install dovecot

download:
	@echo "[*] $@"
	@$(GOMOD) download

test: download
	@echo "[*] $@"
	$(GOTEST) -v $(TESTS)

coverage:
	@echo "[*] $@"
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) $(TESTS)
	$(GOTOOL) cover -html=$(COVERAGE_FILE)

build: download
	@echo "[*] $@"
	$(GOBUILD) -o $(BINARY) -v -ldflags "-X 'main.buildCommit=${BUILD_COMMIT}' -X 'main.buildDate=${BUILD_DATE}' -X 'main.buildBy=make'"

install: download
	@echo "[*] $@"
	$(GOINSTALL) -v -ldflags "-X 'main.buildCommit=${BUILD_COMMIT}' -X 'main.buildDate=${BUILD_DATE}' -X 'main.buildBy=make'"

dovecot:
	@echo "[*] $@"
	docker run -d --rm -p 143:143 -p 993:993 dovecot/dovecot:latest