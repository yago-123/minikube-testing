package runtime

import (
	"context"

	"github.com/docker/docker/api/types"
)

type Controller interface {
	BuildImage(ctx context.Context, image, tag string, dockerfile []byte, files []string, args ...string) error
	BuildImageWithOptions(ctx context.Context, dockerfile []byte, files []string, buildOptions types.ImageBuildOptions) error

	BuildMultiStageImage(ctx context.Context) error

	PushImage(ctx context.Context, image, tag string) error
}
