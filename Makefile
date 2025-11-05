all: gen build

gen:
	@echo "Installing oapi-codegen if missing..."
	@command -v oapi-codegen >/dev/null 2>&1 || go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1
	@echo "Generating API from swagger/openapi.yml..."
	go generate ./...

build:
	@echo "Building binary..."
	go build -o bin/pod_api ./cmd

tidy:
	go mod tidy
