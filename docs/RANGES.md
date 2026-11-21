# 理論範圍：需求與格式

[简体中文](RANGES.zh-Hans.md)｜[English](RANGES.en.md)

PreflopMatrix 的「理論」欄全部來自一個範圍資料夾。本文件說明程式從哪裡讀範圍、需要哪些檔、每個檔的格式，以及怎麼檢查。

範圍不隨程式散布，也不內建在 App 裡：請自行準備（自己計算，或使用你有權使用的來源）。`.gitignore` 排除 `*.txt`（`testdata/parity/` 與 `internal/**/testdata/` 底下的測試資料除外）以及根目錄的 `ranges/`、`Ranges/`；pre-commit 會拒絕符合 `.gitignore` 的已暫存檔，以及不分大小寫的 `.txt`（上述兩個測試資料位置除外），避免範圍檔被 commit。

## 1. 從哪裡讀

### App（GUI）

只用你在畫面上選的「理論範圍資料夾」（App 會記住）。沒有環境變數、預設資料夾這些來源。

- 沒選：仍然產生報告，理論欄全部顯示 `–`，報告上方提醒「未選擇理論範圍」。
- 選了但資料夾已不存在：顯示錯誤，不產生報告；可「重新選擇」或「清除」。
- 選了但資料夾裡沒有任何 `.txt`：顯示錯誤，不產生報告（與 CLI 的結束代碼 1 一致）。

### 命令列（CLI）

| 順序 | 來源 | 例子 |
|---|---|---|
| 1 | 參數 `--ranges DIR` | `preflop-matrix --hands hh --ranges ~/my-ranges` |
| 2 | 環境變數 `PREFLOP_MATRIX_RANGES` | `export PREFLOP_MATRIX_RANGES=~/my-ranges` |
| 3 | 預設資料夾 | macOS／Linux：`$XDG_CONFIG_HOME/preflop-matrix/ranges`，沒設或不是絕對路徑時為 `~/.config/preflop-matrix/ranges`；Windows：`%AppData%\preflop-matrix\ranges`（忽略 `XDG_CONFIG_HOME`；Windows 的路徑邏輯已保留，但尚未正式支援或測試） |

- 每次執行第一行會印出用的是哪一個，例如 `理論範圍來源：預設資料夾 → /Users/…/.config/preflop-matrix/ranges`。
- 取**第一個有設定的來源**；那個來源的資料夾不存在就以結束代碼 `2` 停止，不會再往下找（no fall-through）。`--ranges ""` 視為已設定，路徑是目前資料夾 `.`；空字串的環境變數視為沒設定。
- 錯誤訊息會依來源給提示：預設資料夾不存在時列出三種設定方式；環境變數指錯時提示修正或 `unset PREFLOP_MATRIX_RANGES`。
- 只會展開開頭的 `~`（`~/…`），不展開 `~user`。

### 設定預設資料夾（CLI）

```bash
mkdir -p ~/.config/preflop-matrix/ranges
cp -R "<你的範圍資料夾>"/. ~/.config/preflop-matrix/ranges/
```

### 資料夾怎麼掃

遞迴掃描所有 `.txt`（副檔名要小寫；`.TXT` 不會被讀）；子資料夾可以用來分類，但不會跟隨資料夾的符號連結（檔案的符號連結照讀）。名為 `X.txt` 的資料夾若檔名對得上節點，會列為「無法讀取」。

## 2. 需要哪些檔

只用「兩人對抗」的節點。位置順序：UTG → HJ → CO → BTN → SB → BB，開池者一定比對手早。

| 節點 | 決策者 | 需要的檔（每組） | 組數 | 必要動作位 |
|---|---|---|---|---|
| `open` | 開池者 | 開池範圍 | 5（UTG、HJ、CO、BTN、SB） | 5 |
| `vs_open` | 防守者 | 跟注、3bet | 15（每個開池者 × 每個較晚位置） | 30 |
| `vs_3bet` | 開池者 | 跟注、4bet（`4bet` 或 `4bet all in` 任一個） | 15 | 30 |
| `vs_4bet` | 3bettor | 跟注、5bet（`5bet` 或 `5bet all in` 任一個） | 15 | 30 |

### 必要動作 95 vs 標準組 108

- **必要動作位 95**：5 + 30 + 30 + 30。範圍「完整」的標準是這 95 個位置都有對應的檔，且底範圍（`vs_3bet` 要有開池檔、`vs_4bet` 要有該 3bettor 的 3bet 檔）也在。`vs_3bet` 的 4bet 與 `vs_4bet` 的 5bet 各自有兩種寫法（見下），任一種就算填了。
- **標準組 108**：`--init-ranges`／「產生範圍檔名樣板」建立的檔名數量（5 + 30 + 30 + 43），只是參考；`vs_4bet` 的 15 組各有 `call` 與 `5bet all in`，其中 13 組另有 `5bet`，所以是 43。

