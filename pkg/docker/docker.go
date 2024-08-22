package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"

	img "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type Docker interface {
	BuildImage(ctx context.Context, image, tag, dockerfile string, args ...string) error
	BuildImageWithOptions(ctx context.Context, dockerfile string, buildOptions types.ImageBuildOptions) error

	PushImage(ctx context.Context, image, tag string) error
}

type RuntimeController struct {
	cli         *client.Client
	credentials string
}

func NewDockerController(creds string) (*RuntimeController, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &RuntimeController{cli: cli, credentials: creds}, nil
}

func (dc *RuntimeController) BuildImage(ctx context.Context, image, tag, dockerfile string, args ...string) error {
	options := types.ImageBuildOptions{
		Tags: []string{fmt.Sprintf("%s:%s", image, tag)},
	}

	return dc.BuildImageWithOptions(ctx, dockerfile, options)
}

func (dc *RuntimeController) BuildImageWithOptions(ctx context.Context, dockerfile string, buildOptions types.ImageBuildOptions) error {
	_, err := dc.cli.ImageBuild(ctx, nil, buildOptions)
	if err != nil {
		return err
	}

	return nil
}

func (dc *RuntimeController) PushImage(ctx context.Context, image, tag string) error {
	push, err := dc.cli.ImagePush(ctx, fmt.Sprintf("%s:%s", image, tag), img.PushOptions{
		RegistryAuth: dc.credentials,
	})
	if err != nil {
		return err
	}

	return push.Close()
}
