package gui

import (
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	windowTitle     = "PreflopMatrix"
	windowWidth     = 1200
	windowHeight    = 800
	windowMinWidth  = 960
	windowMinHeight = 640
)

// Run 開啟空白主視窗並阻塞到應用程式結束，回傳 exit code。
func Run(assets fs.FS) int {
	app := application.New(application.Options{
		Name:        "PreflopMatrix",
		Description: "Preflop range matrix report",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     windowTitle,
		Width:     windowWidth,
		Height:    windowHeight,
		MinWidth:  windowMinWidth,
		MinHeight: windowMinHeight,
		URL:       "/",
	})

	if err := app.Run(); err != nil {
		log.Println(err)
		return 1
	}
	return 0
}
