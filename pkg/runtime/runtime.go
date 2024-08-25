package runtime

import (
	"context"

	"github.com/docker/docker/api/types"
)

type Runtime interface {
	BuildImage(ctx context.Context, image, tag string, dockerfile []byte, filesContext []string, args ...string) error
	BuildImageWithOptions(ctx context.Context, dockerfile []byte, filesContext []string, buildOptions types.ImageBuildOptions) error
	BuildImageWithContextPath(ctx context.Context, image, tag string, dockerfile []byte, contextPath string, args ...string) error

	BuildMultiStageImage(ctx context.Context) error

	PushImage(ctx context.Context, image, tag string) error
}
