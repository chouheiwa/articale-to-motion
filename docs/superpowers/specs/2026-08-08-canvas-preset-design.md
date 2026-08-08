# 画幅预设（Canvas Preset）机制设计

- 日期：2026-08-08
- 状态：已确认，待实施
- 范围：本期落地 `vertical-9x16`，横屏 `landscape-16x9` 只留扩展位

## 1. 问题

画幅在仓库里不是配置项，而是硬编码在四层互不相通的地方：

| 层 | 位置 | 内容 |
|---|---|---|
| 校验 | `internal/validate/validate.go:210` | `canvas` 必须严格等于 `1080/1440/30/vertical` |
| 校验 | `internal/validate/validate.go:182` | 风格示例 PNG 必须是 1080×1440 |
| 渲染契约 | `internal/scene/scene.go:154` | 提示词字符串常量 `制作 1080x1440、30fps、静音…` |
| 风格 token | `frame.md:12-15`、`docs/清晰系统蓝图-视频风格说明书.md:12-15` | 两份必须 `reflect.DeepEqual` |
| 交付文档 | `PROMPT-PRODUCTION.md:36/271/328/397` | 成片验收条款 |
| 素材 | `assets/style-guide/examples/*.svg` | 4 张原型图的 viewBox |

`internal/config/config.go` 的 `Config` 结构没有画幅维度。

其中第 3 项是**一致性漏洞**：校验层和执行层各说各的，只是恰好都等于 1080×1440 才没暴露。即使 token 改成别的尺寸，渲染 agent 收到的仍是 `1080x1440`，会静默产出错画幅。

## 2. 目标与非目标

**目标**

- 画幅成为 `am init` 时的一次性选择，项目内仍只有一份 `frame.md`
- 新增 `vertical-9x16`（1080×1920）预设并端到端跑通
- 消除 `scene.go` 的硬编码，让执行层与校验层共用同一个真相源
- 对存量 1080×1440 项目零破坏

**非目标（YAGNI）**

- 横屏 16:9 的实际数值与示例图（只在 preset 表留位）
- 任意自定义画幅（`--canvas 1234x5678`）
- 同一项目内多画幅并存或运行时切换
- SVG 示例图的程序化重排

## 3. 关键设计决策

### 3.1 anchor 语义的安全区推导

`safe_area` 五个框在内置 preset 表里以 anchor 形式声明，`go generate` 时解析成绝对像素写入 `frame.md`。**anchor 只存在于构建期，不进项目文件**——`frame.md` 的 schema 与今天完全一致，渲染 agent 拿到的仍是现成像素值，无需自行运算。

三种 anchor：

| anchor | 参数 | top_px | bottom_px |
|---|---|---|---|
| `Top` | `InsetTopPx`, `HeightPx` | `InsetTopPx` | `H - InsetTopPx - HeightPx` |
| `Bottom` | `InsetBottomPx`, `HeightPx` | `H - InsetBottomPx - HeightPx` | `InsetBottomPx` |
| `Fill` | `InsetTopPx`, `InsetBottomPx` | `InsetTopPx` | `InsetBottomPx` |

`left_px` / `right_px` 不参与推导，直接声明（本期两套预设宽度都是 1080）。

**推导表与验证：**

| 安全区 | anchor | 参数 | → 3:4 (H=1440) | → 9:16 (H=1920) |
|---|---|---|---|---|
| `structural` | Fill | top 96 / bottom 60 | `96 / 60` | `96 / 60` |
| `main_content` | Fill | top 120 / bottom 100 | `120 / 100` | `120 / 100` |
| `critical_text` | Fill | top 120 / bottom 260 | `120 / 260` | `120 / 260` |
| `cover_title` | Top | inset 260 / 高 720 | `260 / 460` | `260 / 940` |
| `subtitles` | Bottom | inset 260 / 高 190 | `990 / 260` | `1470 / 260` |

3:4 那一列逐值等于今天 `frame.md` 的原值。这是零破坏的基础，也是回归测试的断言内容。

语义：字幕带（190px 高）与封面标题块（720px 高）保持贴底/贴顶的固定边距，多出的 480px 全部归给 `main_content` 的可用垂直空间。

### 3.2 宽度不变带来的简化

1080×1440 → 1080×1920 宽度未变，因此：

- `typography.sizes_px` / `weights` / `line_heights` 整套沿用，9:16 不覆盖
- `safe_area` 的 `left_px` / `right_px` 全部沿用

`Preset` 结构保留可选的 typography 覆盖字段，供横屏（1920×1080，宽度变化）将来使用。

### 3.3 生成策略：`go generate` 预生成并签入

preset 表是唯一真相源，生成器把每套完整文件写进 `assets/presets/<id>/`。`am init` 仍是纯拷贝，逻辑不变。生成物可审 diff。

CI 增加一步 `go generate ./... && git diff --exit-code`，防止手改生成物导致真相源与产物分叉。

### 3.4 示例图手写

