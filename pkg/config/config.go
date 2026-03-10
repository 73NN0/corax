package config

type Docker struct {
	DockerfileFolder string
	DockerfilePrefix string
	NetworkName      string
	ContainerTarget  string
}

type User struct {
	Name string
	UID  int
	GID  int
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
	User      User
	Root      string
}
