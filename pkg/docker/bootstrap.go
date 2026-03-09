package docker

import (
	"fmt"

	"github.com/73NN0/corax/pkg/config"
)

func Bootstrap(cfg config.Config, args []string) error {
	for _, step := range cfg.Bootstrap.Steps {
		fmt.Printf("→ %s\n", step.Name)
		if err := step.Run(cfg); err != nil {
			return fmt.Errorf("step %s failed: %w", step.Name, err)
		}
	}
	return nil
}
