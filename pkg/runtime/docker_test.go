package runtime

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildImageWithoutErrors(t *testing.T) {
	dock, err := NewDockerController("credentials")
	require.NoError(t, err)

	dockerfile, err := os.ReadFile("_fixture/Dockerfile")
	require.NoError(t, err)

	filesBuildContext := []string{"_fixture/main.go", "_fixture/go.mod", "_fixture/go.sum"}
	err = dock.BuildImage(context.Background(), "my-image", "latest", dockerfile, filesBuildContext)
	require.NoError(t, err)
}

func TestBuildImageWithoutBuildContext(t *testing.T) {
	dock, err := NewDockerController("credentials")
	require.NoError(t, err)

	dockerfile, err := os.ReadFile("_fixture/Dockerfile")
	require.NoError(t, err)

	err = dock.BuildImage(context.Background(), "my-image", "latest", dockerfile, []string{})
	require.Error(t, err)
}

func TestAnalyzeBuildLogs(t *testing.T) {
	tests := []struct {
		name    string
		logs    string
		wantErr bool
	}{
		{
			name: "Valid JSON objects",
			logs: `{"stream":"Step 1/2 : FROM busybox\n"}
                      {"stream":"Step 2/2 : CMD echo Hello, Docker!\n"}`,
			wantErr: false,
		},
		{
			name: "Malformed JSON (missing closing bracket)",
			logs: `{"stream":"Step 1/2 : FROM busybox\n"}
                      {"stream":"This is not a valid JSON"`,
			wantErr: true,
		},
		{
			name: "Valid JSON with extra data",
			logs: `{"stream":"Step 1/2 : FROM busybox\n"}
                      {"stream":"Step 2/2 : CMD echo Hello, Docker!\n"}
                      {"stream":"Extra line that is valid JSON"}`,
			wantErr: false,
		},
		{
			name: "Valid JSON with error detail",
			logs: `{"stream":"Step 1/2 : FROM busybox\n"}
                      {"stream":"Error detail","ErrorDetail":"{\"message\":\"Build failed\"}"}`,
			wantErr: true,
		},
		{
			name:    "Completely empty input",
			logs:    ``,
			wantErr: false,
		},
		{
			name:    "Single line JSON object",
			logs:    `{"stream":"Single line JSON object"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader([]byte(tt.logs))
			err := analyzeBuildLogs(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("analyzeBuildLogs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
