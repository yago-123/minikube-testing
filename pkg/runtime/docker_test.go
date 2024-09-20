package runtime

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildImageWithoutErrors(t *testing.T) {
	dock, err := NewDockerControllerWithCreds("", "")
	require.NoError(t, err)

	dockerfile, err := os.ReadFile("_fixture/Dockerfile")
	require.NoError(t, err)

	filesBuildContext := []string{"_fixture/main.go", "_fixture/go.mod", "_fixture/go.sum"}
	err = dock.BuildImage(context.Background(), "my-image", "latest", dockerfile, filesBuildContext)
	require.NoError(t, err)
}

func TestBuildImageWithoutBuildContext(t *testing.T) {
	dock, err := NewDockerControllerWithCreds("", "")
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

func TestAnalyzePushOutput(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedError    bool
		expectedErrorMsg string
	}{
		{
			name: "Successful Push",
			input: `{"status":"The push refers to repository [docker.io/yagoninja/minikube-testing]"}
{"status":"Preparing","progressDetail":{},"id":"024f7860c664"}
{"status":"Preparing","progressDetail":{},"id":"77d8e7d826a0"}
{"status":"Pushed","progressDetail":{},"id":"b760cbb380ed"}
{"status":"latest2: digest: sha256:bbfa2f4b50110e673b086a8118fdd4f241d7890dcf8d4cba2d86516ea9cbcb55 size: 1152"}
`,
			expectedError: false,
		},
		{
			name: "Access Denied Error",
			input: `{"status":"The push refers to repository [docker.io/yagoninja/minikube-testing]"}
{"status":"Preparing","progressDetail":{},"id":"024f7860c664"}
{"errorDetail":{"message":"denied: requested access to the resource is denied"},"error":"denied: requested access to the resource is denied"}
`,
			expectedError:    true,
			expectedErrorMsg: "error during push: denied: requested access to the resource is denied",
		},
		{
			name: "Generic Error",
			input: `{"status":"The push refers to repository [docker.io/yagoninja/minikube-testing]"}
{"status":"Preparing","progressDetail":{},"id":"024f7860c664"}
{"error":"some generic error occurred"}
`,
			expectedError:    true,
			expectedErrorMsg: "error during push: some generic error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBufferString(tt.input)
			err := analyzePushLogs(buf)

			if (err != nil) != tt.expectedError {
				t.Errorf("analyzePushOutput() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if err != nil && !strings.Contains(err.Error(), tt.expectedErrorMsg) {
				t.Errorf("analyzePushOutput() error = %v, expectedErrorMsg %v", err, tt.expectedErrorMsg)
			}
		})
	}
}
