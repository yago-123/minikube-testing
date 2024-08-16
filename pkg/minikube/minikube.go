package minikube

import (
	"fmt"
	"io"
	"os/exec"
)

type Minikube interface {
	Create(version string, nodes, cpusPerNode, memoryPerNode uint) error
	Destroy() error
}

type MinikubeController struct {
	stdout io.Writer
	stderr io.Writer
}

func NewMinikubeController(stdout, stderr io.Writer) *MinikubeController {
	return &MinikubeController{
		stdout: stdout,
		stderr: stderr,
	}
}

func (mc *MinikubeController) Create(version string, nodes, cpusPerNode, memoryPerNode uint) error {
	cmd := exec.Command(
		"minikube",
		"start",
		fmt.Sprintf("--kubernetes-version=%s", version),
		fmt.Sprintf("--nodes=%d", nodes),
		fmt.Sprintf("--cpus=%d", cpusPerNode),
		fmt.Sprintf("--memory=%d", memoryPerNode),
	)

	cmd.Stdout = mc.stdout
	cmd.Stderr = mc.stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start minikube: %w", err)
	}

	return nil
}

func (mc *MinikubeController) Destroy() error {
	cmd := exec.Command(
		"minikube",
		"delete",
	)

	cmd.Stdout = mc.stdout
	cmd.Stderr = mc.stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to delete minikube: %w", err)
	}

	return nil
}
