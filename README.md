# Minikube testing
Package designed for running E2E and integration tests over Minikube

## Docker
Building and pushing images don’t have a clear error pattern (e.g., the image failed to be pushed); there are no errors 
being returned from the functions themselves. To check the status of the operation, the output logs must be parsed. If 
you know a better way to handle this, please open an issue or submit a PR. :)