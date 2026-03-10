package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/73NN0/corax/pkg/config"
)

func usage() {
	fmt.Println("usage: corax docker <command> [args]")
	fmt.Println("commands:")
	fmt.Println("  build <template> <name> [app]   build an image from template")
	fmt.Println("  create <template> <image> <name> create a container")
	fmt.Println("  start <name>                     start a container")
	fmt.Println("  stop <name>                      stop a container")
	fmt.Println("  enter <name>                     enter a container shell")
}

func Help() {
	usage()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Build(cfg config.Config, args []string) error {
	n := len(args)
	if n < 2 {
		return fmt.Errorf("usage: corax docker build <template> <name> [app]")
	}

	tmpl := args[0]
	name := args[1]
	app := ""

	if n >= 3 {
		app = args[2]
	}
	// TODO : build in temp dir ?
	// dockerfilePrefix+tmpl => ex : Dockerfile.dev || Dockerfile.prod
	dockerfile := filepath.Join(cfg.Root, cfg.Docker.DockerfileFolder, cfg.Docker.DockerfilePrefix+tmpl)

	if _, err := os.Stat(dockerfile); err != nil {
		return fmt.Errorf("Dockerfile not found for template %s: %w", tmpl, err)
	}

	return run("docker", "build",
		"-f", dockerfile,
		"-t", name,
		"--progress=plain",
		"--build-arg", "app="+app,
		cfg.Root,
	)
}

//

// Create creates a new development container from the given image.
//
// Flow:
//
// 1. detect environment (WSL vs Linux, docker socket path)
//
// 2. ensure dev network exists
//
// 3. create container with proper mounts and capabilities
//
// 4. patch uid/gid in container to match host user
func Create(cfg config.Config, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: corax docker create <image> <name>")
	}

	image := args[0]
	name := args[1]

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	uid := fmt.Sprintf("%d", os.Getuid())
	gid := fmt.Sprintf("%d", os.Getgid())

	socketPath := detectSocket()
	isWSL := isWSL()
	hostHome := detectHostHome(isWSL)

	appDir := getAppDir(cfg.Root, hostHome)

	if err := networkEnsure(cfg.Docker.NetworkName); err != nil {
		return fmt.Errorf("failed to ensure network: %w", err)
	}

	if err := run("docker", "create",
		"-it",
		"-h", hostname,
		"--user", uid,
		"--label", "mcapp.type=dev",
		"--cap-add=SYS_PTRACE",
		"--security-opt", "seccomp=unconfined",
		"--name", name,
		"--network", cfg.Docker.NetworkName,
		"-e", "HOST_HOME="+hostHome,
		"-e", "DOCKER_SOCKET_PATH="+socketPath,
		// 					  host				container
		"--mount", "type=bind,source="+appDir+",target="+cfg.Docker.ContainerTarget,
		"--mount", "type=bind,source="+socketPath+",target=/var/run/docker.sock",
		image,
	); err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	return patchIDs(name, uid, gid, cfg.User.Name)
}

// Start starts an existing container.
func Start(cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: corax docker start <name>")
	}
	return run("docker", "start", args[0])
}

// Stop stops a running container.
func Stop(cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: corax docker stop <name>")
	}
	return run("docker", "stop", args[0])
}

// Restart stops and starts a container.
func Restart(cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: corax docker restart <name>")
	}
	if err := run("docker", "stop", args[0]); err != nil {
		return err
	}
	return run("docker", "start", args[0])
}

// Enter opens an interactive shell in a running container.
//
// Uses the label mcapp.type to find the right shell command.
// dev containers use /bin/bash, prod containers may differ.
func Enter(cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: corax docker enter <name>")
	}
	return run("docker", "exec", "-it", args[0], "/bin/bash")
}

// Execute runs a corax command inside a running container.
//
// Equivalent of docker::container::execute in the bash version.
// Allows running corax commands from the host inside the container.
func Execute(cfg config.Config, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: corax docker execute <name> <command> [args]")
	}
	name := args[0]
	corax := filepath.Join(cfg.Docker.ContainerTarget, "corax")
	cmdArgs := append([]string{"exec", "-it", name, "/bin/bash", corax}, args[1:]...)
	return run("docker", cmdArgs...)
}

func User(cfg config.Config, _ []string) error {
	return setupUser(cfg)
}

func Main(cfg config.Config, args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("no command name passed")
	}

	switch args[0] {
	case "build":
		return Build(cfg, args[1:])
	case "create":
		return Create(cfg, args[1:])
	case "user":
		return User(cfg, args[1:])
	case "execute":
		return Execute(cfg, args[1:])
	default:
		return fmt.Errorf("unknown: %s", args[0])
	}
}
