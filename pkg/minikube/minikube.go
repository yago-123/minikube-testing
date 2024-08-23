package minikube

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/google/uuid"
)

type Minikube interface {
	Create(version string, nodes, cpusPerNode, memoryPerNode uint) error
	Deploy(app string) error
	DeployWithHelm() error
	Destroy() error
}

type Controller struct {
	stdout  io.Writer
	stderr  io.Writer
	profile string
}

func NewMinikubeController(stdout, stderr io.Writer) *Controller {
	return &Controller{
		stdout:  stdout,
		stderr:  stderr,
		profile: uuid.NewString(),
	}
}

func NewMiniKubeControllerWithProfile(stdout, stderr io.Writer, profile string) *Controller {
	return &Controller{
		stdout:  stdout,
		stderr:  stderr,
		profile: profile,
	}
}

func (mc *Controller) Create(version string, nodes, cpusPerNode, memoryPerNode uint) error {
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
		return fmt.Errorf("failed to start minikube: %w", err)
	}

	return nil
}

func (mc *Controller) Deploy(_ string) error {
	return nil
}

func (mc *Controller) DeployWithHelm() error {
	return nil
}

func (mc *Controller) Destroy() error {
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
