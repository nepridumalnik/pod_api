.PHONY: all gen build tidy

GO_BIN := $(shell go env GOPATH)/bin

all: gen build

gen:
	@echo "Installing oapi-codegen if missing..."
	@command -v $(GO_BIN)/oapi-codegen >/dev/null 2>&1 || go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1
	@echo "Installing easyjson if missing..."
	@command -v $(GO_BIN)/easyjson >/dev/null 2>&1 || go install github.com/mailru/easyjson/...@latest
	@echo "Generating API from swagger/openapi.yml..."
	PATH="$(GO_BIN):$$PATH" go generate ./...

build:
	@echo "Building binary..."
	go build -o bin/pod_api ./cmd

tidy:
	go mod tidy
