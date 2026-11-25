package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/builtin"
	"github.com/sunerpy/router-tools/internal/safety/oplock"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application, err := newApplicationService()
	if err != nil {
		log.Fatal(err)
	}
	desktop := NewApp(application)
	if err := wails.Run(&options.App{
		Title:            "Router Tools",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        640,
		BackgroundColour: &options.RGBA{R: 242, G: 243, B: 246, A: 255}, // --rt-canvas (light)
		AssetServer:      &assetserver.Options{Assets: assets},
		Bind:             []any{desktop},
		OnStartup:        desktop.startup,
	}); err != nil {
		log.Fatal(err)
	}
}

func newApplicationService() (*app.Service, error) {
	registry, err := device.NewRegistry(builtin.Adapters()...)
	if err != nil {
		return nil, err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	locker := oplock.FlockLocker{Dir: filepath.Join(configDir, "routerctl", "locks")}
	return app.NewServiceWithLocker(registry, builtin.Discovery(), locker)
}
