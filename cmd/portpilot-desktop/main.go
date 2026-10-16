package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/senpai295/portpilot/internal/desktop"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var embeddedAssets embed.FS

func main() {
	assets, err := fs.Sub(embeddedAssets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	app := desktop.NewApp()

	err = wails.Run(&options.App{
		Title:  "PortPilot",
		Width:  1100,
		Height: 760,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.Startup,
		Bind: []any{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
