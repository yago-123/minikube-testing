package runtime

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"

	img "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type RuntimeController interface {
	BuildImage(ctx context.Context, image, tag, dockerfile string, args ...string) error
	BuildImageWithOptions(ctx context.Context, dockerfile string, buildOptions types.ImageBuildOptions) error

	PushImage(ctx context.Context, image, tag string) error
}

type Docker struct {
	cli         *client.Client
	credentials string
}

func NewDockerController(creds string) (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Docker{cli: cli, credentials: creds}, nil
}

func (dc *Docker) BuildImage(ctx context.Context, image, tag, dockerfile string, args ...string) error {
	options := types.ImageBuildOptions{
		Tags: []string{fmt.Sprintf("%s:%s", image, tag)},
	}

	return dc.BuildImageWithOptions(ctx, dockerfile, options)
}

func (dc *Docker) BuildImageWithOptions(ctx context.Context, dockerfile string, buildOptions types.ImageBuildOptions) error {
	_, err := dc.cli.ImageBuild(ctx, nil, buildOptions)
	if err != nil {
		return err
	}

	return nil
}

func (dc *Docker) PushImage(ctx context.Context, image, tag string) error {
	push, err := dc.cli.ImagePush(ctx, fmt.Sprintf("%s:%s", image, tag), img.PushOptions{
		RegistryAuth: dc.credentials,
	})
	if err != nil {
		return err
	}

	return push.Close()
}
