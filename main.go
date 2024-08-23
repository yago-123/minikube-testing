package main

import (
	"context"
	"minikube-testing/pkg/minikube"
	"minikube-testing/pkg/runtime"
	"os"
)

const (
	KubernetesVersion  = "1.20.0"
	NumberOfNodes      = 1
	NumberOfCPUs       = 2
	AmountOfRAMPerNode = 2048
)

func main() {
	mini := minikube.NewMinikubeController(os.Stdout, os.Stderr)

	_ = mini.Create(KubernetesVersion, NumberOfNodes, NumberOfCPUs, AmountOfRAMPerNode)
	err := mini.Destroy()
	if err != nil {
		panic(err)
	}

	dock, err := runtime.NewDockerController("credentials")
	if err != nil {
		panic(err)
	}

	if err = dock.BuildImage(context.Background(), "my-image", "latest", []byte("Dockerfile-content"), []string{}); err != nil {
		panic(err)
	}

	if err = dock.PushImage(context.Background(), "my-image", "latest"); err != nil {
		panic(err)
	}
}
