package runtime

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	img "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
)

const DockerfileDefaultName = "Dockerfile"

type Docker struct {
	cli         *client.Client
	credentials string
}

func NewDockerController(user, pass string) (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	creds, err := generateCredentials(user, pass)
	if err != nil {
		return &Docker{}, err
	}

	return &Docker{cli: cli, credentials: creds}, nil
}

func (dc *Docker) BuildImage(ctx context.Context, image, tag string, dockerfile []byte, filesContext []string, args ...string) error {
	options := types.ImageBuildOptions{
		Tags:       []string{fmt.Sprintf("%s:%s", image, tag)},
		Remove:     true, // remove intermediate containers from final image
		NoCache:    true, // remove caching during layer build
		Dockerfile: DockerfileDefaultName,
		// more default options
	}

	return dc.BuildImageWithOptions(ctx, dockerfile, filesContext, options)
}

func (dc *Docker) BuildImageWithContextPath(ctx context.Context, image, tag string, dockerfile []byte, contextPath string, args ...string) error {
	options := types.ImageBuildOptions{
		Tags:       []string{fmt.Sprintf("%s:%s", image, tag)},
		Remove:     true, // remove intermediate containers from final image
		NoCache:    true, // remove caching during layer build
		Dockerfile: DockerfileDefaultName,
		// more default options
	}

	filesContext, err := retrieveContextBuildFiles(contextPath)
	if err != nil {
		return fmt.Errorf("error retrieving files for context build: %w", err)
	}

	return dc.BuildImageWithOptions(ctx, dockerfile, filesContext, options)
}

func (dc *Docker) BuildImageWithOptions(ctx context.Context, dockerfile []byte, filesContext []string, buildOptions types.ImageBuildOptions) error {
	// put together the dockerfile and the files required for the build
	buf, err := generateBuildContext(dockerfile, filesContext)
	if err != nil {
		return fmt.Errorf("error generating build buffer: %w", err)
	}

	resp, err := dc.cli.ImageBuild(ctx, buf, buildOptions)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// analyze the build logs to verify correct image build  (NOTE: there is no alternative that can replace this)
	if err = analyzeBuildLogs(resp.Body); err != nil {
		return err
	}

	return nil
}

func (dc *Docker) BuildMultiStageImage(_ context.Context) error {
	return nil
}

func (dc *Docker) PushImage(ctx context.Context, image, tag string) error {
	push, err := dc.cli.ImagePush(ctx, fmt.Sprintf("%s:%s", image, tag), img.PushOptions{
		RegistryAuth: dc.credentials,
	})
	if err != nil {
		return err
	}
	defer push.Close()

	// analyze the push logs to verify correct push build (NOTE: there is no server response that can replace this)
	if err = analyzePushLogs(push); err != nil {
		return err
	}

	return nil
}

// generateBuildContext creates a buffer that contains the Dockerfile body and the dependency files required
// during the build step
func generateBuildContext(dockerfile []byte, filesContext []string) (*bytes.Buffer, error) {
	var err error
	var file *os.File
	var info os.FileInfo

	// create buffer that will hold data and tar writer
	buf := new(bytes.Buffer)
	tarBuf := tar.NewWriter(buf)

	// create image header for build instructions
	err = tarBuf.WriteHeader(&tar.Header{
		Name: DockerfileDefaultName,
		Size: int64(len(dockerfile)),
	})
	if err != nil {
		return &bytes.Buffer{}, fmt.Errorf("error writing tar header for dockerfile: %w", err)
	}

	// add image build instructions
	_, err = tarBuf.Write(dockerfile)
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

// buildLogMessage represents the log structure of docker when building an image
type buildLogMessage struct {
	Stream      string          `json:"stream,omitempty"`
	ErrorDetail json.RawMessage `json:"errorDetail,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// analyzeBuildLogs reads and analyzes docker build logs to detect errors
func analyzeBuildLogs(buildLogs io.Reader) error {
	scanner := bufio.NewScanner(buildLogs)
	for scanner.Scan() {
		line := scanner.Text()

		// skip empty lines if any
		if len(line) == 0 {
			continue
		}

		// unmarshal the log line into a buildLogMessage struct
		var logMsg buildLogMessage
		if err := json.Unmarshal([]byte(line), &logMsg); err != nil {
			return fmt.Errorf("error parsing build output: %w", err)
		}

		// check if the log contains an error
		if len(logMsg.ErrorDetail) > 0 {
			return fmt.Errorf("error during build step: %s", string(logMsg.ErrorDetail))
		}
	}

	// check for scanner errors
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading build output: %w", err)
	}

	// return nil if there are no errors
	return nil
}

// pushLogMessage represents the log structure of docker when pushing an image
type pushLogMessage struct {
	Status      string          `json:"status"`
	ErrorDetail json.RawMessage `json:"errorDetail"`
	Error       string          `json:"error"`
	Aux         json.RawMessage `json:"aux"`
}

// analyzePushLogs reads and analyzes docker push logs to detect errors
func analyzePushLogs(pushLogs io.Reader) error {
	scanner := bufio.NewScanner(pushLogs)
	for scanner.Scan() {
		line := scanner.Text()

		// skip empty lines if any
		if len(line) == 0 {
			continue
		}

		// unmarshal the log line into a buildLogMessage struct
		var logMsg pushLogMessage
		if err := json.Unmarshal([]byte(line), &logMsg); err != nil {
			return fmt.Errorf("error parsing push output: %w", err)
		}

		// check if the log contains an error
		if len(logMsg.Error) > 0 {
			return fmt.Errorf("error during push: %s", logMsg.Error)
		}

		// check if the log contains a denied error detail
		if len(logMsg.ErrorDetail) > 0 {
			return fmt.Errorf("error during push: %s", string(logMsg.ErrorDetail))
		}

		// Optionally check for specific statuses if needed
		if strings.Contains(logMsg.Status, "denied") {
			return fmt.Errorf("access denied during push: %s", logMsg.Status)
		}
	}

	// check for scanner errors
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading push output: %w", err)
	}

	// return nil if there are no errors
	return nil
}

// generateCredentials turns user and password into docker OAuth credentials
func generateCredentials(user, pass string) (string, error) {
	authConfig := registry.AuthConfig{
		Username: user,
		Password: pass,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("error generating credentials: %w", err)
	}

	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}

func retrieveContextBuildFiles(path string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() { // check if the entry is a file
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return []string{}, err
	}

	return files, nil
}
