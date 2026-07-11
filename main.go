package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "Pal Save Relay",
		Description: "Palworld co-op save relay",
		Services: []application.Service{
			application.NewService(NewApp()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:            "Pal Save Relay",
		URL:              "/",
		Width:            960,
		Height:           680,
		BackgroundColour: application.NewRGB(245, 245, 247),
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
