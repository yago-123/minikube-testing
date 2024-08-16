package docker

type Docker interface {
	BuildImage()
	PushImage()
}
