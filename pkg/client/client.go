package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	PodWaitTime        = 500 * time.Millisecond
	PortForwardLocal   = 8080
	PortForwardTimeout = 20 * time.Second
)

type Client interface {
	CurlEndpoint(ctx context.Context, url string) (string, error)
	CurlServiceEndpoint(ctx context.Context, url string) error

	ClientSet() *kubernetes.Clientset
}

type k8sClient struct {
	cs         *kubernetes.Clientset
	restConfig *rest.Config
}

func NewClient() (*k8sClient, error) {
	cs, restConfig, err := loadKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading kubeconfig: %w", err)
	}

	return &k8sClient{
		cs:         cs,
		restConfig: restConfig,
	}, nil
}

func (c *k8sClient) CurlEndpoint(ctx context.Context, namespace, podName string, podPort uint, path string) (*http.Response, error) {
	stopChan, err := c.portForward(ctx, namespace, podName, podPort)
	if err != nil {
		return nil, fmt.Errorf("error setting up port forwarding: %w", err)
	}
	defer close(stopChan)

	return http.Get(fmt.Sprintf("http://localhost:%d/%s", PortForwardLocal, path))
}

func (c *k8sClient) CurlServiceEndpoint(ctx context.Context, url string) error {
	// curl http://my-service.namespace-b.svc.cluster.local
	return nil
}

func (c *k8sClient) ClientSet() *kubernetes.Clientset {
	return c.cs
}

// loadKubeConfig loads the kubeconfig from ${HOME}/.kube/config
func loadKubeConfig() (*kubernetes.Clientset, *rest.Config, error) {
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
		return nil, nil, fmt.Errorf("error building config from flags: %w", err)
	}

	// initialize Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error initializing Kubernetes client: %w", err)
	}

	return client, config, nil
}

// waitForPodLogs waits for the pod to
func (c *k8sClient) waitForPod(ctx context.Context, pod *v1.Pod) error {
	for {
		time.Sleep(PodWaitTime)

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

// portForward sets up port forwarding to the pod
func (c *k8sClient) portForward(ctx context.Context, namespace, podName string, podPort uint) (chan struct{}, error) {
	// create a round tripper
	roundTripper, upgrader, err := spdy.RoundTripperFor(c.restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating round tripper: %w", err)
	}

	// build URL for port forwarding
	req := c.cs.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	// create the port forwarding dialer
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, "POST", req.URL())

	// channels for managing the port forward lifecycle
	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)

	out, errOut := new(strings.Builder), new(strings.Builder)

	// specify the ports to forward
	ports := []string{fmt.Sprintf("%d:%d", PortForwardLocal, podPort)}

	// create a channel to capture errors from the goroutine
	errChan := make(chan error, 1)

	// create the port forwarder
	pf, err := portforward.New(dialer, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return nil, fmt.Errorf("error setting up port forwarding: %w", err)
	}

	// start port forwarding in a separate goroutine
	go func() {
		if errForward := pf.ForwardPorts(); errForward != nil {
			errChan <- fmt.Errorf("failed to forward ports: %w", errForward)
		}
		close(errChan)
	}()

	// use a select statement to wait for either readyChan or timeout
	select {
	case <-ctx.Done():
		// context is done, stop port forwarding
		close(stopChan)
		return nil, ctx.Err()
	case <-time.After(PortForwardTimeout):
		// timeout occurred, stop port forwarding
		close(stopChan)
		return nil, fmt.Errorf("port forwarding timed out after %v", PortForwardTimeout)
	case errC := <-errChan:
		// port forwarding encountered an error
		return nil, fmt.Errorf("error starting port forwarding: %w", errC)
	case <-readyChan:
		// port forwarding is ready
		return stopChan, nil
	}
}
