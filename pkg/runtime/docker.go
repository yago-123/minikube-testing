package runtime

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"

	img "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type Controller interface {
	BuildImage(ctx context.Context, image, tag, dockerfile string, files []string, args ...string) error
	BuildImageWithOptions(ctx context.Context, dockerfile string, files []string, buildOptions types.ImageBuildOptions) error

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

func (dc *Docker) BuildImage(ctx context.Context, image, tag, dockerfile string, filesContext []string, args ...string) error {
	options := types.ImageBuildOptions{
		Tags: []string{fmt.Sprintf("%s:%s", image, tag)},
	}

	return dc.BuildImageWithOptions(ctx, dockerfile, filesContext, options)
}

func (dc *Docker) BuildImageWithOptions(ctx context.Context, dockerfile string, filesContext []string, buildOptions types.ImageBuildOptions) error {
	buf, err := generateBuildContext(dockerfile, filesContext)
	if err != nil {
		return fmt.Errorf("error generating build buffer: %w", err)
	}

	_, err = dc.cli.ImageBuild(ctx, buf, buildOptions)
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

// generateBuildContext creates a buffer that contains the Dockerfile body and the dependency files required
// during the build step
func generateBuildContext(dockerfile string, filesContext []string) (*bytes.Buffer, error) {
	var err error
	var file *os.File
	var info os.FileInfo

	// create buffer that will hold data and tar writer
	buf := new(bytes.Buffer)
	tarBuf := tar.NewWriter(buf)

	// create image header for build instructions
	err = tarBuf.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	})
	if err != nil {
		return &bytes.Buffer{}, fmt.Errorf("error writing tar header for dockerfile: %w", err)
	}

	// add image build instructions
	_, err = tarBuf.Write([]byte(dockerfile))
	if err != nil {
		return &bytes.Buffer{}, fmt.Errorf("error writing dockerfile content to buffer: %w", err)
	}

	// add all additional files required to execute the dockerfile (build context)
	for _, filePath := range filesContext {
		file, err = os.Open(filePath)
		if err != nil {
			return &bytes.Buffer{}, fmt.Errorf("error accessing %s file: %w", filePath, err)
		}
		defer file.Close()

		info, err = file.Stat()
		if err != nil {
			return &bytes.Buffer{}, fmt.Errorf("error stating %s file: %w", filePath, err)
		}

		// generate header
		err = tarBuf.WriteHeader(&tar.Header{
			Name: filepath.Base(filePath),
			Size: info.Size(),
		})
		if err != nil {
			return &bytes.Buffer{}, fmt.Errorf("error writing header for %s: %w", filePath, err)
		}

		// append to the tar together with the dockerfile
		_, err = io.Copy(tarBuf, file)
		if err != nil {
			return &bytes.Buffer{}, fmt.Errorf("error copying file %s: %w", filePath, err)
		}
	}

	if err = tarBuf.Close(); err != nil {
		return &bytes.Buffer{}, fmt.Errorf("error closing buffer: %w", err)
	}

	return buf, nil
}
