# PreflopMatrix

[繁體中文](README.md)｜[简体中文](README.zh-Hans.md)｜[English](README.en.md)

6-max 翻前“位置对位置”频率热力图的桌面程序（macOS，Go + [Wails v3.0.0](https://v3.wails.io)），移植自 Python 版 [`preflop_matrix`](https://github.com/ken19960728ken/preflop_matrix)。界面和报告支持三种语言：繁體中文、简体中文、English。

## 1. 状态

开发中，尚未发布。目前只支持 macOS（13 及以上），需要自行构建（见[开发](#8-开发)）。命令行（CLI）保留了 Linux／Windows 的路径逻辑，但尚未正式支持或测试。

## 2. 功能概览

读取 GG Rush & Cash 6-max 手牌，再加上一个理论范围文件夹，输出一份翻前位置矩阵报告：

- 每个位置对抗（例如 BB 面对 CO 开池）显示三种频率：**理论**、**Hero（我）**、**玩家池**（除 Hero 以外的所有人）。
- 两种视角：条件频率，以及乘回上一步之后“占全部 1326 种起手牌”的原始空间热力图。
- **偏离摘要**：列出实际频率与理论差距最明显的格子（见[第 4 节](#4-偏离摘要怎么计算)）。
- **范围检查**：检查范围文件夹是否完整、缺哪些文件、哪些文件无法读取。
- **范围文件名模板**：一键创建空白的标准文件名，再自行填入范围。
- 三语界面、三语报告（报告页内也可以切换语言）。

节点（每个节点只涉及两个人之间的决策；一旦有第三个人做过非弃牌动作，该决策就被排除）：

| 节点 | 决策者 | 选项 |
|---|---|---|
| `open` | 前面全部弃牌的位置（SB 的 limp 不算开池） | 开池 / 其他 |
| `vs_open` | 开池后，中间没人跟注或加注的防守位置 | fold / call / 3bet |
| `vs_3bet` | 开池者面对单一 3bet | fold / call / 4bet |
| `vs_4bet` | 3bettor 面对开池者的 4bet | fold / call / 5bet |
| `vs_5bet` | 开池者面对 5bet | fold / call / 6bet（没有理论） |

限制：只支持 GG／Natural8 Rush & Cash 格式，只解析翻前，只采用 6 人桌（其他人数的手牌会被排除，并在摘要中计数）。报告使用 Google Fonts，离线时回退到系统字体。

## 3. App 用法

1. **手牌文件夹**：点“选择…”选择 GG 导出的手牌文件夹（递归扫描 `.txt`，同一 hand id 只计一次）。
2. **理论范围文件夹**（可选）：选择你自己准备的范围文件夹。不选也能生成报告，理论列全部显示 `–`，报告顶部提示“未选择理论范围”。选过但文件夹已不存在时会显示错误，可以“重新选择”或“清除”。范围的要求和格式见 [docs/RANGES.zh-Hans.md](docs/RANGES.zh-Hans.md)。
3. **Hero 名称**：手牌文件里你的玩家名称（默认 `Hero`），按回车或离开输入框时保存。
4. 点“**生成报告**”。生成过程中按钮会停用；完成后界面显示摘要和警告，报告显示在主区域。
5. **范围检查**：展开面板，查看“必要动作 X／95；文件 N（标准组 108）”，以及缺的节点、缺的动作文件、未填、忽略、重复、无法读取。选好范围文件夹后会自动检查。
6. **生成范围文件名模板…**：选一个文件夹，在里面创建空白的标准文件名（不覆盖已有文件，并附带 `HOW-TO-FILL.md` 三语填写说明）。如果选的正是当前的范围文件夹，会自动重新检查。
7. **偏离摘要**：分“玩家池”和“Hero”两张表；“行数”可设为 0 到 20（默认 5，0 表示不列出）；点任意一行，报告会跳转到对应的格子。
8. **保存 HTML**：保存为单个 HTML 文件（当前语言，含三语切换器）。**打印**：打开新窗口并调出打印对话框（打印时隐藏语言切换器、保留偏离摘要）。**用浏览器打开**：写入 `~/Library/Caches/preflop-matrix/report.html`（不会自动清除）后用默认浏览器打开。
9. **语言**：顶部可切换繁中／简中／English。首次使用跟随系统语言，选择过之后会记住。

设置保存位置：`~/Library/Application Support/preflop-matrix/settings.json`（语言、手牌文件夹、范围文件夹、Hero、列数、上次保存位置）。文件损坏时会改名保留，并改用默认值。

在 macOS 的选择文件夹对话框里，要显示隐藏文件夹（例如 `~/.config`），请按 `⌘⇧.`（Command + Shift + 句号）。

## 4. 偏离摘要怎么计算

- 纳入的节点：`open`、`vs_open`、`vs_3bet`、`vs_4bet`（`vs_5bet` 没有理论，不列入）。该格需要同时有理论值和实际观察，且样本数 **n ≥ 30**。
- `open` 只看开池率；其他节点看 fold、call、raise 三个动作（理论 fold = 1 − call − raise，限制在 0 到 1 之间）。
- 显著性：z = (实际 − θ) ÷ √(θc(1 − θc) ÷ n)，只列出 **|z| ≥ 3** 的动作。θc 是把理论值 θ 限制在 0.5%–99.5% 之间后的值，只用于分母；分子仍使用原来的理论值。被限制过的行会标注“理论值已调整”。
- 排序：按 **Cohen's h**（|2·asin√实际 − 2·asin√θ|，使用原始 θ）从大到小；相同时依次比较 |z|、|差|，再比较 n（大者在前），最后按节点、格子、动作（fold、call、raise）的顺序。
- **同一个格子只列出 h 最大的那个动作**。
- 玩家池和 Hero 各列前 N 条（默认 5，范围 0–20）。
- 每行显示：位置对抗（决策者在前，例如 `BB vs CO`）、节点、动作、实际%、理论%、差（百分点）、n、z。

这只是把数字呈现出来，**不是策略建议**。

## 5. 理论范围文件

理论范围**不随程序分发**，也不内置在 App 里；请自行准备（自己计算，或使用你有权使用的来源）。需要哪些文件、文件名规则、内容格式、联合权重的含义、如何检查，见：

- [docs/RANGES.md](docs/RANGES.md)（繁體中文）
- [docs/RANGES.zh-Hans.md](docs/RANGES.zh-Hans.md)（简体中文）
- [docs/RANGES.en.md](docs/RANGES.en.md)（English）

文件名模板：使用 App 的“生成范围文件名模板…”或 `--init-ranges DIR`，会创建 108 个空白的标准文件名，例如 `UTG open`、`BB vs UTG open call`、`BB vs UTG open 3bet`、`UTG vs BB 3bet call`、`UTG vs BB 3bet 4bet`、`BB vs UTG 4bet call`、`BB vs UTG 4bet 5bet`、`BB vs UTG 4bet 5bet all in`。

## 6. 命令行（CLI）

两种运行方式：

```sh
go run ./cmd/preflop-matrix --hands <GG手牌文件夹> --ranges <范围文件夹> -o out.html
# 或先构建
go build -o preflop-matrix ./cmd/preflop-matrix
./preflop-matrix --help
```

App 本体（`PreflopMatrix.app/Contents/MacOS/PreflopMatrix`）在第一个参数是下列选项之一时也走命令行，否则打开窗口。

### 三种模式

```text
preflop-matrix --hands DIR [--ranges DIR] [-o FILE] [--hero NAME] [--title TEXT] [--json PATH] [--lang LANG] [--top N]
preflop-matrix --check-ranges [--ranges DIR] [--lang LANG]
preflop-matrix --init-ranges DIR [--lang LANG]
```

| 选项 | 说明 |
|---|---|
| `--hands DIR` | 生成报告时必填。递归扫描 `.txt`，同一 hand id 只计一次 |
| `--ranges DIR` | 理论范围文件夹（`.txt`） |
| `-o, --out FILE` | 输出 HTML，默认 `preflop_matrix.html` |
| `--hero NAME` | Hero 的玩家名称，默认 `Hero` |
| `--title TEXT` | 页面标题；默认随输出语言（繁中 `翻前位置矩陣`、简中 `翻前位置矩阵`、英文 `Preflop Position Matrix`） |
| `--json PATH` | 另存矩阵数据 JSON（与 HTML 内嵌的相同；不能与 `-o` 指向同一个文件） |
| `--lang LANG` | 输出语言：`zh-Hant`、`zh-Hans`、`en`（也接受 `zh-TW`、`zh-CN`）；默认跟随系统语言 |
| `--top N` | 偏离摘要每组（玩家池、Hero）列出的条数，0 到 20，默认 5，0 表示不显示；超出范围时退出码为 2 |
| `--check-ranges` | 只检查范围文件夹的完整度，不生成报告 |
| `--init-ranges DIR` | 在 DIR 创建空白的范围文件名模板，不覆盖已有文件，不生成报告 |
| `-h, --help` | 显示帮助 |

`--check-ranges` 与 `--init-ranges` 不能互相同时使用，也不能与生成报告的选项同时使用（违反时退出码为 2）；`--init-ranges` 也不能搭配 `--ranges`。

### 退出码

| 模式 | `0` | `1` | `2` |
|---|---|---|---|
| 生成报告 | 成功（只要有可用手牌就输出，无法解析的手牌会计数跳过并在摘要中列出） | 没有可用手牌、范围文件夹里没有 `.txt`、无法写入输出 | 文件夹不存在、参数错误 |
| `--check-ranges` | 范围完整 | 不完整（包括没有 `.txt`、只有空白文件、因无法读取造成缺位） | 文件夹不存在、参数错误 |
| `--init-ranges` | 成功（全部跳过也算） | 写入失败 | 参数错误、路径不是文件夹 |

### 范围来源顺序

`--ranges` > 环境变量 `PREFLOP_MATRIX_RANGES` > 默认文件夹（macOS／Linux：`$XDG_CONFIG_HOME/preflop-matrix/ranges`，未设置或不是绝对路径时为 `~/.config/preflop-matrix/ranges`）。取第一个已设置的来源，该来源的文件夹不存在就停止，不会继续往下找（no fall-through）。`--ranges ""` 视为 `.`。详见 [docs/RANGES.zh-Hans.md](docs/RANGES.zh-Hans.md)。

生成报告时，stdout 会打印手牌与范围摘要，以及偏离摘要（`--top 0` 则不打印）。

### 与 Python 版的关系

这是 Python 版 `preflop_matrix` 的移植，并通过测试对照：

- 在 `testdata/parity/` 的合成语料（合成手牌和合成范围）上，`--json` 的矩阵数据与 Python 版逐值相等（零容差）。
- 使用 `--lang zh-Hant --top 0` 时，stdout 的分类摘要与 Python 版相同（“已输出”的路径行和无法读取的原因除外），stderr 逐字相同。
- 下列差异在极端输入下可能使结果不同；一般数据不受影响。

Go 版新增了 Python 版没有的功能：三语界面、偏离摘要、范围检查、文件名模板、桌面 App。

可接受的差异（完整清单见 [`internal/cli/doc.go`](internal/cli/doc.go) 和 [`internal/engine/doc.go`](internal/engine/doc.go)）：

- 不支持 argparse 风格的选项缩写（`--ran`）；参数错误一律优先于 `-h`；错误信息是 Go 版自己的文字。
- 不展开 `~user`（只展开开头的 `~`）。
- `--json` 的格式细节（缩进、不做 HTML 转义、U+2028／U+2029 写成转义序列），数值相同。
- Unicode 版本差异：`\d` 只匹配 ASCII 数字；空白和可打印字符的判定表也可能不同。
- “已输出”行里的 `-o`、`--json` 路径不会转成绝对路径。
- 金额使用精确有理数计算，而不是 28 位有效数字的 Decimal。
- 无法读取文件的原因文字是 Go 的错误文字。
- 在 Windows 上，文件排序和大小写匹配可能不同。

## 7. 在 macOS 上首次打开未签名的 App

如果你是自行构建，或从别人那里拿到这个 App，它没有经过 Apple 公证，首次打开时 macOS 可能会拦截（在本机自行构建的通常不带隔离标记）。放行方法：

- **macOS 13–14**：在 Finder 中右键点击 App（或按住 Control 点击）→“打开”→ 再点“打开”。
- **macOS 15 及以上**：先尝试打开一次，然后进入“系统设置 → 隐私与安全性”，在“安全性”区域点“仍要打开”（该按钮在尝试打开后约一小时内可用），并输入登录密码。
- 或者在终端里移除隔离标记：

```sh
xattr -dr com.apple.quarantine /path/to/PreflopMatrix.app
```

详见 Apple 的说明：[在 Mac 上打开来自未知开发者的 App](https://support.apple.com/zh-cn/guide/mac-help/mh40616/mac)。

## 8. 开发

要求：macOS 13+、Go ≥ 1.26.10、Node（用 nvm 安装；版本需满足 `^20.19.0 || >=22.12.0`，项目的 `.nvmrc` 为 22，`nvm use` 即可；已用 v24 验证 `npm ci` 和 `npm run build`）、`wails3` 精确为 v3.0.0。

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
export PATH="$PATH:$(go env GOPATH)/bin"   # wails3 安装在 ~/go/bin，需要在 PATH 中
git config core.hooksPath .githooks         # 启用 pre-commit
cd frontend && npm ci                       # @wailsio/runtime、typescript、vite 均固定为精确版本
```

`.gitignore` 排除 `*.txt`（`testdata/parity/` 和 `internal/**/testdata/` 下的测试数据除外）、根目录的 `ranges/` 和 `Ranges/`、`*.html`、`*.json`（项目需要的几个除外）、`.env` 等。pre-commit 会拒绝任何匹配 `.gitignore` 的已暂存文件（包括用 `git add -f` 强行添加的），以及不区分大小写的 `.txt`（上述两处测试数据除外），避免范围文件、手牌、报告被提交。`wails3 build` 会先检查 `wails3 version` 必须是 v3.0.0。

`frontend/dist/` 的内容不进入版本控制，只保留 `.gitkeep`，所以干净的 clone 不必先构建前端就能直接运行 `go vet ./...`、`go test ./...`。

| 命令 | 说明 |
|---|---|
| `go test -short ./...` | 日常测试（跳过需要 Chrome 的测试；包含与 golden 的逐值比对） |
| `task test:report` | 报告测试，包含 headless Chrome（需要安装 Chrome）；提交前运行 |
| `npm run typecheck`（在 `frontend/` 中） | TypeScript 类型检查 |
| `wails3 build` | 构建，输出 `bin/PreflopMatrix` |
| `wails3 task darwin:package:universal` | 打包 arm64 + amd64 的 universal `bin/PreflopMatrix.app`（ad-hoc 签名） |
| `task dev`（即 `wails3 dev -config ./build/config.yml -port 9245`） | 开发模式 |

`task` 可以用 `brew install go-task` 安装，或者改用 `wails3 task <名称>`。打包 universal 之后如果要再运行 `wails3 build`，请先删除 universal 的 `bin/PreflopMatrix`，否则 `go build` 会拒绝覆盖这个 universal 文件而失败。

### Parity（与 Python 版比对）

```sh
tools/parity/make_golden.sh [PY_REPO]             # 用 Python 版重新生成 golden（Python 版仓库以参数指定，必须是干净的 checkout）
scripts/parity_real.sh <手牌文件夹> [范围文件夹]   # 用真实数据比对 Python 与 Go，输出不保留；范围默认 ~/.config/preflop-matrix/ranges（只读）；Python 版仓库用 PY_REPO 环境变量指定
go run ./tools/paritydiff-cli [--summary] a b     # 比对两个 JSON（加 --summary 则比对两份 stdout 摘要）
```

语料由 `tools/parity/make_corpus.py` 生成，golden 由 `tools/parity/make_golden.sh` 运行 Python 版生成；全部是合成数据，不包含任何真实手牌或范围。

### 开发用环境变量与模式

- `PMA_SETTINGS_PATH`：把设置文件指向别处，避免改动真实设置。**只在没有 `production` build tag 的构建中有效**（例如 `wails3 dev`）；`wails3 build`／`package` 的产物会忽略它。
- 纯前端开发（不需要 Go 后端，使用假报告）：

```sh
cd frontend && npm run dev:mock    # 等同于 VITE_MOCK=1 vite，地址 http://127.0.0.1:9245
```

## 9. 许可证

GPL-3.0-only。可以自由使用、修改、分发；分发本程序（含修改版）时，须以 GPL-3.0 授权并提供对应源代码。全文见 [LICENSE](LICENSE)。
