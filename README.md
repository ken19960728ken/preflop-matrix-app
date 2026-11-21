# PreflopMatrix

[繁體中文](README.md)｜[简体中文](README.zh-Hans.md)｜[English](README.en.md)

6-max 翻前「位置對位置」頻率熱力圖的桌面程式（macOS，Go + [Wails v3.0.0](https://v3.wails.io)），移植自 Python 版 [`preflop_matrix`](https://github.com/ken19960728ken/preflop_matrix)。介面與報告支援三種語言：繁體中文、简体中文、English。

## 1. 狀態

開發中，尚未發佈。目前只支援 macOS（13 以上），需要自行建置（見[開發](#8-開發)）。命令列（CLI）保留了 Linux／Windows 的路徑邏輯，但尚未正式支援或測試。

## 2. 功能概覽

讀取 GG Rush & Cash 6-max 手牌，加上一個理論範圍資料夾，輸出單一份翻前位置矩陣報告：

- 每個位置對抗（例如 BB 面對 CO 開池）顯示三種頻率：**理論**、**Hero（我）**、**玩家池**（除 Hero 以外所有人）。
- 兩種視角：條件頻率，以及乘回前一步後「佔全部 1326 種起手牌」的原始空間熱力圖。
- **偏離摘要**：列出實際頻率和理論差距最明顯的格子（見[第 4 節](#4-偏離摘要怎麼算)）。
- **範圍檢查**：檢查範圍資料夾是否完整、哪些檔缺、哪些檔無法讀取。
- **範圍檔名樣板**：一鍵建立空白的標準檔名，再自行填入範圍。
- 三語介面、三語報告（報告頁內也可切換語言）。

節點（每個節點只有兩人之間的決策；一旦有第三人做過非棄牌動作，該決策就排除）：

| 節點 | 決策者 | 選項 |
|---|---|---|
| `open` | 前面全棄的位置（SB 的 limp 算不開池） | 開池 / 其他 |
| `vs_open` | 開池後，中間沒人跟或加注的防守位置 | fold / call / 3bet |
| `vs_3bet` | 開池者面對單一 3bet | fold / call / 4bet |
| `vs_4bet` | 3bettor 面對開池者的 4bet | fold / call / 5bet |
| `vs_5bet` | 開池者面對 5bet | fold / call / 6bet（沒有理論） |

限制：只支援 GG／Natural8 Rush & Cash 格式，只解析翻前，只採用 6 人桌（其他人數的手牌排除並在摘要中計數）。報告使用 Google Fonts，離線時退回系統字型。

## 3. App 用法

1. **手牌資料夾**：按「選擇…」選 GG 匯出的手牌資料夾（遞迴掃描 `.txt`，同一 hand id 只算一次）。
2. **理論範圍資料夾**（選填）：選你自己準備的範圍資料夾。不選也能產生報告，理論欄全部顯示 `–`，報告上方提醒「未選擇理論範圍」。選過但資料夾已不存在會顯示錯誤，可「重新選擇」或「清除」。範圍的需求與格式見 [docs/RANGES.md](docs/RANGES.md)。
3. **Hero 名稱**：手牌檔裡你的玩家名稱（預設 `Hero`），按 Enter 或離開欄位時儲存。
4. 按「**產生報告**」。產生中按鈕會停用；完成後畫面顯示摘要與警告，報告顯示在主區域。
5. **範圍檢查**：展開面板，看「必要動作 X／95；檔案 N（標準組 108）」，以及缺的節點、缺的動作檔、未填、忽略、重複、無法讀取。選好範圍資料夾後會自動檢查。
6. **產生範圍檔名樣板…**：選一個資料夾，在裡面建立空白的標準檔名（不覆蓋既有檔案，另附 `HOW-TO-FILL.md` 三語填寫說明）。若選的就是目前的範圍資料夾，會自動重新檢查。
7. **偏離摘要**：分「玩家池」與「Hero」兩張表；「列數」可設 0 至 20（預設 5，0 代表不列出）；點任一列，報告會跳到對應的格子。
8. **存 HTML**：存成單一 HTML 檔（目前語言，含三語切換器）。**列印**：開新視窗並叫出列印對話框（列印時隱藏語言切換器、保留偏離摘要）。**用瀏覽器開啟**：寫到 `~/Library/Caches/preflop-matrix/report.html`（不會自動清除）後用預設瀏覽器開啟。
9. **語言**：頂部可切換繁中／简中／English。第一次使用跟隨系統語言，選過之後會記住。

設定存放位置：`~/Library/Application Support/preflop-matrix/settings.json`（語言、手牌資料夾、範圍資料夾、Hero、列數、上次存檔位置）。檔案損壞時會改名保留並改用預設值。

在 macOS 的選資料夾對話框中，要看到隱藏資料夾（例如 `~/.config`）請按 `⌘⇧.`（Command + Shift + 句點）。

## 4. 偏離摘要怎麼算

- 納入的節點：`open`、`vs_open`、`vs_3bet`、`vs_4bet`（`vs_5bet` 沒有理論，不列入）。該格要有理論值與實際觀察，且樣本數 **n ≥ 30**。
- `open` 只看開池率；其他節點看 fold、call、raise 三個動作（理論 fold = 1 − call − raise，限制在 0 到 1 之間）。
- 顯著性：z = (實際 − θ) ÷ √(θc(1 − θc) ÷ n)，只列出 **|z| ≥ 3** 的動作。θc 是把理論值 θ 夾在 0.5%–99.5% 之間後的值，只用在分母；分子仍用原本的理論值。被夾過的列會標示「理論值已調整」。
- 排序：以 **Cohen's h**（|2·asin√實際 − 2·asin√θ|，用原始 θ）由大到小；相同時依序比 |z|、|差|，再比 n（大者在前），最後依節點、格子、動作（fold、call、raise）的順序。
- **同一格只列 h 最大的那個動作**。
- 玩家池與 Hero 各列前 N 筆（預設 5，範圍 0–20）。
- 每列顯示：位置對抗（決策者在前，例如 `BB vs CO`）、節點、動作、實際%、理論%、差（百分點）、n、z。

這只是把數字呈現出來，**不是策略建議**。

## 5. 理論範圍檔

理論範圍**不隨程式散布**，也不內建在 App 裡；請自行準備（自己計算，或使用你有權使用的來源）。需要哪些檔、檔名規則、內容格式、聯合權重的意義、如何檢查，見：

- [docs/RANGES.md](docs/RANGES.md)（繁體中文）
- [docs/RANGES.zh-Hans.md](docs/RANGES.zh-Hans.md)（简体中文）
- [docs/RANGES.en.md](docs/RANGES.en.md)（English）

檔名樣板：用 App 的「產生範圍檔名樣板…」或 `--init-ranges DIR`，會建立 108 個空白的標準檔名，例如 `UTG open`、`BB vs UTG open call`、`BB vs UTG open 3bet`、`UTG vs BB 3bet call`、`UTG vs BB 3bet 4bet`、`BB vs UTG 4bet call`、`BB vs UTG 4bet 5bet`、`BB vs UTG 4bet 5bet all in`。

## 6. 命令列（CLI）

兩種執行方式：

```sh
go run ./cmd/preflop-matrix --hands <GG手牌資料夾> --ranges <範圍資料夾> -o out.html
# 或先建置
go build -o preflop-matrix ./cmd/preflop-matrix
./preflop-matrix --help
```

App 本體（`PreflopMatrix.app/Contents/MacOS/PreflopMatrix`）在第一個參數是下列旗標之一時也走命令列，否則開啟視窗。

### 三種模式

```text
preflop-matrix --hands DIR [--ranges DIR] [-o FILE] [--hero NAME] [--title TEXT] [--json PATH] [--lang LANG] [--top N]
preflop-matrix --check-ranges [--ranges DIR] [--lang LANG]
preflop-matrix --init-ranges DIR [--lang LANG]
```

| 旗標 | 說明 |
|---|---|
| `--hands DIR` | 產生報告時必填。遞迴掃描 `.txt`，同一 hand id 只算一次 |
| `--ranges DIR` | 理論範圍資料夾（`.txt`） |
| `-o, --out FILE` | 輸出 HTML，預設 `preflop_matrix.html` |
| `--hero NAME` | Hero 的玩家名稱，預設 `Hero` |
| `--title TEXT` | 頁面標題；預設依輸出語言（繁中 `翻前位置矩陣`、簡中 `翻前位置矩阵`、英文 `Preflop Position Matrix`） |
| `--json PATH` | 另存矩陣資料 JSON（與 HTML 內嵌的相同；不可與 `-o` 指向同一個檔案） |
| `--lang LANG` | 輸出語言：`zh-Hant`、`zh-Hans`、`en`（也接受 `zh-TW`、`zh-CN`）；預設跟隨系統語言 |
| `--top N` | 偏離摘要每組（玩家池、Hero）列出的筆數，0 到 20，預設 5，0 為不顯示；超出範圍結束代碼 2 |
| `--check-ranges` | 只檢查範圍資料夾的完整度，不產生報告 |
| `--init-ranges DIR` | 在 DIR 建立空白的範圍檔名樣板，不覆蓋既有檔案，不產生報告 |
| `-h, --help` | 顯示說明 |

`--check-ranges` 與 `--init-ranges` 不能彼此併用，也不能和產生報告的參數併用（違反時結束代碼 2）；`--init-ranges` 也不能搭配 `--ranges`。

### 結束代碼

| 模式 | `0` | `1` | `2` |
|---|---|---|---|
| 產生報告 | 成功（有任一手可用就輸出，無法解析的手牌計數跳過並在摘要列出） | 沒有可用手牌、範圍資料夾沒有 `.txt`、無法寫入輸出 | 資料夾不存在、參數錯誤 |
| `--check-ranges` | 範圍完整 | 不完整（含沒有 `.txt`、只有空白檔、無法讀取造成缺位） | 資料夾不存在、參數錯誤 |
| `--init-ranges` | 成功（全部略過也算） | 寫入失敗 | 參數錯誤、路徑不是資料夾 |

### 範圍來源順序

`--ranges` > 環境變數 `PREFLOP_MATRIX_RANGES` > 預設資料夾（macOS／Linux：`$XDG_CONFIG_HOME/preflop-matrix/ranges`，未設或非絕對路徑時 `~/.config/preflop-matrix/ranges`）。取第一個有設定的來源，該來源的資料夾不存在就停止，不會往下找（no fall-through）。`--ranges ""` 視為 `.`。詳見 [docs/RANGES.md](docs/RANGES.md)。

產生報告時，stdout 會印出手牌與範圍摘要，以及偏離摘要（`--top 0` 則不印）。

### 與 Python 版的關係

這是 Python 版 `preflop_matrix` 的移植，並以測試對照：

- 在 `testdata/parity/` 的合成語料（合成手牌與合成範圍）上，`--json` 的矩陣資料與 Python 版逐值相等（零容差）。
- 用 `--lang zh-Hant --top 0` 時，stdout 的分類摘要與 Python 版相同（「已輸出」的路徑行與無法讀取的原因除外），stderr 逐字相同。
- 下列差異在極端輸入下可能讓結果不同；一般資料不受影響。

Go 版新增了 Python 版沒有的功能：三語介面、偏離摘要、範圍檢查、檔名樣板、桌面 App。

可接受的差異（完整清單見 [`internal/cli/doc.go`](internal/cli/doc.go) 與 [`internal/engine/doc.go`](internal/engine/doc.go)）：

- 不接受 argparse 式的旗標縮寫（`--ran`）；引數錯誤一律優先於 `-h`；錯誤訊息是 Go 版自己的文字。
- 不展開 `~user`（只展開開頭的 `~`）。
- `--json` 的格式細節（縮排、不做 HTML 跳脫、U+2028／U+2029 寫成跳脫序列），數值相同。
- Unicode 版本差異：`\d` 只比對 ASCII 數字；空白與可列印字元的判定表也可能不同。
- 「已輸出」行裡的 `-o`、`--json` 路徑不轉成絕對路徑。
- 金額以精確有理數計算，不是 28 位有效數字的 Decimal。
- 無法讀取檔案的原因文字是 Go 的錯誤文字。
- Windows 上檔案排序與大小寫比對可能不同。

## 7. 在 macOS 首次開啟未簽章的 App

若你自行建置，或從他人取得這個 App，它沒有經過 Apple 公證，第一次開啟時 macOS 可能擋下（自行在本機建置的通常不會帶隔離標記）。放行方式：

- **macOS 13–14**：在 Finder 對 App 按右鍵（或 Control + 點擊）→「打開」→ 再按「打開」。
- **macOS 15 以上**：先嘗試開啟一次，再到「系統設定 → 隱私權與安全性」，在「安全性」區塊按「強制打開」（按鈕在嘗試開啟後約一小時內可用），並輸入登入密碼。
- 或用終端機移除隔離標記：

```sh
xattr -dr com.apple.quarantine /path/to/PreflopMatrix.app
```

詳見 Apple 的說明：[在 Mac 上開啟來自未識別開發者的 App](https://support.apple.com/zh-tw/guide/mac-help/mh40616/mac)。

## 8. 開發

需求：macOS 13+、Go ≥ 1.26.10、Node（用 nvm 安裝；版本需滿足 `^20.19.0 || >=22.12.0`，專案的 `.nvmrc` 為 22，`nvm use` 即可；已用 v24 驗證 `npm ci` 與 `npm run build`）、`wails3` 精確 v3.0.0。

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
export PATH="$PATH:$(go env GOPATH)/bin"   # wails3 安裝在 ~/go/bin，需在 PATH 內
git config core.hooksPath .githooks         # 啟用 pre-commit
cd frontend && npm ci                       # @wailsio/runtime、typescript、vite 皆固定精確版本
```

`.gitignore` 排除 `*.txt`（`testdata/parity/` 與 `internal/**/testdata/` 底下的測試資料除外）、根目錄的 `ranges/` 與 `Ranges/`、`*.html`、`*.json`（專案需要的幾個除外）、`.env` 等。pre-commit 會拒絕任何符合 `.gitignore` 的已暫存檔（含 `git add -f` 強加的），以及不分大小寫的 `.txt`（上述兩個測試資料位置除外），避免範圍檔、手牌、報告被 commit。`wails3 build` 會先檢查 `wails3 version` 必須是 v3.0.0。

`frontend/dist/` 的內容不進版控，只保留 `.gitkeep`，所以乾淨 clone 不必先建置前端就能直接執行 `go vet ./...`、`go test ./...`。

| 指令 | 說明 |
|---|---|
| `go test -short ./...` | 日常測試（略過需要 Chrome 的測試；含對 golden 的逐值比對） |
| `task test:report` | 報告測試，含 headless Chrome（需要安裝 Chrome）；commit 前跑 |
| `npm run typecheck`（在 `frontend/`） | TypeScript 型別檢查 |
| `wails3 build` | 建置，輸出 `bin/PreflopMatrix` |
| `wails3 task darwin:package:universal` | 打包 arm64 + amd64 的 universal `bin/PreflopMatrix.app`（ad-hoc 簽署） |
| `task dev`（即 `wails3 dev -config ./build/config.yml -port 9245`） | 開發模式 |

`task` 可用 `brew install go-task` 安裝，或改用 `wails3 task <名稱>`。打包 universal 之後若要再跑 `wails3 build`，請先刪掉 universal 的 `bin/PreflopMatrix`，否則 `go build` 會拒絕覆蓋這個 universal 檔案而失敗。

### Parity（與 Python 版比對）

```sh
tools/parity/make_golden.sh [PY_REPO]             # 用 Python 版重新產生 golden（Python 版 repo 以參數指定，必須是乾淨的 checkout）
scripts/parity_real.sh <手牌資料夾> [範圍資料夾]   # 用真實資料比對 Python 與 Go，輸出不留存；範圍預設 ~/.config/preflop-matrix/ranges（唯讀）；Python 版 repo 以 PY_REPO 環境變數指定
go run ./tools/paritydiff-cli [--summary] a b     # 比對兩個 JSON（加 --summary 則比對兩份 stdout 摘要）
```

語料由 `tools/parity/make_corpus.py` 產生，golden 由 `tools/parity/make_golden.sh` 執行 Python 版產生；全是合成資料，不含任何真實手牌或範圍。

### 開發用環境變數與模式

- `PMA_SETTINGS_PATH`：把設定檔指到別處，避免動到真實設定。**只在沒有 `production` build tag 的建置有效**（例如 `wails3 dev`）；`wails3 build`／`package` 的產出會忽略它。
- 純前端開發（不需要 Go 後端，用假報告）：

```sh
cd frontend && npm run dev:mock    # 等同 VITE_MOCK=1 vite，網址 http://127.0.0.1:9245
```

## 9. 授權

GPL-3.0-only。可以自由使用、修改、散布；散布本程式（含修改版）時，須以 GPL-3.0 授權並提供對應原始碼。全文見 [LICENSE](LICENSE)。
