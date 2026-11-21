package gui

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
	"github.com/ken19960728ken/preflop-matrix-app/internal/i18n"
)

const (
	appName         = "PreflopMatrix"
	appDescription  = "Preflop range matrix report"
	windowTitle     = appName
	windowWidth     = 1200
	windowHeight    = 800
	windowMinWidth  = 960
	windowMinHeight = 640
	reportRoute     = "/report"
	tempSubdir      = "preflop-matrix"
)

// settingsPath 是設定檔路徑；非 production 建置可用 PMA_SETTINGS_PATH 覆寫。
func settingsPath() (string, error) {
	if p := settingsPathOverride(); p != "" {
		return p, nil
	}
	return config.SettingsPath()
}

// Run 開啟主視窗並阻塞到應用程式結束，回傳 exit code。
func Run(assets fs.FS) int {
	path, err := settingsPath()
	if err != nil {
		log.Println(err)
		return 1
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		log.Println(err)
		return 1
	}
	svc := NewAppService(Options{SettingsPath: path, CacheDir: filepath.Join(cache, tempSubdir), DetectLang: i18n.Detect})
	app := application.New(application.Options{
		Name:        appName,
		Description: appDescription,
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: guardRuntime,
		},
		Services: []application.Service{
			application.NewServiceWithOptions(svc, application.ServiceOptions{Route: reportRoute}),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     windowTitle,
		Width:     windowWidth,
		Height:    windowHeight,
		MinWidth:  windowMinWidth,
		MinHeight: windowMinHeight,
		URL:       "/",
	})

	svc.host = wailsHost{app, win}

	if err := app.Run(); err != nil {
		log.Println(err)
		return 1
	}
	return 0
}
