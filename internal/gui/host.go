package gui

// host 是 AppService 需要的作業系統與視窗功能；正式版由 wailsHost 實作，測試用假物件取代。
type host interface {
	// PickDir 開資料夾選擇對話框；取消回傳 ("", nil)。startDir 為空或不是資料夾時由系統決定起點。
	PickDir(title, startDir string) (string, error)
	// SaveFile 開存檔對話框；取消回傳 ("", nil)。dir 為空時由系統決定起點。
	SaveFile(title, defaultName, dir string) (string, error)
	// OpenFile 用系統預設程式（HTML 為預設瀏覽器）開啟檔案。
	OpenFile(path string) error
	// PrintReport 開新視窗載入 url 並在載入完成後叫出列印面板。
	PrintReport(title, url string) error
}