### 5bet 與 5bet all in 互為替代

填了其中一個就算完整，另一個空白檔可以刪除；兩個都填則兩檔相加（上限 1）。樣板在 `BB vs SB 4bet` 與 `BTN vs CO 4bet` 這兩組只產生 `5bet all in`：在標準組採用的 Rush & Cash 6-max 設定中，這兩個位置面對 4bet 的 5bet 一律是全下（沒有較小的尺寸），所以不提供非全下的 5bet 檔。檢查時兩種寫法仍都接受。`vs_3bet` 的 `4bet` 與 `4bet all in` 也是同樣的相加規則。

### 缺檔時的行為

| 情況 | 結果 |
|---|---|
| 缺底範圍（`vs_3bet` 缺開池檔、`vs_4bet` 缺 3bet 檔） | 該格顯示 `–`（無理論） |
| 底範圍在、只缺一個動作檔 | 缺的動作當 0，摘要印「理論範圍不完整：… 缺 … 已當 0」 |
| 整組都沒有 | 該格顯示 `–`，不報錯 |
| `vs_5bet` | 一律沒有理論（檔案會被忽略） |

## 3. 檔名規則

檔名（不含 `.txt`）決定它屬於哪個節點。不分大小寫；尺寸 token（`2bb`、`9.5bb`）可省略（`BTN open` 也可以）；多個空白視為一個。

| 節點 | 檔名格式 | 例子 |
|---|---|---|
| `open` | `<開池者> open [尺寸]` | `BTN Open 2bb`、`SB open` |
| `vs_open` | `<防守者> vs <開池者> open [尺寸] call` | `BB vs BTN open 2bb call`、`BB vs UTG open call` |
| | `<防守者> vs <開池者> open [尺寸] 3bet [尺寸]`（`3b` 也可） | `BB vs BTN open 2bb 3bet 9.5bb`、`BB vs HJ open 2bb 3b 10bb` |
| `vs_3bet` | `<開池者> vs <3bettor> 3bet [尺寸] call` | `BTN vs BB 3bet 9.5bb call` |
| | `<開池者> vs <3bettor> 3bet [尺寸] 4bet [尺寸]` | `BTN vs BB 3bet 9.5bb 4bet 22bb` |
| | `… 4bet all in`（和 4bet 檔相加，上限 1） | `SB vs BB 3bet 9bb 4bet all in` |
| `vs_4bet` | `<3bettor> vs <開池者> 4bet [尺寸] call` | `BB vs BTN 4bet 22bb call` |
| | `<3bettor> vs <開池者> 4bet [尺寸] 5bet [尺寸]` | `BB vs CO 4bet 23bb 5bet 48.5bb` |
| | `… 5bet all in`（和 5bet 檔相加，上限 1） | `BB vs BTN 4bet 22bb 5bet all in` |

注意第二個位置的意思不同：`vs_open`、`vs_4bet` 是「對手」，`vs_3bet` 是「3bettor」；第一個位置永遠是做決策的人。

**會被忽略的檔**（摘要列為「忽略」）：
- 名字裡出現第三個位置或多人動作：`BB vs BTN open 2bb SB call call`、`… squeeze …`、`CO vs HJ 3bet UTG …`
- 5bet／6bet 之後的節點：`BTN vs BB 5bet 46bb all in`、`… 6bet all in call`
- 其他對不上格式的檔

同一節點有多個檔時，依完整路徑**逐層比較路徑元件**（區分大小寫、按 Unicode code point；例如 `a/` 排在 `a b/` 之前）排序，**先成功讀到、且不是空白檔**的那個勝出；其他有內容的檔列為「重複」（包括讀不了的）；空白檔一律列為「未填」（見第 4 節）。

### 檔名樣板

不想手打 108 個檔名時，用 App 的「產生範圍檔名樣板…」或 CLI 的 `--init-ranges DIR`，在指定資料夾建立空白檔：

```
UTG open.txt
BB vs UTG open call.txt
BB vs UTG open 3bet.txt
UTG vs BB 3bet call.txt
UTG vs BB 3bet 4bet.txt
BB vs UTG 4bet call.txt
BB vs UTG 4bet 5bet.txt
BB vs UTG 4bet 5bet all in.txt
```

- 樣板的檔名省略尺寸；`BB vs SB 4bet` 與 `BTN vs CO 4bet` 只有 `5bet all in`（沒有 `5bet`）。
- 不會覆蓋既有的檔，會回報新增與略過的數量；資料夾裡已有對應同一節點的檔（例如帶尺寸的檔名）就不再建立重複的空白檔。
- 同時寫入一份 `HOW-TO-FILL.md`（三語填寫說明），已存在則不覆蓋。
- 空白檔＝未填：建立樣板後，把要用的檔填上內容；不打的動作要寫成權重 0（見下一節）。

## 4. 檔案內容格式

常見求解器匯出的 combo 權重文字格式，逗號或換行分隔：

