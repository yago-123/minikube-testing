package main

import (
	"fmt"
	"minikube-testing/pkg/minikube"
	"os"
)

func main() {
	fmt.Println("Hello, World!")
	mini := minikube.NewMinikubeController(os.Stdout, os.Stderr)

	mini.Create("1.20.0", 1, 2, 2048)
	mini.Destroy()
}

//TIP See GoLand help at <a href="https://www.jetbrains.com/help/go/">jetbrains.com/help/go/</a>.
// Also, you can try interactive lessons for GoLand by selecting 'Help | Learn IDE Features' from the main menu.
