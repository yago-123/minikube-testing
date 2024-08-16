package client

type Client interface {
	HitEndpoint(url string) error
}
