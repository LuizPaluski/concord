package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := &App{releasesURL: latestRelease}
	err := wails.Run(&options.App{
		Title:            "Concord",
		Width:            1200,
		Height:           780,
		MinWidth:         760,
		MinHeight:        500,
		Frameless:        true,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 30, G: 31, B: 34, A: 255},
		OnStartup:        app.startup,
		Bind:             []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
