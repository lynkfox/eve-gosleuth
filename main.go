package main

import (
	"embed"

	"github.com/lynkfox/go_eve_sleuth/gui"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// holds the app's version, bumped there as info.productVersion
//
//go:embed wails.json
var wailsJSON []byte

func main() {
	version := gui.ProjectVersion(wailsJSON)
	app := gui.NewApp(version)
	startup, shutdown := gui.Lifecycle(app)

	err := wails.Run(&options.App{
		Title:     "EVE GoSleuth",
		Width:     1375,
		Height:    1000,
		MinWidth:  720,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 11, G: 14, B: 19, A: 1},
		OnStartup:        startup,
		OnShutdown:       shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
