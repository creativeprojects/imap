GOCMD=go
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GORUN=$(GOCMD) run
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOTOOL=$(GOCMD) tool
GOMOD=$(GOCMD) mod
GOPATH?=$(shell $(GOCMD) env GOPATH)
GOBIN?=$(shell $(GOCMD) env GOBIN)

BINARY=imap
BINARY_LINUX=$(BINARY)_linux

TESTS=./...
COVERAGE_FILE=coverage.out

BUILD=build/

BUILD_DATE=`date`
BUILD_COMMIT=`git rev-parse HEAD`

all: download test build
.PHONY: all download test coverage build build-linux install dovecot nightly generate-install

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

build-linux: download
	@echo "[*] $@"
	GOOS="linux" GOARCH="amd64" $(GOBUILD) -o $(BINARY_LINUX) -v -ldflags "-X 'main.buildCommit=${BUILD_COMMIT}' -X 'main.buildDate=${BUILD_DATE}' -X 'main.buildBy=make'"

install: download
	@echo "[*] $@"
	$(GOINSTALL) -v -ldflags "-X 'main.buildCommit=${BUILD_COMMIT}' -X 'main.buildDate=${BUILD_DATE}' -X 'main.buildBy=make'"

dovecot:
	@echo "[*] $@"
	docker run -d --rm -p 143:143 -p 993:993 dovecot/dovecot:latest

$(GOBIN)/eget:
	@echo "[*] $@"
	go install -v github.com/zyedidia/eget@latest

$(GOBIN)/goreleaser: $(GOBIN)/eget
	@echo "[*] $@"
	eget goreleaser/goreleaser --to $(GOBIN)

nightly: $(GOBIN)/goreleaser
	@echo "[*] $@"
	goreleaser --snapshot --skip-publish --rm-dist

generate-install:
	@echo "[*] $@"
	godownloader .godownloader.yml -r creativeprojects/imap -o install.sh
