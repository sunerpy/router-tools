package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/buildinfo"
	"github.com/sunerpy/router-tools/internal/cli"
	"github.com/sunerpy/router-tools/internal/device"
	rd08v1196 "github.com/sunerpy/router-tools/internal/devices/xiaomi/rd08/v1_1_96"
	"github.com/sunerpy/router-tools/internal/safety/oplock"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	driver := rd08v1196.New()
	registry, err := device.NewRegistry(driver)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: initialize device registry: %v\n", err)
		os.Exit(12)
	}
	configDir, configErr := os.UserConfigDir()
	if configErr != nil {
		configDir = "."
	}
	locker := oplock.FlockLocker{Dir: filepath.Join(configDir, "routerctl", "locks")}
	service, err := app.NewServiceWithLocker(registry, rd08v1196.NewProber(nil), locker)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: initialize application: %v\n", err)
		os.Exit(12)
	}
	os.Exit(cli.ExecuteWithInput(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, service, buildinfo.Current()))
}
