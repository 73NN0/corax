package docker

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/73NN0/corax/pkg/config"
)

func setupUser(cfg config.Config) error {
	uid := strconv.Itoa(cfg.User.UID)
	gid := strconv.Itoa(cfg.User.GID)

	if err := run("groupadd", "-g", gid, cfg.User.Name); err != nil {
		return fmt.Errorf("groupadd: %w", err)
	}

	if err := run("useradd",
		"-r",
		"-u", uid,
		"-g", cfg.User.Name,
		"-G", "sudo",
		"-d", cfg.Docker.ContainerTarget,
		cfg.User.Name,
	); err != nil {
		return fmt.Errorf("useradd: %w", err)
	}

	// write sudoers rule via visudo for syntax validation
	// writes to /etc/sudoers.d/<user> instead of touching /etc/sudoers directly
	sudoers := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: ALL\n", cfg.User.Name)
	cmd := exec.Command("visudo", "--stdin", "-f", "/etc/sudoers.d/"+cfg.User.Name)
	cmd.Stdin = strings.NewReader(sudoers)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("visudo: %w", err)
	}

	return nil
}
