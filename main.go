package main

import (
	"context"
	"log"
	"minikube-testing/pkg/runtime"
	"os"

	"github.com/joho/godotenv"
)

const (
	KubernetesVersion  = "1.20.0"
	NumberOfNodes      = 1
	NumberOfCPUs       = 2
	AmountOfRAMPerNode = 2048
)

func main() {
	// mini := minikube.NewMinikubeController(os.Stdout, os.Stderr)

	// _ = mini.Create(KubernetesVersion, NumberOfNodes, NumberOfCPUs, AmountOfRAMPerNode)
	// err := mini.Destroy()
	// if err != nil {
	// 	panic(err)
	// }

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// todo(): think about better way, probably not the best option :)
	user := os.Getenv("DOCKER_USER")
	pass := os.Getenv("DOCKER_PASSWORD")

	dock, err := runtime.NewDockerController(user, pass)
	if err != nil {
		panic(err)
	}

	dockerfile := `
		# Use the official Alpine Linux base image
		FROM alpine:latest
		
		# Install a text editor to create the script
		RUN apk add --no-cache bash
		
		# Add a simple script to the image
		RUN echo 'echo "Hello, World!"' > /hello.sh
		
		# Make the script executable
		RUN chmod +x /hello.sh
		
		# Run the script when the container starts
		CMD ["/hello.sh"]
	`

	if err = dock.BuildImage(context.Background(), "yagoninja/minikube-testing", "latest2", []byte(dockerfile), []string{}); err != nil {
		panic(err)
	}

	if err = dock.PushImage(context.Background(), "yagoninja/minikube-testing", "latest2"); err != nil {
		panic(err)
	}
}
