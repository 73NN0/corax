package docker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	wslSocket     string = "/mnt/wsl/shared-docker/docker.sock"
	defaultSocket string = "/var/run/docker.sock"
)

// detectSocket returns the Docker deamon socket path.
// On a native Linux system, Dokcer socket is always at /var/run/docker.sock
//
// On WSL, Docker Desktop for windows exposes a shared sockert at /mnt/wsl/shared-docker/docker.sock instead.
// This allows WSL to communicate with the docker deamon running on Windows
//
// We check WSL path first - fi it exists, we are on WSL.
func detectSocket() string {
	if _, err := os.Stat(wslSocket); err == nil {
		return wslSocket
	}

	return defaultSocket
}

// detectHostHome returns the host home directory
//
// on native linux, HOME is the actual host home directory
//
// on WSL, HOME is the linux path ( /home/user) but Docker Desktop
// runs on Windows and understand only windows-mapped paths
// (/mnt/c/Users/user). we check if HOST_HOME is already set in
// the environment ( set by the user in their .bashrc/.zshrc)
// and fall back to HOME if not.
//
// The user should set HOST_HOME in their shell config on WSL:
// export HOST_HOME=/mnt/c/users/yourname
func detectHostHome(isWSL bool) string {
	// if explicitly set in env trust it
	if h := os.Getenv("HOST_HOME"); h != "" {
		return h
	}

	// on WSL without HOST_HOME, warn the user
	if isWSL {
		fmt.Fprintln(os.Stderr, "warning: WSL detected but HOST_HOME is not set, bind mount may fail")
		fmt.Fprintln(os.Stderr, "hint: add 'export HOST_HOME=/mnt/c/Users/yourname' to your shell config")
	}

	return os.Getenv("HOME")
}

// isWSL returns true if running inside WSL
//
// /proc/version contains "microsoft" or "WSL" on WSL systems
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}

	content := strings.ToLower(string(data))
	return strings.Contains(content, "microsoft") || strings.Contains(content, "wsl")
}

// networkEnsure creates a Docker bridge network if it does not already exist.
//
// A britge network allows containers to communicate with each other by name.
// for example, a container named relay can be reached at
// http://network-relay:3128 from any container on the same network.
func networkEnsure(name string) error {
	// exec.Command ( and not run()) because we only want to check the exit code, not the GIANT json ^^
	cmd := exec.Command("docker", "network", "inspect", name)
	if err := cmd.Run(); err == nil {
		// network already exists
		return nil
	}

	fmt.Printf("creating docker network %s\n", name)
	return run("docker", "network", "create", "--driver", "bridge", name)
}

const (
	gidsize   int = 3
	gidIDpart int = 2
)

// hostDockerGID returns the GID of the docker group on the host.
// /etc/group format is colon-separated:
// groupname:password:GID:members
// example: docker:x:998:alice,bob
//
// we need this GID to add the container user to a group with the same GID, so it can access the docker socket mounted
// from the host.
// the socket is owned by the docker group on the host - matching the GID inside the container grants access without running as root.
func hostDockerGID() (string, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return "", fmt.Errorf("failed to open /etc/group: %w", err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// looking for a line starting with "docker:"
		if !strings.HasPrefix(line, "docker:") {
			continue
		}

		// docker:x:998:alice,bob
		//		  ^  ^
		// [0]	 [1][2] [3]
		parts := strings.Split(line, ":")
		if len(parts) < gidsize {
			return "", fmt.Errorf("unexpected /etc/group format: %s", line)
		}
		return parts[gidIDpart], nil
	}

	return "", fmt.Errorf("docker group not found in /etc/group")
}

const (
	passwdsize = 4
)

