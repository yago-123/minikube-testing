package orchestrator

type Orchestrator interface {
	Create(version string, nodes, cpusPerNode, memoryPerNode uint) error
	// todo(): add method for uploading app to save bandwith (there must be some way via Minikube API)
	// todo(): check minikube command line
	LoadImage(image string) error
	Delete() error
}
