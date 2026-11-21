package gui

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	printWindowWidth  = 1100
	printWindowHeight = 800
	// WebViewDidFinishNavigation 之後 WKWebView 還在排版，立刻 Print 會印出空白頁；實測 500ms 穩定。
	printDelay = 500 * time.Millisecond
	blankPage  = "<!doctype html><title></title>"
)

// wailsHost 是 host 的 Wails 實作，只做 API 轉接，沒有邏輯（單元測試不涵蓋，靠手動驗證）。
type wailsHost struct {
	app *application.App
	win *application.WebviewWindow // 主視窗，對話框以 sheet 形式附在上面
}

func (h wailsHost) PickDir(title, startDir string) (string, error) {
	d := h.app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(true).
		SetTitle(title).SetMessage(title).AttachToWindow(h.win)
	if startDir != "" {
		d.SetDirectory(startDir)
	}
	return d.PromptForSingleSelection()
}

func (h wailsHost) SaveFile(title, defaultName, dir string) (string, error) {
	d := h.app.Dialog.SaveFile().SetMessage(title).SetFilename(defaultName).AddFilter("HTML", "*.html").AttachToWindow(h.win)
	if dir != "" {
		d.SetDirectory(dir)
	}
	return d.PromptForSingleSelection()
}

func (h wailsHost) OpenFile(path string) error { return h.app.Browser.OpenFile(path) }

// PrintReport 先用空白頁建立視窗並註冊事件，再 SetURL，避免載入完成事件在註冊前就發出而漏掉；
// 只有 SetURL 之後的載入完成才會列印，且只印一次。
func (h wailsHost) PrintReport(title, url string) error {
	w := h.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: title, Width: printWindowWidth, Height: printWindowHeight, HTML: blankPage,
	})
	var armed atomic.Bool
	var once sync.Once
	w.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(*application.WindowEvent) {
		if !armed.Load() {
			return
		}
		once.Do(func() {
			time.AfterFunc(printDelay, func() {
				if err := w.Print(); err != nil {
					log.Println("print:", err)
				}
			})
		})
	})
	armed.Store(true)
	w.SetURL(url)
	return nil
}
