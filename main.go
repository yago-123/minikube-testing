package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"log"
	"minikube-testing/pkg/orchestrator"
	"minikube-testing/pkg/runtime"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const (
	KubernetesVersion  = "1.20.0"
	NumberOfNodes      = 1
	NumberOfCPUs       = 2
	AmountOfRAMPerNode = 2048
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %w", err)
	}

	logger := logrus.New()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// todo(): think about better way, probably not the best option :)
	dock, err := runtime.NewDockerController(
		os.Getenv("DOCKER_USER"),
		os.Getenv("DOCKER_PASSWORD"),
	)
	if err != nil {
		logger.Fatalf("unable to start docker controller: %v", err)
	}

	dockerfile := `
		# Use the official Golang image as a base image
		FROM golang:1.23-alpine AS builder
		
		# Set the Current Working Directory inside the container
		WORKDIR /app
		
		# Copy the Go Modules manifests
		COPY go.mod go.sum ./
		
		# Download Go Modules dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
		RUN go mod download
		
		# Copy the source code into the container
		COPY . .
		
		# Build the Go app
		RUN go build -o main .
		
		# Start a new stage from scratch
		FROM alpine:latest
		
		# Set the Current Working Directory inside the container
		WORKDIR /root/
		
		# Copy the Pre-built binary file from the previous stage
		COPY --from=builder /app/main .
		
		# Expose port 8080 to the outside world
		EXPOSE 8080
		
		# Command to run the executable
		CMD ["./main"]
	`

	if err = dock.BuildImageWithContextPath(ctx, "yagoninja/api-server-test", "0.1.0", []byte(dockerfile), "build/docker/test-pod"); err != nil {
		logger.Fatalf("unable to build image: %w", err)
	}

	// if err = dock.PushImage(ctx, "yagoninja/api-server-test", "0.1.0"); err != nil {
	// 	log.Fatalf("unable to push image: %w", err)
	// }

	minikube := orchestrator.NewMinikube(os.Stdout, os.Stderr)
	cli, err := minikube.Create(KubernetesVersion, NumberOfNodes, NumberOfCPUs, AmountOfRAMPerNode)
	if err != nil {
		logger.Fatalf("unable to create minikube cluster: %w", err)
	}
	defer minikube.Delete()

	// todo(): add some sort of wait mechanism
	time.Sleep(10 * time.Second)

	err = minikube.LoadImage("yagoninja/api-server-test:0.1.0")
	if err != nil {
		logger.Errorf("unable to load image: %w", err)
		return
	}

	yamlManifest := `
apiVersion: v1
kind: Pod
metadata:
    name: go-app
spec:
    containers:
      - name: go-app
        image: yagoninja/api-server-test:0.1.0
        imagePullPolicy: IfNotPresent
        ports:
          - containerPort: 8080
`

	err = cli.RunYAML(ctx, []byte(yamlManifest))
	if err != nil {
		logger.Errorf("unable to run yaml manifest: %w", err)
		return
	}

	// todo(): add some sort of wait mechanism
	time.Sleep(10 * time.Second)

	pod, err := cli.GetPod(ctx, "go-app", "default")
	if err != nil {
		logger.Errorf("unable to get pod: %w", err)
		return
	}

	resp, err := cli.CurlPod(ctx, pod, 8080, "api")
	if err != nil {
		logger.Errorf("unable to curl pod: %w", err)
		return
	}

	logger.Infof("HTTP response from %s: %d", pod.Name, resp.StatusCode)
}
