.PHONY: all
all: main

.PHONY: main
main:
	go build -o minikube-testing main.go
