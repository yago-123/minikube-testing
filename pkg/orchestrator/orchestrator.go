package orchestrator

type Orchestrator interface {
	Create(version string, nodes, cpusPerNode, memoryPerNode uint) error
	// todo(): add method for uploading app to save bandwith (there must be some way via docker API)
	// todo(): check minikube command line
	Destroy() error
}
