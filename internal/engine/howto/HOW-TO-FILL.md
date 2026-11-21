# 範圍檔填寫說明（繁體中文）

這個資料夾裡每個 `.txt` 是一個「理論範圍」。檔名決定它屬於哪個節點，請保留檔名的位置與動作字樣；下注尺寸可以省略，也可以加上（例如 `BB vs UTG open 2.5bb 3bet 9bb`、`UTG open 2.5bb`）。

- 內容格式：以逗號或換行分隔的組合，每個組合寫成 `AsAh: 1`（冒號後是權重，必須 ≥ 0，通常 0 到 1，超過 1 會截到 1；省略冒號與權重視為 1），例如 `AsAh: 1,KsKh: 0.5`。同一個組合不能出現兩次。
- **空白檔＝尚未填寫**：程式會當作沒有這個檔，檢查會顯示「不完整」。不打的動作請寫成權重 0（例如 `AsAh: 0`，或只寫一個逗號 `,`），不要留空白。
- 同一個節點有多個檔時，只讀第一個有內容的檔，其他列為重複。
- `open`：該位置開池（加注）的範圍。
- `vs ... open call` / `open 3bet`：面對開池時的跟注、3bet 範圍。
- `vs ... 3bet call` / `3bet 4bet`：開池者面對 3bet 時的跟注、4bet 範圍。也可以另外加 `... 3bet 4bet all in` 檔，與 4bet 檔相加，上限 1。
- `vs ... 4bet call` / `4bet 5bet` / `4bet 5bet all in`：3bet 者面對 4bet 時的跟注、5bet 範圍。5bet 與 5bet all in 互為替代：填了其中一個就算完整，另一個空白檔可以刪除；兩個都填則兩檔相加，上限 1。`BB vs SB 4bet` 與 `BTN vs CO 4bet` 這兩組只提供 `all in` 檔。
- `vs ... 3bet`、`vs ... 4bet` 的檔是聯合權重（相對於全部發牌）：例如 `AhKh: 0.3` 表示全部 AhKh 中有 30% 走到「開池、被 3bet、跟注」這條路線，不是開池範圍內的 30%。超過底範圍的部分會被截掉。
- 完整的標準組共 108 個檔，必要的動作位共 95 個；用「檢查範圍」功能可以看還缺哪些。
- 再次產生樣板不會覆蓋既有的檔；如果資料夾裡已有對應同一節點的檔（例如帶尺寸的檔名），不會再產生重複的空白檔。

---

# 范围文件填写说明（简体中文）

这个文件夹里每个 `.txt` 是一个“理论范围”。文件名决定它属于哪个节点，请保留文件名中的位置和动作字样；下注尺寸可以省略，也可以加上（例如 `BB vs UTG open 2.5bb 3bet 9bb`、`UTG open 2.5bb`）。

- 内容格式：以逗号或换行分隔的组合，每个组合写成 `AsAh: 1`（冒号后是权重，必须 ≥ 0，通常 0 到 1，超过 1 会截到 1；省略冒号和权重视为 1），例如 `AsAh: 1,KsKh: 0.5`。同一个组合不能出现两次。
- **空白文件＝尚未填写**：程序会当作没有这个文件，检查会显示“不完整”。不打的动作请写成权重 0（例如 `AsAh: 0`，或只写一个逗号 `,`），不要留空白。
- 同一个节点有多个文件时，只读第一个有内容的文件，其他列为重复。
- `open`：该位置开池（加注）的范围。
- `vs ... open call` / `open 3bet`：面对开池时的跟注、3bet 范围。
- `vs ... 3bet call` / `3bet 4bet`：开池者面对 3bet 时的跟注、4bet 范围。也可以另外加 `... 3bet 4bet all in` 文件，与 4bet 文件相加，上限 1。
- `vs ... 4bet call` / `4bet 5bet` / `4bet 5bet all in`：3bet 者面对 4bet 时的跟注、5bet 范围。5bet 与 5bet all in 互为替代：填了其中一个就算完整，另一个空白文件可以删除；两个都填则两个文件相加，上限 1。`BB vs SB 4bet` 与 `BTN vs CO 4bet` 这两组只提供 `all in` 文件。
- `vs ... 3bet`、`vs ... 4bet` 的文件是联合权重（相对于全部发牌）：例如 `AhKh: 0.3` 表示全部 AhKh 中有 30% 走到“开池、被 3bet、跟注”这条路线，不是开池范围内的 30%。超过底范围的部分会被截掉。
- 完整的标准组共 108 个文件，必要的动作位共 95 个；用“检查范围”功能可以看还缺哪些。
- 再次生成模板不会覆盖已有的文件；如果文件夹里已有对应同一节点的文件（例如带尺寸的文件名），不会再生成重复的空白文件。

---

# How to fill in the range files (English)

Every `.txt` in this folder is one "theory range". The file name decides which node it belongs to; keep the positions and action words of the name. The bet size may be left out or added (for example `BB vs UTG open 2.5bb 3bet 9bb`, `UTG open 2.5bb`).

- Content: combos separated by commas or newlines, each written as `AsAh: 1` (the weight after the colon must be 0 or more, usually 0 to 1, and is capped at 1; with no colon and weight it is 1), for example `AsAh: 1,KsKh: 0.5`. A combo may not appear twice.
- **A blank file means "not filled in yet"**: the program treats it as missing, and the check shows "incomplete". For an action you never play, write weight 0 (for example `AsAh: 0`, or just a single comma `,`) instead of leaving the file blank.
- If several files map to the same node, only the first one with content is read; the others are listed as duplicates.
- `open`: the range the position opens (raises).
- `vs ... open call` / `open 3bet`: calling and 3betting ranges against an open.
- `vs ... 3bet call` / `3bet 4bet`: the opener's calling and 4betting ranges against a 3bet. You can also add a `... 3bet 4bet all in` file; it is added to the 4bet file, capped at 1.
- `vs ... 4bet call` / `4bet 5bet` / `4bet 5bet all in`: the 3bettor's calling and 5betting ranges against a 4bet. The plain 5bet and the 5bet all in file replace each other: filling one of them is enough, and the other blank file can be deleted; if both are filled they are added, capped at 1. `BB vs SB 4bet` and `BTN vs CO 4bet` only have the `all in` file.
- Files of `vs ... 3bet` and `vs ... 4bet` hold joint weights (relative to all deals): `AhKh: 0.3` means 30% of all AhKh go down the line "open, get 3bet, call", not 30% of the opening range. Any part above the base range is cut off.
- The full standard set is 108 files, with 95 required action slots; use the range check to see what is still missing.
- Creating the template again never overwrites existing files; if the folder already has a file for the same node (for example one with a bet size in its name), no blank duplicate is created.