```
AsAh: 1,AsAd: 1,KsKh: 0.5,AhKh,7c6c: 0.25
```

- 每個 token 是 `<兩張牌>[: 權重]`。牌是點數 `AKQJT98765432` + 花色 `shdc`，例如 `Ts9s`。
- 省略權重 = 1。權重必須是 ≥ 0 的有限數字。
- 同一個 combo（不分順序）不可出現兩次；一個 combo 的兩張牌不可相同（不同 combo 共用同一張牌沒問題）。
- 檔案要是 UTF-8，可以有一個 BOM。
- 違反任何一條，該檔列為「無法讀取」，不會中斷整個執行。

### 空白檔＝未填，權重 0 怎麼寫

- **能對應到節點的檔**，去掉一個 BOM 後只剩空白的檔，視為**未填**：不佔用節點（同節點排序在後、有內容的檔會勝出），摘要與範圍檢查列為「未填（空白檔）」，不算無法讀取、也不算重複。
- 「空白」是 Python `str.isspace` 的集合：空白、Tab、換行、U+001C–U+001F、U+0085、U+00A0、U+3000 等。U+200B（零寬空白）不算，有它的檔列為「無法讀取」；兩個 BOM 也列為「無法讀取」。
- 對不上節點的空白檔（例如多人檔）仍列為「忽略」。
- **某個動作你根本不打，要明確寫成權重 0**，不要留空白：`AsAh: 0`，或只寫一個逗號 `,`（只有逗號的檔不是空白檔，是所有權重為 0 的有效範圍）。

程式把 combo 合併成 169 種手牌：每種手牌的值＝權重總和 ÷ combo 數（對子 6、同花 4、不同花 12），上限 1。

### 聯合權重

| 節點 | 權重代表 |
|---|---|
| `open`、`vs_open` | 這手牌在**全部發牌**中做這個動作的比例 |
| `vs_3bet`、`vs_4bet` | 同上，是**聯合權重**：已經包含前一層的篩選 |

例如 `BTN vs BB 3bet 9.5bb call` 裡 `AhKh: 0.3`，意思是在所有 AhKh 裡有 30% 走到「BTN 開池 → 被 BB 3bet → 跟注」，不是「開池的 AhKh 裡 30% 跟注」。求解器匯出某個節點的範圍時通常就是這種權重。

計算式：每手牌（169 種手牌層級，不是逐個 combo）先截在底範圍以內，取 `min(檔案權重, 底範圍權重)`；整格的條件頻率＝以底範圍加權的平均：

```
條件頻率 = Σ min(w, base) × combos ÷ Σ base × combos
```

底範圍權重為 0 的手牌不計。底範圍：`vs_3bet` 用開池檔，`vs_4bet` 用該 3bettor 對該開池者的 3bet 檔。`open` 與 `vs_open` 沒有底範圍，直接用 `Σ w × combos ÷ 1326`。

如果你的檔是條件權重（例如「開池的牌裡 30% 跟注」），`vs_3bet`／`vs_4bet` 的理論值會被高估。請先乘上底範圍再存檔。

## 5. 檢查你的範圍資料夾

### App

選好範圍資料夾後會自動檢查（也可按「重新檢查」）。「範圍檢查」面板顯示：

```
必要動作 X／95；檔案 N（標準組 108）
```

下面分組列出：缺的節點（整組缺，或缺底範圍，用標準檔名列出）、缺的動作檔（當 0）、未填（仍缺／可刪）、忽略、重複、無法讀取（含原因）。

### 命令列

```bash
preflop-matrix --check-ranges --ranges <範圍資料夾>
```

輸出同樣的內容；結束代碼：完整 `0`、不完整 `1`（含資料夾裡沒有 `.txt`、只有空白檔、因無法讀取造成缺位）、資料夾不存在或參數錯誤 `2`。

產生報告時，stdout 的摘要也會有這一行：

```
理論範圍：108 個檔案，對應到節點 108 個；忽略（多人或不認得）0、重複 0、無法讀取 0、未填（空白檔）0
```

- 「對應到節點」少於 108：用第 3 節的格式檢查檔名。
- 有「無法讀取」：會列出檔名和原因（例如壞牌、重複 combo、權重不合法）。
- 範圍資料夾裡沒有任何 `.txt` 才會 exit 1；全部空白或全部被忽略仍會輸出報告並 exit 0。
- 有「未填（空白檔）」：這些檔還沒填內容，不算有範圍，也不會佔用節點。
- 有「理論範圍不完整」：該節點缺某個動作檔。
- 有「警告：… 理論的跟注 + 加注 = … 超過 1，已按比例縮到 1」（stderr）：整格的跟注 + 加注超過 1。`vs_3bet`／`vs_4bet` 通常是條件／聯合權重混用；`vs_open` 沒有底範圍，表示跟注檔和 3bet 檔佔全部發牌的比例加起來超過 1（兩檔重疊）。
