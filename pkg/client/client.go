package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const PodWaitTime = 500 * time.Millisecond

type Client interface {
	CurlEndpoint(ctx context.Context, url string) (string, error)
	CurlServiceEndpoint(ctx context.Context, url string) error

	ClientSet() *kubernetes.Clientset
}

type ResponseData struct {
	Status   string          `json:"status"`
	Response json.RawMessage `json:"response"`
}

type k8sClient struct {
	cs       *kubernetes.Clientset
	waitTime time.Duration
}

func NewClient() (*k8sClient, error) {
	cs, err := loadKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading kubeconfig: %w", err)
	}

	return &k8sClient{
		cs:       cs,
		waitTime: PodWaitTime,
	}, nil
}

func (c *k8sClient) CurlEndpoint(ctx context.Context, endpoint string) (string, error) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "http-client",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "curl-container",
					Image: "yagoninja/mini-curl:latest", // todo(): use versioning here
					Command: []string{
						"sh", "-c", "response=$(curl -s \"$ENDPOINT_URL\"); echo '{\"status\":\"success\",\"response\":'\"$response\"'}'",
					},
					Env: []v1.EnvVar{
						{
							Name:  "ENDPOINT_URL",
							Value: endpoint, // pass the endpoint URL dynamically
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	// create the Pod in the Kubernetes cluster
	namespace := "default"
	_, err := c.cs.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("error creating pod: %w", err)
	}

	// wait for pod to complete
	if err = c.waitForPod(ctx, pod); err != nil {
		return "", fmt.Errorf("error waiting for pod: %w", err)
	}

	// retrieve logs from the pod
	logs, err := c.retrieveLogsFromPod(ctx, pod)
	if err != nil {
		return "", fmt.Errorf("error retrieving logs from pod: %w", err)
	}

	// decode logs
	var responseData ResponseData
	fmt.Println("current logs: %s", string(logs))
	err = json.Unmarshal(logs, &responseData)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	fmt.Printf("Status: %s\nResponse: %s\n", responseData.Status, responseData.Response)

	// delete pod created
	err = c.cs.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	if err != nil {
		panic(err.Error())
	}

	return string(logs), nil
}

func (c *k8sClient) CurlServiceEndpoint(ctx context.Context, url string) error {
	// curl http://my-service.namespace-b.svc.cluster.local
	return nil
}

func (c *k8sClient) ClientSet() *kubernetes.Clientset {
	return c.cs
}

// loadKubeConfig loads the kubeconfig from ${HOME}/.kube/config
func loadKubeConfig() (*kubernetes.Clientset, error) {
	// access kubeconfig file
	kubeconfigPath := ""
	if home := homedir.HomeDir(); home != "" {
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	} else if home == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
	}

	// building config from flags
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("error building config from flags: %w", err)
	}

	// initialize Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error initializing Kubernetes client: %w", err)
	}

	return client, nil
}

// waitForPodLogs waits for the pod to
func (c *k8sClient) waitForPod(ctx context.Context, pod *v1.Pod) error {
	for {
		time.Sleep(c.waitTime)

		select {
		case <-ctx.Done():
			return ctx.Err() // break the loop and return if the context is done.
		default:
			// proceed with checking the pod status
			p, errGet := c.cs.CoreV1().Pods("default").Get(ctx, pod.Name, metav1.GetOptions{})
			if errGet != nil {
				return fmt.Errorf("error getting pod %s: %w", pod.Name, errGet)
			}

			// in case pod finished, then break the loop
			if p.Status.Phase == v1.PodSucceeded || p.Status.Phase == v1.PodFailed {
				return nil
			}
		}
	}
}

// retrieveLogsFromPod retrieves the logs from the pod
func (c *k8sClient) retrieveLogsFromPod(ctx context.Context, pod *v1.Pod) ([]byte, error) {
	podLogOpts := v1.PodLogOptions{}
	// retrieve logs
	req := c.cs.CoreV1().Pods("default").GetLogs(pod.Name, &podLogOpts)
	logs, err := req.Stream(ctx)
	if err != nil {
		return []byte{}, err
	}
	defer logs.Close()

	// copy logs to buffer
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, logs)
	if err != nil {
		return []byte{}, err
	}

	return buf.Bytes(), nil
}
