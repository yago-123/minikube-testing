package main

import (
	"context"
	"minikube-testing/pkg/minikube"
	"minikube-testing/pkg/runtime"
	"os"
)

func main() {
	mini := minikube.NewMinikubeController(os.Stdout, os.Stderr)

	mini.Create("1.20.0", 1, 2, 2048)
	err := mini.Destroy()
	if err != nil {
		panic(err)
	}

	dock, err := runtime.NewDockerController("credentials")
	if err != nil {
		panic(err)
	}

	if err = dock.BuildImage(context.Background(), "my-image", "latest", "Dockerfile"); err != nil {
		panic(err)
	}

	if err = dock.PushImage(context.Background(), "my-image", "latest"); err != nil {
		panic(err)
	}
}
