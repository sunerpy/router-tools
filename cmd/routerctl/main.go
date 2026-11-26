package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/buildinfo"
	"github.com/sunerpy/router-tools/internal/cli"
	"github.com/sunerpy/router-tools/internal/wiring"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	paths, err := wiring.DefaultPaths()
	if err == nil {
		var service *app.Service
		service, err = wiring.NewService(paths)
		if err == nil {
			os.Exit(cli.ExecuteWithInput(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, service, buildinfo.Current()))
		}
	}
	_, _ = fmt.Fprintf(os.Stderr, "Error: initialize application: %v\n", err)
	os.Exit(12)
}
