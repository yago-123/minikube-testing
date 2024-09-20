package orchestrator

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/yago-123/minikube-testing/pkg/client"

	"github.com/google/uuid"
)

type Minikube struct {
	stdout  io.Writer
	stderr  io.Writer
	profile string
}

func NewMinikube(stdout, stderr io.Writer) *Minikube {
	return &Minikube{
		stdout:  stdout,
		stderr:  stderr,
		profile: uuid.NewString(),
	}
}

func NewMinikubeWithProfile(stdout, stderr io.Writer, profile string) *Minikube {
	return &Minikube{
		stdout:  stdout,
		stderr:  stderr,
		profile: profile,
	}
}

func (mc *Minikube) Create(version string, nodes, cpusPerNode, memoryPerNode uint) (client.Client, error) {
	cmd := exec.Command(
		"minikube",
		"start",
		fmt.Sprintf("--kubernetes-version=%s", version),
		fmt.Sprintf("--nodes=%d", nodes),
		fmt.Sprintf("--cpus=%d", cpusPerNode),
		fmt.Sprintf("--memory=%d", memoryPerNode),
		fmt.Sprintf("--profile=%s", mc.profile),
	)

	cmd.Stdout = mc.stdout
	cmd.Stderr = mc.stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start minikube: %w", err)
	}

	cli, err := client.NewClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}

	return cli, nil
}

func (mc *Minikube) LoadImage(image string) error {
	cmd := exec.Command(
		"minikube",
		"image",
		"load",
		"--profile",
		mc.profile,
		image,
	)

	cmd.Stdout = mc.stdout
	cmd.Stderr = mc.stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to load image %s: %w", image, err)
	}

	return nil
}

func (mc *Minikube) Delete() error {
	cmd := exec.Command(
		"minikube",
		"delete",
		fmt.Sprintf("--profile=%s", mc.profile),
	)

	cmd.Stdout = mc.stdout
	cmd.Stderr = mc.stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to delete minikube: %w", err)
	}

	return nil
}
