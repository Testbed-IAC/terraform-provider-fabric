HOSTNAME=registry.terraform.io
NAMESPACE=Testbed-IAC
NAME=fabric
VERSION=0.1.0
BINARY=terraform-provider-${NAME}
OS_ARCH=$(shell go env GOOS)_$(shell go env GOARCH)

default: build

build:
	go build -o ${BINARY}

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

test:
	go test ./... -timeout 120s

testacc:
	TF_ACC=1 go test ./internal/provider -v -run TestAccFabric -timeout 60m

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	go vet ./...

docs:
	go generate ./...

tidy:
	go mod tidy

.PHONY: build install test testacc lint fmt docs tidy
