package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/73NN0/corax/pkg/config"
)

func die(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, BRed+msg+Coff)
	os.Exit(1)
}

func usage(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, BYellow+msg+Coff)
}

func init() {
	commands["help"] = map[string]command{
		"main": helpMain,
	}
}

func helpMain(_ config.Config, args []string) error {
	fmt.Println(Yellow + "usage: corax <modules> [args]" + Coff)
	fmt.Println(BYellow + "modules:" + Coff)
	for name := range commands {
		fmt.Printf(" %s%s%s\n", BYellow, name, Coff)
	}
	return nil
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {

		if _, err := os.Stat(filepath.Join(dir, projectMarker)); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)

		if parent == dir {
			return "", fmt.Errorf("not in a project")
		}

		dir = parent
	}
}

func main() {
	root, err := projectRoot()

	if err != nil {
		die("%s", err)
	}

	Cfg.Root = root

	args := os.Args[1:]

	module := "help"
	method := "main"

	if len(args) >= 1 {
		module = args[0]
		// create a slice from 1
		args = args[1:]
	}
	// if their is a list of args so the arg at emplacement 2 is the methode the rest the args
	if len(args) >= 1 {
		method = args[0]
		args = args[1:]
	}

	mod, ok := commands[module]
	if !ok {
		die("module %s not found", module)
	}

	fn, ok := mod[method]
	if !ok {
		fn, ok = mod["main"]

		if !ok {
			die("module %s method %s not found\n", module, method)
		}

		args = append([]string{method}, args...)
	}

	if err := fn(Cfg, args); err != nil {
		die("%s", err)
	}
}
