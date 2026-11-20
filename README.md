# PreflopMatrix

> 開發中，尚未發佈。

翻前範圍矩陣報告桌面程式（macOS，Go + [Wails v3.0.0](https://v3.wails.io)），移植自 Python 版 `preflop_matrix`。支援三語：繁體中文、简体中文、English。

其他語言說明（之後補上）：[简体中文](README.zh-Hans.md)｜[English](README.en.md)

## 開發

需求：macOS 13+、Go >= 1.26.10、Node（nvm）、`wails3` 精確 v3.0.0。

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
export PATH="$PATH:$(go env GOPATH)/bin"   # wails3 安裝在 ~/go/bin，需在 PATH 內
git config core.hooksPath .githooks         # 啟用 pre-commit（見下）
```

`wails3 build` 會先檢查 `wails3 version` 必須是 v3.0.0，否則中止。

pre-commit 會拒絕任何符合 `.gitignore` 的檔案（含 `git add -f` 強加的）以及 `testdata/` 以外的 `.txt`，避免範圍檔、手牌、報告被 commit。

前端：

```sh
cd frontend && npm ci                      # @wailsio/runtime、typescript、vite 皆固定精確版本
npm run typecheck                          # tsc --noEmit；npm run build 會先執行它
```

`frontend/dist/` 的內容不進版控，只保留 `.gitkeep`，所以乾淨 clone 不必先建置前端就能直接 `go vet ./...`、`go test ./...`（`main.go` 以 `go:embed all:frontend/dist` 嵌入它）。

```sh
go vet ./... ; echo "exit=$?"
go test ./... ; echo "exit=$?"
```

日常開發用 `go test -short ./...`（略過需要 Google Chrome 的報告渲染測試）。commit 前跑 `task test:report`（等效：`PMA_REQUIRE_CHROME=1 go test ./internal/report/...`），它用 headless Chrome（`/Applications/Google Chrome.app`）實際渲染三語報告，找不到 Chrome 會失敗而不是略過。

### 建置與執行

```sh
wails3 build                               # 產出 bin/PreflopMatrix（裸執行檔）
wails3 package                             # 產出 bin/PreflopMatrix.app
open bin/PreflopMatrix.app
```

裸執行檔沒有 Info.plist，不會帶 `CFBundleLocalizations` 等資訊；測試系統語系或 bundle 行為請用 `.app`。

### 命令列

GUI App 只在第一個參數是已知 CLI 旗標（`--hands --ranges -o --out --hero --title --json --lang --top --check-ranges --init-ranges -h --help`）時才走命令列，其餘一律開視窗。純命令列入口（無 GUI 依賴，Windows 也適用）：

```sh
go run ./cmd/preflop-matrix --help
go build -o bin/preflop-matrix ./cmd/preflop-matrix
```

## 授權

GPL-3.0-only（全文見 [LICENSE](LICENSE)）。可以自由使用、修改、散布；散布本程式（含修改版）時，須以 GPL-3.0 授權並提供對應原始碼。

Copyright © 2026 ken19960728ken