// patchPasswd replaces the uid and gid of the container uer in /etc/passwd.
//
// /etc/passwd format:
// username:password:UID:GID:comment:home:shell
// example : foo:x:1000:1000::/opt/apps:/bin/bash
//
// the image hardcordes uid=1000 for the container user. We replace it
// with teh host user uild/gid so files created in teh bind mount are owned by the host user.
func patchPasswd(path, uid, gid, containerUser string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read passwd: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, containerUser+":") {
			continue
		}
		// mcs:x:1000:1000::/opt/apps:/bin/bash
		parts := strings.Split(line, ":")
		if len(parts) < passwdsize {
			return fmt.Errorf("unexpected passwd format: %s", line)
		}

		// replace uid and gid

		parts[2] = uid
		parts[3] = gid
		lines[i] = strings.Join(parts, ":")
		break
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// patchGroup replaces the gid of the container user in /etc/group
// and adds the container user to the docker group
//
// /etc/group format:
// groupname:password:GID:members
// example:mcs:x:1000
//
// Two operations :
// 1/ replace mcs gid with the host user gid - ownership consistency
// 2/ add mcs to docker group with host docker gid - socket access
func patchGroup(path, gid, dockerGID, containerUser string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read group: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, containerUser+":") {
			continue
		}

		//mcs:x:1000:
		parts := strings.Split(line, ":")
		if len(parts) < gidsize {
			return fmt.Errorf("unexpected group format: %s", line)
		}

		// replace gid
		parts[2] = gid
		lines[i] = strings.Join(parts, ":")
		break
	}

	// add container user to the docker group with host docker gid
	// docker:x:998:mcs
	lines = append(lines, fmt.Sprintf("docker:x:%s:%s", dockerGID, containerUser))

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// patchIDs updates the container user uid/gid to match the host user.
//
// Flow:
// 1. copy /etc/passwd and /etc/group from container to a temp dir
// 2. patch uid/gid of container user in passwd
// 3. patch gid of container user in group + add docker group
// 4. copy patched files back to container
//
// This is necessary because the image hardcodes uid=1000 for the
// container user. We need to match the host user uid/gid so files
// created in the bind mount are owned by the host user.
func patchIDs(name, uid, gid, containerUser string) error {
	// get host docker group gid before touching anything
	dockerGID, err := hostDockerGID()
	if err != nil {
		return fmt.Errorf("failed to get docker group gid: %w", err)
	}

	// create temp dir on host
	tmp, err := os.MkdirTemp("", "corax-patch-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	passwdPath := filepath.Join(tmp, "passwd")
	groupPath := filepath.Join(tmp, "group")

	// copy from container to host
	if err := run("docker", "cp", name+":/etc/passwd", passwdPath); err != nil {
		return fmt.Errorf("failed to copy passwd from container: %w", err)
	}
	if err := run("docker", "cp", name+":/etc/group", groupPath); err != nil {
		return fmt.Errorf("failed to copy group from container: %w", err)
	}

	// patch
	if err := patchPasswd(passwdPath, uid, gid, containerUser); err != nil {
		return fmt.Errorf("failed to patch passwd: %w", err)
	}
	if err := patchGroup(groupPath, gid, dockerGID, containerUser); err != nil {
		return fmt.Errorf("failed to patch group: %w", err)
	}

	// copy back to container
	if err := run("docker", "cp", passwdPath, name+":/etc/passwd"); err != nil {
		return fmt.Errorf("failed to restore passwd to container: %w", err)
	}
	if err := run("docker", "cp", groupPath, name+":/etc/group"); err != nil {
		return fmt.Errorf("failed to restore group to container: %w", err)
	}

	return nil
}

// Linux natif :
//     HOME      = /home/toi
//     HOST_HOME = /home/toi   (identiques)
//     appDir    = cfg.Root    (pas de remplacement)

// WSL, projet sous HOME :
//     HOME      = /home/toi
//     HOST_HOME = /mnt/c/Users/toi
//     cfg.Root  = /home/toi/mon_projet
//     appDir    = /mnt/c/Users/toi/mon_projet  (remplacement)

// WSL, projet sous /opt :
//
//	HOME      = /home/toi
//	HOST_HOME = /mnt/c/Users/toi
//	cfg.Root  = /opt/mon_projet
//	appDir    = /opt/mon_projet  (pas de remplacement, pas sous HOME)

func getAppDir(root, hostHome string) (appDir string) {

	// on WSL, paths under /home are mapped to /mnt/c/Users on Windows side
	// but paths like /opt or /workspace are not affected
	// the user must set HOST_HOME correctly in their environment
	// if their project is under HOME — otherwise cfg.Root is used as-is
	appDir = root
	if hostHome != os.Getenv("HOME") {
		// we are on WSL and HOST_HOME differs from HOME
		// only replace if project is actually under HOME
		if strings.HasPrefix(root, os.Getenv("HOME")) {
			appDir = strings.Replace(root, os.Getenv("HOME"), hostHome, 1)
		}
	}

	return
}
