package main

import (
	"context"
	"fmt"
	"log"
	"minikube-testing/pkg/client"
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
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// mini := minikube.NewMinikubeController(os.Stdout, os.Stderr)

	// err = mini.Create(KubernetesVersion, NumberOfNodes, NumberOfCPUs, AmountOfRAMPerNode)
	// if err != nil {
	// 	panic(err)
	// }
	// defer mini.Destroy()

	// todo(): think about better way, probably not the best option :)
	dock, err := runtime.NewDockerController(
		os.Getenv("DOCKER_USER"),
		os.Getenv("DOCKER_PASSWORD"),
	)
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

	// if err = dock.PushImage(context.Background(), "yagoninja/minikube-testing", "latest2"); err != nil {
	// 	panic(err)
	// }

	// mini.DeployWithHelm()

	client, err := client.NewClient()
	if err != nil {
		panic(err)
	}

	logs, err := client.CurlEndpoint(context.Background(), "http://172.17.0.3:8081/api/data")
	if err != nil {
		panic(err)
	}

	fmt.Println("logs expected: %s", logs)
}
