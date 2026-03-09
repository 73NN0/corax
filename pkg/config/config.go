package config

type Docker struct {
	DockerfileFolder string
	DockerfilePrefix string
	NetworkName      string
	ContainerTarget  string
	ContainerUser    string
}

type Step struct {
	Name string
	Run  func(cfg Config) error
}

type Bootstrap struct {
	Steps []Step
}

type Config struct {
	Docker    Docker
	Bootstrap Bootstrap
	Root      string
}
