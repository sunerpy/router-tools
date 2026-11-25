package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/sunerpy/router-tools/internal/config"
)

func loadCLIConfig(args []string) (config.Config, error) {
	path := ""
	for index, argument := range args {
		switch {
		case argument == "--config":
			if index+1 >= len(args) {
				return config.Config{}, fmt.Errorf("--config requires a path")
			}
			path = args[index+1]
		case strings.HasPrefix(argument, "--config="):
			path = strings.TrimPrefix(argument, "--config=")
		}
	}
	if path == "" {
		path = config.DefaultPath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return config.Default(), nil
		}
	}
	return config.Load(path)
}
