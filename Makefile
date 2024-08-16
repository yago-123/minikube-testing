.PHONY: all
all: lint main

.PHONY: main
main:
	@echo "Building main..."
	@go build -o minikube-testing main.go

.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...