4 张原型示意图（`proposition` / `comparison` / `process` / `capability_deck`）是手工绘制的绝对坐标插图（40–77 行），每套预设手写一份 1080×1920 版本。它们是给人看的排版基准，拉伸出来的不能当基准。

PNG 与 contact-sheet 仍由 `magick` 自动产出，复用 `RegenerateExamples` 现有的颜色替换逻辑。

## 4. 架构

### 4.1 新包 `internal/preset`

```
type Anchor int  // AnchorTop | AnchorBottom | AnchorFill

type Canvas struct {
    WidthPx, HeightPx, FPS int
    Orientation string      // vertical | horizontal
}

type Box struct {
    LeftPx, RightPx int
    Anchor          Anchor
    InsetTopPx      int  // Top / Fill
    InsetBottomPx   int  // Bottom / Fill
    HeightPx        int  // Top / Bottom
}

type Preset struct {
    ID, Label  string
    Canvas     Canvas
    SafeArea   []NamedBox            // 有序，与 frame.md 字段顺序一致
    Typography *TypographyOverride   // nil 表示沿用 base
}
```

对外暴露：

- `All() []Preset` — 有序列表，供选择框与错误提示使用
- `ByID(id string) (Preset, bool)`
- `ByCanvas(w, h, fps int, orientation string) (Preset, bool)` — 供 `validate` 反查
- `(Preset) ResolveSafeArea() map[string]ResolvedBox` — anchor → 绝对像素

与画幅无关的 token（`colors` / `motion` / `audio` / `spacing` / `radius` / `subtitles` / `cover` / `forbidden`）放在共享 base，不随 preset 复制。

本期内置：`vertical-3x4`（1080×1440，默认）、`vertical-9x16`（1080×1920）。

### 4.2 素材重组

按「是否含画幅数字」拆成两个源树。**每个源树的内部路径就是它在项目根下的目标路径**，`am init` 把两棵树叠加拷贝，不做任何路径改写：

```
assets/
  shared/                       # 与画幅无关，两套预设共用
    PROMPT.md
    article-to-motion.conf
    .env.example
    templates/publish.md
    assets/fonts/               # → 项目 assets/fonts/
      noto-sans-sc-{400,600,700,900}.woff2
      README.md
      LICENSE-Noto-Sans-SC.txt

  presets/
    vertical-3x4/               # 含画幅数字，每套一份
      frame.md
      PROMPT-PRODUCTION.md
      docs/清晰系统蓝图-视频风格说明书.md
      assets/style-guide/examples/*.svg|*.png
    vertical-9x16/              # 同构
```

`shared/assets/fonts/` 这一层嵌套是刻意的：它对应项目里的 `assets/fonts/`。preset 目录下的 `assets/style-guide/` 同理。

仓库根现有的 `frame.md`、`PROMPT-PRODUCTION.md`、`docs/清晰系统蓝图-视频风格说明书.md`、`assets/style-guide/` 移入 `assets/presets/vertical-3x4/`；`PROMPT.md`、`article-to-motion.conf`、`.env.example`、`templates/publish.md`、`assets/fonts/` 移入 `assets/shared/`。`assets.go` 的 `//go:embed` 指令相应更新为 `assets/shared` 与 `assets/presets` 两棵树。

### 4.3 生成器

`internal/preset/gen`（`//go:generate` 驱动），对每套 preset：

1. 由 `Canvas` + `ResolveSafeArea()` + base token 渲染 `frame.md` 的 YAML frontmatter，正文取自模板
2. 渲染 `docs/清晰系统蓝图-视频风格说明书.md`，frontmatter 与 `frame.md` **逐字节相同**（`validate` 要求 `reflect.DeepEqual`），正文替换 3 处画幅数字（原 171 / 467 / 481 行）
3. 渲染 `PROMPT-PRODUCTION.md`，替换 4 处画幅数字（原 36 / 271 / 328 / 397 行）
4. 调 `magick` 把手写 SVG 转 PNG，再拼 contact-sheet

第 4 步需要 ImageMagick。`magick` 缺失时生成器报错退出（非零退出码），不静默产出残缺目录。

因此 **CI 跑 `go generate` 校验的那个 job 必须预装 ImageMagick**。日常 `go build` / `go test` 不需要——PNG 是签入产物，只有重新生成时才用得上。

### 4.4 `am init` 交互

```
am init [DIR] [--canvas <id>] [--skip-hyperframes]
```

`<id>` 取 preset 的完整 id：`vertical-3x4` 或 `vertical-9x16`。不设短别名——将来横屏是 `landscape-16x9`，`9x16` / `16x9` 这类简写在视觉上极易混淆。

画幅来源判定：

1. `--canvas` 显式传入 → 用它；id 未知则报错并列出可选值
2. 未传且 stdin 是 TTY → bubbletea 方向键选择框，默认高亮 `vertical-3x4`
3. 未传且非 TTY → **报错退出**，提示必须显式传 `--canvas`

