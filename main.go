package main

import (
	"context"
	"minikube-testing/pkg/docker"
	"minikube-testing/pkg/minikube"
	"os"
)

func main() {
	mini := minikube.NewMinikubeController(os.Stdout, os.Stderr)

	mini.Create("1.20.0", 1, 2, 2048)
	mini.Destroy()

	dock, err := docker.NewDockerController("credentials")
	if err != nil {
		panic(err)
	}

	dock.BuildImage(context.Background(), "my-image", "latest", "Dockerfile")
	dock.PushImage(context.Background(), "my-image", "latest")
}
