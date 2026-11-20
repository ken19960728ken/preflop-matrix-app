# PreflopMatrix

> 開發中，尚未發佈。

翻前範圍矩陣報告桌面程式（macOS，Go + [Wails v3.0.0](https://v3.wails.io)），移植自 Python 版 `preflop_matrix`。支援三語：繁體中文、简体中文、English。

其他語言說明（之後補上）：[简体中文](README.zh-Hans.md)｜[English](README.en.md)

## 開發

需求：Go >= 1.26.10、Node（nvm）、`wails3` v3.0.0。

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
git config core.hooksPath .githooks   # 啟用 pre-commit：只有 testdata/parity/ 底下的 .txt 能 commit
```

前端：

```sh
cd frontend && npm install            # @wailsio/runtime 固定 3.0.0
```

`frontend/dist/` 不進版控，但 `main.go` 以 `go:embed all:frontend/dist` 嵌入它。乾淨 clone 在執行 `go vet ./...`、`go test ./...` 前，必須先產生 dist：

```sh
cd frontend && npm run build && cd ..
```

建置與執行：

```sh
wails3 build                          # 產出 bin/PreflopMatrix
./bin/PreflopMatrix                   # 無參數：開啟 GUI
./bin/PreflopMatrix --foo             # 有參數：命令列模式
```

測試：

```sh
go vet ./... ; echo "exit=$?"
go test ./... ; echo "exit=$?"
```

## 授權

GPL-3.0（全文見 [LICENSE](LICENSE)）。可以自由使用、修改、散布；修改後再發佈必須以 GPL-3.0 公開原始碼。

Copyright © 2026 ken19960728ken