第 3 条采用失败关闭，与项目现有安全模式风格一致（`internal/cli/cli.go` 的受限子进程在能力不足时也是失败关闭而非降级）。

拷贝范围为 `presets/<选中>/**` + `shared/**` + `fonts/**` 三者合并进项目根。`internal/project/init.go` 现有的「目标文件已存在且内容不同则报错」与失败回滚逻辑原样保留，仅把单一 `fs.FS` 输入改为多源合并。

新增依赖 `github.com/charmbracelet/bubbletea`（约 12 个间接依赖）。

### 4.5 `validate` 改造

`internal/validate/validate.go`：

- `validateStyleSchema` 中 `canvas` 的四值硬编码（:210）→ 调 `preset.ByCanvas` 反查；查不到时报错并列出所有内置画幅
- `safe_area` 校验从「字段齐全且非负」升级为「**必须逐值等于该 preset 的 anchor 推导结果**」。今天手改 `subtitles.top_px` 查不出来，改完能查出
- 示例 PNG 尺寸断言（:182）→ 按 `canvas.width_px` / `canvas.height_px` 而非常量
- `typography` 若该 preset 声明了覆盖，按覆盖值校验；否则按 base

`RegenerateExamples` 的 SVG 模板来源从固定路径改为按项目 `frame.md` 的 canvas 反查 preset 目录。

### 4.6 修复 `scene.go` 一致性漏洞

`BuildPrompt`（`internal/scene/scene.go:132`）：

- 当 `s.StyleGuide` 非空 → 读该文件 frontmatter 的 `canvas`，把 `width_px × height_px`、`fps` 写进提示词，删除写死的 `1080x1440、30fps`
- 当 `s.StyleGuide` 为空 → 沿用 1080×1440、30fps 并向 stderr 打印警告

第二条与今天行为等价，保证存量无 `style_guide` 的镜头目录不受影响。`PROMPT.md:30` 与 `PROMPT-PRODUCTION.md:268` 均要求编排器声明 `style_guide`，实践中第一条是常态路径。

`contained()` 已保证 `StyleGuide` 路径不逃出镜头目录且文件存在，无需额外防护。

## 5. 错误处理

| 场景 | 行为 |
|---|---|
| `--canvas` 传了未知 id | 报错，列出全部内置 id 与尺寸 |
| 非交互环境未传 `--canvas` | 报错，提示显式传入 |
| `frame.md` 的 canvas 不匹配任何 preset | `validate` 报错，列出可选画幅 |
| `safe_area` 与 anchor 推导不符 | `validate` 报错，指出字段名、期望值、实际值 |
| 示例 PNG 尺寸与 canvas 不符 | `validate` 报错，给出路径与两边尺寸 |
| 生成器找不到 `magick` | 提示缺失并跳过 PNG，退出码非零 |
| `am init` 目标已存在同名不同内容文件 | 沿用现有行为：报错 + 回滚已写入文件 |

## 6. 测试

| 层 | 用例 |
|---|---|
| `internal/preset` | 表驱动 anchor 推导；**3:4 推导结果逐值等于今天 `frame.md` 的字面值**（回归锁）；`ByCanvas` 反查命中与未命中 |
| 生成器 | `go generate` 幂等：CI 跑完 `git diff --exit-code` 为空 |
| `internal/project` | 两套 preset 各 init 一次，产物齐全；`fonts` 只有一份；已存在冲突文件时回滚 |
| `internal/validate` | 9:16 项目通过校验；手改任一 `safe_area` 数字被拒；示例 PNG 尺寸错被拒；未知 canvas 被拒 |
| `internal/scene` | 9:16 的 `style_guide` 下 `BuildPrompt` 产出 `1080x1920`；无 `style_guide` 时产出 `1080x1440` 并告警 |
| `internal/cli` | `--canvas` 未知值报错；非 TTY 无 flag 报错 |

全部沿用现有 `go test ./...` / `-race` / `go vet` 流程。

## 7. 验收标准

1. `go generate ./...` 后 `git diff` 对 `assets/presets/vertical-3x4/` 为空——生成的 `frame.md` 与迁移前的仓库根 `frame.md` 逐字节相同。否则存量项目重跑 `am init` 会撞「目标文件已存在且内容不同」
2. `am init demo --canvas vertical-9x16` 产出的项目通过 `am validate style --project-root demo`
3. 该项目的一个镜头经 `BuildPrompt` 产出的提示词包含 `1080x1920`，不含 `1080x1440`
4. `am init demo2`（TTY）弹出选择框；`am init demo2 < /dev/null` 报错退出
5. `go test ./... && go test -race ./... && go vet ./...` 全绿

## 8. 后续（不在本期）

横屏 `landscape-16x9`（1920×1080）：宽度从 1080 变为 1920，需要重定 `typography.sizes_px` 全套阶梯与 `safe_area` 的 `left_px` / `right_px`，并手写 4 张横版示例图。`Preset.Typography` 覆盖字段与 `Box` 的左右声明已为此预留。
