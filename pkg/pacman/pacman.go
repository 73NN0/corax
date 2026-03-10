package pacman

import (
	"os"
	"os/exec"

	"github.com/73NN0/corax/pkg/config"
)

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Update() error { // TODO noconfirm not needed when called from user explicitly
	return run("pacman", "-Syu", "--noconfirm")
}

func Install(pkgs ...string) error {
	return run("pacman", append([]string{"-S", "--needed"}, pkgs...)...)
}

func setup() error {
	if err := run("pacman-key", "--init"); err != nil {
		return err
	}
	if err := run("pacman", "-Syu", "--noconfirm"); err != nil {
		return err
	}
	if err := run("pacman", "-S", "--noconfirm", "--needed", "sudo"); err != nil {
		return err
	}

	return nil
}

func Bootstrap(_ config.Config) error {
	if err := run("pacman", "-Syu", "--noconfirm"); err != nil {
		return err
	}
	if err := setup(); err != nil {
		return err
	}
	return Install(
		"base-level",
		"git",
		"curl",
		"go",
		"go-tools",
		"gopls",
		"staiccheck",
		"gcc",
	)
}
