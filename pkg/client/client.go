package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/yaml"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	PodWaitTime        = 500 * time.Millisecond
	PortForwardLocal   = 9191
	PortForwardTimeout = 20 * time.Second

	DefaultNamespace      = "default"
	HelmDriverEnvVariable = "HELM_DRIVER"
)

type Client interface {
	GetPod(ctx context.Context, podName, namespace string) (*v1.Pod, error)
	GetPodLogs(ctx context.Context, pod *v1.Pod) ([]byte, error)

	RunYAML(ctx context.Context, yamlManifest []byte) error
	DeployHelmWithLocalChart(chartPath string, ns, release string, args map[string]interface{}) error
	DeployHelmWithRemoteChart() error

	CurlPod(ctx context.Context, pod *v1.Pod, podPort uint, path string) (*http.Response, error)
	CurlService(ctx context.Context, url string) error

	ClientSet() *kubernetes.Clientset
}

type K8sClient struct {
	cs        *kubernetes.Clientset
	dynClient *dynamic.DynamicClient
	config    *rest.Config

	settings     *cli.EnvSettings
	actionConfig *action.Configuration
}

func NewClient() (*K8sClient, error) {
	config, err := loadKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading kubeconfig: %w", err)
	}

	// initialize Kubernetes client
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error initializing Kubernetes client: %w", err)
	}

	// initialize dynamic client
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error initializing dynamic client: %w", err)
	}

	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err = actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv(HelmDriverEnvVariable), nil); err != nil {
		return nil, fmt.Errorf("error initializing action config: %w", err)
	}

	return &K8sClient{
		cs:           cs,
		dynClient:    dynClient,
		config:       config,
		settings:     settings,
		actionConfig: actionConfig,
	}, nil
}

func (c *K8sClient) GetPod(ctx context.Context, podName, namespace string) (*v1.Pod, error) {
	pod, err := c.cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting pod %s: %w", podName, err)
	}

	return pod, nil
}

func (c *K8sClient) GetPodLogs(ctx context.Context, pod *v1.Pod) ([]byte, error) {
	// todo(): add additional method that retrieve logs once the pod have stopped running
	return c.retrieveLogsFromPod(ctx, pod)
}

func (c *K8sClient) CreatePod() error {
	return nil
}

func (c *K8sClient) DeployHelmWithLocalChart(chartPath string, ns, release string, args map[string]interface{}) error {
	// load the Helm chart
	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("error loading chart: %w", err)
	}

	// create install action
	install := action.NewInstall(c.actionConfig)
	install.ReleaseName = release
	install.Namespace = ns

	// install the chart
	_, err = install.Run(chart, args)
	if err != nil {
		return fmt.Errorf("error during chart installation: %w", err)
	}

	return nil
}

func (c *K8sClient) DeployHelmWithRemoteChart() error {
	return nil
}

func (c *K8sClient) RunYAML(ctx context.Context, yamlManifest []byte) error {
	// convert YAML to Unstructured
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal(yamlManifest, &obj); err != nil {
		return fmt.Errorf("error unmarshalling YAML: %w", err)
	}

	// create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(c.config)
	if err != nil {
		return fmt.Errorf("error creating discovery client: %w", err)
	}

	// discover API resources
	_, apiResources, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return fmt.Errorf("error retrieving server resources: %w", err)
	}

	// map API resources to GVR
	gvr, err := findGVR(apiResources, obj.GroupVersionKind())
	if err != nil {
		return fmt.Errorf("error finding group version kind: %w", err)
	}

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = DefaultNamespace
	}

	// create the resource
	_, err = c.dynClient.
		Resource(gvr).
		Namespace(namespace).
		Create(ctx, &obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating resource: %w", err)
	}

	return nil
}

func (c *K8sClient) CurlPod(ctx context.Context, pod *v1.Pod, podPort uint, path string) (*http.Response, error) {
	stopChan, err := c.portForward(ctx, pod.Namespace, pod.Name, podPort)
	if err != nil {
		return nil, fmt.Errorf("error setting up port forwarding: %w", err)
	}
	defer close(stopChan)

	return http.Get(fmt.Sprintf("http://localhost:%d/%s", PortForwardLocal, path))
}

func (c *K8sClient) CurlService(ctx context.Context, url string) error {
	// curl http://my-service.namespace-b.svc.cluster.local
	return nil
}

func (c *K8sClient) ClientSet() *kubernetes.Clientset {
	return c.cs
}

// loadKubeConfig loads the kubeconfig from ${HOME}/.kube/config
func loadKubeConfig() (*rest.Config, error) {
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

	return config, nil
}

// waitForPod waits for the pod to finish running
func (c *K8sClient) waitForPod(ctx context.Context, pod *v1.Pod) error {
	for {
		// todo(): do more sophisticated wait for pod (Kubernetes API probably have watchers)
		time.Sleep(PodWaitTime)

		select {
		case <-ctx.Done():
			return ctx.Err() // break the loop and return if the context is done.
		default:
			// proceed with checking the pod status
			p, errGet := c.cs.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
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
func (c *K8sClient) retrieveLogsFromPod(ctx context.Context, pod *v1.Pod) ([]byte, error) {
	// retrieve logs
	podLogOpts := v1.PodLogOptions{}
	req := c.cs.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
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
func (c *K8sClient) portForward(ctx context.Context, namespace, podName string, podPort uint) (chan struct{}, error) {
	// create a round tripper
	roundTripper, upgrader, err := spdy.RoundTripperFor(c.config)
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

// findGVR returns the GroupVersionResource for the given GVK
func findGVR(apiResources []*metav1.APIResourceList, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	for _, resourceList := range apiResources {
		for _, resource := range resourceList.APIResources {
			if resource.Kind == gvk.Kind && resourceList.GroupVersion == gvk.GroupVersion().String() {
				return schema.GroupVersionResource{
					Group:    gvk.Group,
					Version:  gvk.Version,
					Resource: resource.Name, // Correct field to use
				}, nil
			}
		}
	}
	return schema.GroupVersionResource{}, fmt.Errorf("GVR for %s %s not found", gvk.GroupVersion().String(), gvk.Kind)
}
