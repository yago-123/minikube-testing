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

.PHONY: image
image:
	@echo "Building images..."
	@docker build -t yagoninja/mini-curl -f build/docker/Dockerfile .
	@docker build -t yagoninja/test-curl -f build/docker/test-pod/Dockerfile build/docker/test-pod

.PHONY: imports
imports:
	@find . -name "*.go" | xargs goimports -w

