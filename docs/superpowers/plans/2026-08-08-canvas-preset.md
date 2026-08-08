# 画幅预设机制 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把画幅从六处硬编码收敛成 `am init` 时选定的预设，落地 `vertical-9x16`，并消除校验层与渲染执行层各说各的一致性漏洞。

**Architecture:** 新增 `internal/preset` 作为唯一真相源，用 anchor 语义在构建期把安全区推导成绝对像素；`go generate` 把每套预设的完整文件写进 `assets/presets/<id>/`；`am init` 退化为纯拷贝两棵源树。`validate` 与 `scene.BuildPrompt` 都改为向 `internal/preset` 反查，不再持有常量。

**Tech Stack:** Go 1.25.8、cobra、yaml.v3、bubbletea（新增）、ImageMagick（仅生成期）

**Spec:** `docs/superpowers/specs/2026-08-08-canvas-preset-design.md`

## Global Constraints

- 提交信息禁止 `Co-Authored-By` 行；格式 `<type>: <描述>`，type 取 feat/fix/refactor/docs/test/chore
- 所有面向用户的字符串、错误信息、注释一律中文，与现有代码保持一致
- 每个任务结束前必须 `go test ./... && go vet ./...` 全绿
- **零破坏红线**：`assets/presets/vertical-3x4/` 下的 `frame.md`、`PROMPT-PRODUCTION.md`、`docs/清晰系统蓝图-视频风格说明书.md` 必须与迁移前仓库根的同名文件**逐字节相同**。否则存量项目重跑 `am init` 会撞「目标文件已存在且内容不同」
- 预设 id 用完整形式 `vertical-3x4` / `vertical-9x16`，不设 `9x16` 之类短别名
- 安全区推导表两套预设共用同一份 anchor 声明——这是设计意图，不要为 9:16 复制一份
- 失败关闭：任何画幅解析不出来的情况都报错退出，不静默取默认

## 关键实现约束：生成器不做 YAML 重序列化

`frame.md` 的 frontmatter 用了 flow 映射（`{left_px: 40, right_px: 40, top_px: 96, bottom_px: 60}`）、`0.30` 这类保留尾零的浮点写法，以及人工排定的键序。`yaml.Marshal` 一律还原不出来，会让「逐字节相同」红线立刻失守。

**生成器只做两件事：**
1. 用生成文本整体替换 `canvas:` 块和 `safe_area:` 块（两块的行边界在模板里用标记划定）
2. 正文里 `{{CANVAS}}` 占位符替换成 `1080×1440` 形式的字符串（全角 `×`）

frontmatter 其余部分、正文其余部分原样透传。

---

## 文件结构

| 文件 | 职责 |
|---|---|
| `internal/preset/preset.go` | 类型定义（`Anchor` / `Canvas` / `Box` / `ResolvedBox` / `Preset`）与推导逻辑 |
| `internal/preset/table.go` | 两套内置预设的声明表与查找函数 |
| `internal/preset/preset_test.go` | anchor 推导表驱动测试 + 3:4 回归锁 |
| `internal/preset/gen/main.go` | `go generate` 入口，写出 `assets/presets/<id>/` |
| `internal/preset/gen/templates/frontmatter.yaml` | 共享 frontmatter 模板，含两个替换标记 |
| `internal/preset/gen/templates/frame.body.md` | `frame.md` 正文模板 |
| `internal/preset/gen/templates/style-guide.body.md` | 说明书正文模板 |
| `internal/preset/gen/templates/PROMPT-PRODUCTION.md` | 制作提示词模板 |
| `assets/shared/**` | 与画幅无关的项目模板（路径即项目内目标路径） |
| `assets/presets/<id>/**` | 含画幅数字的生成产物（路径即项目内目标路径） |
| `assets.go` | embed 指令改为两棵树 |
| `internal/project/init.go` | `Initialize` 接受多个 `fs.FS` 源并叠加拷贝 |
| `internal/cli/canvas.go` | `--canvas` 解析与 bubbletea 选择框 |
| `internal/validate/validate.go` | 画幅常量改为向 `preset` 反查 |
| `internal/scene/scene.go` | `BuildPrompt` 从 `style_guide` 读 canvas |

---

### Task 1: `internal/preset` 包与 anchor 推导

**Files:**
- Create: `internal/preset/preset.go`
- Create: `internal/preset/table.go`
- Test: `internal/preset/preset_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
  - `type Anchor string`，常量 `AnchorTop` / `AnchorBottom` / `AnchorFill`
  - `type Canvas struct { WidthPx, HeightPx, FPS int; Orientation string }`
  - `func (c Canvas) Label() string` → `"1080×1440"`（全角 ×）
  - `type Box struct { Name string; LeftPx, RightPx int; Anchor Anchor; InsetTopPx, InsetBottomPx, HeightPx int }`
  - `type ResolvedBox struct { LeftPx, RightPx, TopPx, BottomPx int }`
  - `func (b Box) Resolve(canvasHeight int) (ResolvedBox, error)`
  - `type Preset struct { ID, Label string; Canvas Canvas; SafeArea []Box }`
  - `func (p Preset) ResolveSafeArea() ([]NamedBox, error)`，`type NamedBox struct { Name string; Box ResolvedBox }`
  - `func All() []Preset`、`func ByID(id string) (Preset, bool)`、`func ByCanvas(width, height, fps int, orientation string) (Preset, bool)`、`func Default() Preset`、`func IDs() []string`

- [ ] **Step 1: 写失败测试**

创建 `internal/preset/preset_test.go`：

```go
package preset

import "testing"

// 3:4 的推导结果必须逐值等于仓库现有 frame.md 的字面值。
// 这是整个预设机制的零破坏地基，改坏了存量项目会撞 am init 的内容冲突检查。
func TestVertical3x4MatchesShippedFrameValues(t *testing.T) {
	assertSafeArea(t, "vertical-3x4", map[string]ResolvedBox{
		"structural":    {LeftPx: 40, RightPx: 40, TopPx: 96, BottomPx: 60},
		"main_content":  {LeftPx: 88, RightPx: 88, TopPx: 120, BottomPx: 100},
		"critical_text": {LeftPx: 88, RightPx: 180, TopPx: 120, BottomPx: 260},
		"cover_title":   {LeftPx: 88, RightPx: 180, TopPx: 260, BottomPx: 460},
		"subtitles":     {LeftPx: 88, RightPx: 180, TopPx: 990, BottomPx: 260},
	})
}

func TestVertical9x16DerivesFromSameAnchors(t *testing.T) {
	assertSafeArea(t, "vertical-9x16", map[string]ResolvedBox{
		"structural":    {LeftPx: 40, RightPx: 40, TopPx: 96, BottomPx: 60},
		"main_content":  {LeftPx: 88, RightPx: 88, TopPx: 120, BottomPx: 100},
		"critical_text": {LeftPx: 88, RightPx: 180, TopPx: 120, BottomPx: 260},
		"cover_title":   {LeftPx: 88, RightPx: 180, TopPx: 260, BottomPx: 940},
		"subtitles":     {LeftPx: 88, RightPx: 180, TopPx: 1470, BottomPx: 260},
	})
}

func assertSafeArea(t *testing.T, id string, want map[string]ResolvedBox) {
	t.Helper()
	p, ok := ByID(id)
	if !ok {
		t.Fatalf("找不到预设 %s", id)
	}
	boxes, err := p.ResolveSafeArea()
	if err != nil {
		t.Fatalf("推导失败：%v", err)
	}
	if len(boxes) != len(want) {
		t.Fatalf("安全区数量 = %d，期望 %d", len(boxes), len(want))
	}
	for _, item := range boxes {
		expected, ok := want[item.Name]
		if !ok {
			t.Fatalf("多出安全区 %s", item.Name)
		}
		if item.Box != expected {
			t.Errorf("%s = %+v，期望 %+v", item.Name, item.Box, expected)
		}
	}
}

// 安全区顺序必须与 frame.md 的字段顺序一致，生成器直接按此顺序写文件。
func TestSafeAreaOrderIsStable(t *testing.T) {
	p, _ := ByID("vertical-3x4")
	boxes, err := p.ResolveSafeArea()
	if err != nil {
		t.Fatalf("推导失败：%v", err)
	}
	want := []string{"structural", "main_content", "critical_text", "cover_title", "subtitles"}
	for i, name := range want {
		if boxes[i].Name != name {
			t.Errorf("第 %d 项 = %s，期望 %s", i, boxes[i].Name, name)
		}
	}
}

func TestResolveRejectsOverflow(t *testing.T) {
	b := Box{Name: "subtitles", Anchor: AnchorBottom, InsetBottomPx: 260, HeightPx: 190}
	if _, err := b.Resolve(400); err == nil {
		t.Fatal("画布高度不足时应报错")
	}
}

func TestByCanvasLookup(t *testing.T) {
	p, ok := ByCanvas(1080, 1920, 30, "vertical")
	if !ok || p.ID != "vertical-9x16" {
		t.Fatalf("反查 = %v %v，期望 vertical-9x16", p.ID, ok)
	}
	if _, ok := ByCanvas(1920, 1080, 30, "horizontal"); ok {
		t.Fatal("未内置的画幅不应命中")
	}
}

func TestCanvasLabelUsesFullWidthMultiplicationSign(t *testing.T) {
	got := Canvas{WidthPx: 1080, HeightPx: 1440}.Label()
	if got != "1080×1440" {
		t.Fatalf("Label = %q，期望 1080×1440", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/preset/ -v`
Expected: 编译失败，`undefined: ByID` 等

- [ ] **Step 3: 写 `internal/preset/preset.go`**

```go
// Package preset 是画幅预设的唯一真相源。
//
// 安全区在这里用 anchor 语义声明，由 go generate 在构建期推导成绝对像素写进
// frame.md。anchor 不进项目文件：渲染 agent 读到的始终是现成像素值，无需自行运算。
package preset

import "fmt"

type Anchor string

const (
	// AnchorTop 贴顶：距顶固定，高度固定，底部随画布高度浮动。
	AnchorTop Anchor = "top"
	// AnchorBottom 贴底：距底固定，高度固定，顶部随画布高度浮动。
	AnchorBottom Anchor = "bottom"
	// AnchorFill 上下都固定边距，高度随画布吸收剩余空间。
	AnchorFill Anchor = "fill"
)

type Canvas struct {
	WidthPx     int
	HeightPx    int
	FPS         int
	Orientation string
}

// Label 返回人类可读画幅，使用全角乘号，与文档正文里的写法一致。
func (c Canvas) Label() string {
	return fmt.Sprintf("%d×%d", c.WidthPx, c.HeightPx)
}

type Box struct {
	Name          string
	LeftPx        int
	RightPx       int
	Anchor        Anchor
	InsetTopPx    int // AnchorTop / AnchorFill
	InsetBottomPx int // AnchorBottom / AnchorFill
	HeightPx      int // AnchorTop / AnchorBottom
}

// ResolvedBox 用与 frame.md 一致的四边内缩表示，不是坐标。
type ResolvedBox struct {
	LeftPx   int
	RightPx  int
	TopPx    int
	BottomPx int
}

func (b Box) Resolve(canvasHeight int) (ResolvedBox, error) {
	out := ResolvedBox{LeftPx: b.LeftPx, RightPx: b.RightPx}
	switch b.Anchor {
	case AnchorTop:
		out.TopPx = b.InsetTopPx
		out.BottomPx = canvasHeight - b.InsetTopPx - b.HeightPx
	case AnchorBottom:
		out.BottomPx = b.InsetBottomPx
		out.TopPx = canvasHeight - b.InsetBottomPx - b.HeightPx
	case AnchorFill:
		out.TopPx = b.InsetTopPx
		out.BottomPx = b.InsetBottomPx
	default:
		return ResolvedBox{}, fmt.Errorf("安全区 %s 的 anchor 无效：%s", b.Name, b.Anchor)
	}
	if out.TopPx < 0 || out.BottomPx < 0 || out.TopPx+out.BottomPx >= canvasHeight {
		return ResolvedBox{}, fmt.Errorf("安全区 %s 在 %dpx 画布高度下无效：top=%d bottom=%d", b.Name, canvasHeight, out.TopPx, out.BottomPx)
	}
	return out, nil
}

type NamedBox struct {
	Name string
	Box  ResolvedBox
}

type Preset struct {
	ID       string
	Label    string
	Canvas   Canvas
	SafeArea []Box
}

// ResolveSafeArea 按声明顺序推导全部安全区，顺序与 frame.md 字段顺序一致。
func (p Preset) ResolveSafeArea() ([]NamedBox, error) {
	out := make([]NamedBox, 0, len(p.SafeArea))
	for _, box := range p.SafeArea {
		resolved, err := box.Resolve(p.Canvas.HeightPx)
		if err != nil {
			return nil, fmt.Errorf("预设 %s：%w", p.ID, err)
		}
		out = append(out, NamedBox{Name: box.Name, Box: resolved})
	}
	return out, nil
}
```

- [ ] **Step 4: 写 `internal/preset/table.go`**

```go
package preset

// baseSafeArea 是两套竖屏预设共用的 anchor 声明。
//
// 共用是设计意图而非巧合：1080×1440 与 1080×1920 宽度相同，字幕带（190px 高）
// 与封面标题块（720px 高）保持同样的贴底/贴顶边距，多出的 480px 全部归给
// main_content 的可用垂直空间。新增竖屏画幅不应复制这张表。
func baseSafeArea() []Box {
	return []Box{
		{Name: "structural", LeftPx: 40, RightPx: 40, Anchor: AnchorFill, InsetTopPx: 96, InsetBottomPx: 60},
		{Name: "main_content", LeftPx: 88, RightPx: 88, Anchor: AnchorFill, InsetTopPx: 120, InsetBottomPx: 100},
		{Name: "critical_text", LeftPx: 88, RightPx: 180, Anchor: AnchorFill, InsetTopPx: 120, InsetBottomPx: 260},
		{Name: "cover_title", LeftPx: 88, RightPx: 180, Anchor: AnchorTop, InsetTopPx: 260, HeightPx: 720},
		{Name: "subtitles", LeftPx: 88, RightPx: 180, Anchor: AnchorBottom, InsetBottomPx: 260, HeightPx: 190},
	}
}

// 顺序即 am init 选择框里的展示顺序，第一项是默认值。
var builtin = []Preset{
	{
		ID:       "vertical-3x4",
		Label:    "3:4  竖屏  1080×1440",
		Canvas:   Canvas{WidthPx: 1080, HeightPx: 1440, FPS: 30, Orientation: "vertical"},
		SafeArea: baseSafeArea(),
	},
	{
		ID:       "vertical-9x16",
		Label:    "9:16 竖屏  1080×1920",
		Canvas:   Canvas{WidthPx: 1080, HeightPx: 1920, FPS: 30, Orientation: "vertical"},
		SafeArea: baseSafeArea(),
	},
}

func All() []Preset {
	out := make([]Preset, len(builtin))
	copy(out, builtin)
	return out
}

func Default() Preset { return builtin[0] }

func IDs() []string {
	out := make([]string, 0, len(builtin))
	for _, p := range builtin {
		out = append(out, p.ID)
	}
	return out
}

func ByID(id string) (Preset, bool) {
	for _, p := range builtin {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// ByCanvas 供 validate 从 frame.md 的 canvas 反查预设。
func ByCanvas(width, height, fps int, orientation string) (Preset, bool) {
	for _, p := range builtin {
		c := p.Canvas
		if c.WidthPx == width && c.HeightPx == height && c.FPS == fps && c.Orientation == orientation {
			return p, true
		}
	}
	return Preset{}, false
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/preset/ -v && go vet ./internal/preset/`
Expected: 全部 PASS

- [ ] **Step 6: 提交**

```bash
git add internal/preset/
git commit -m "feat: 新增 internal/preset 画幅预设表与 anchor 安全区推导"
```

---

### Task 2: 素材重组与多源初始化

把仓库根的模板文件搬进两棵源树，`Initialize` 改为接受多个 `fs.FS`。本任务**不改变任何对外行为**——init 产出的项目与今天逐字节一致。

**Files:**
- Move: `frame.md` → `assets/presets/vertical-3x4/frame.md`
- Move: `PROMPT-PRODUCTION.md` → `assets/presets/vertical-3x4/PROMPT-PRODUCTION.md`
- Move: `docs/清晰系统蓝图-视频风格说明书.md` → `assets/presets/vertical-3x4/docs/清晰系统蓝图-视频风格说明书.md`
- Move: `assets/style-guide/` → `assets/presets/vertical-3x4/assets/style-guide/`
- Move: `PROMPT.md`, `article-to-motion.conf`, `.env.example` → `assets/shared/`
- Move: `templates/publish.md` → `assets/shared/templates/publish.md`
- Move: `assets/fonts/` → `assets/shared/assets/fonts/`
- Modify: `assets.go`
- Modify: `internal/project/init.go`
- Modify: `internal/cli/cli.go:111`
- Modify: `internal/validate/validate.go:528`
- Test: `internal/project/init_test.go`

**Interfaces:**
- Consumes: `preset.Default()`（Task 1）
- Produces:
  - `func project.Initialize(target string, sources ...fs.FS) (InitResult, error)` — 可变参数，多棵树叠加拷贝
  - `assets.Shared() fs.FS`、`assets.Preset(id string) (fs.FS, error)`

- [ ] **Step 1: 移动文件（保留 git 历史）**

```bash
mkdir -p assets/presets/vertical-3x4/docs assets/shared/templates assets/shared/assets
git mv frame.md assets/presets/vertical-3x4/frame.md
git mv PROMPT-PRODUCTION.md assets/presets/vertical-3x4/PROMPT-PRODUCTION.md
git mv "docs/清晰系统蓝图-视频风格说明书.md" "assets/presets/vertical-3x4/docs/清晰系统蓝图-视频风格说明书.md"
git mv assets/style-guide assets/presets/vertical-3x4/assets/style-guide
git mv PROMPT.md assets/shared/PROMPT.md
git mv article-to-motion.conf assets/shared/article-to-motion.conf
git mv .env.example assets/shared/.env.example
git mv templates/publish.md assets/shared/templates/publish.md
git mv assets/fonts assets/shared/assets/fonts
rmdir templates docs 2>/dev/null || true
```

注意 `assets/presets/vertical-3x4/assets/style-guide/` 与 `assets/shared/assets/fonts/` 这两层嵌套是刻意的：源树内部路径就是它在项目根下的目标路径。

- [ ] **Step 2: 改 `assets.go`**

```go
package assets

import (
	"embed"
	"fmt"
	"io/fs"
)

// Files 保存 am 二进制自带的项目骨架，分两棵源树：
//
//	assets/shared/    与画幅无关，所有预设共用
//	assets/presets/   含画幅数字，每套预设一份（由 go generate 产出）
//
// 每棵树的内部路径就是它在项目根下的目标路径，Initialize 直接叠加拷贝，
// 不做任何路径改写。assets/shared/assets/fonts 里的 CJK 字体必须随项目走：
// 渲染机是干净的无头 Chrome，字体栈里没有 @font-face 的字体族会静默回退。
//
//go:embed assets/shared assets/presets
var Files embed.FS

func Shared() (fs.FS, error) {
	return fs.Sub(Files, "assets/shared")
}

func Preset(id string) (fs.FS, error) {
	sub, err := fs.Sub(Files, "assets/presets/"+id)
	if err != nil {
		return nil, fmt.Errorf("找不到预设素材 %s: %w", id, err)
	}
	if _, err := fs.Stat(sub, "frame.md"); err != nil {
		return nil, fmt.Errorf("预设素材 %s 不完整，缺少 frame.md", id)
	}
	return sub, nil
}
```

`//go:embed` 默认忽略 `.` 开头的文件，`assets/shared/.env.example` 不会被打进去。用显式模式补上：把 embed 指令改为

```go
//go:embed assets/shared assets/presets assets/shared/.env.example
```

- [ ] **Step 3: 写 `Initialize` 多源的失败测试**

在 `internal/project/init_test.go` 追加：

```go
func TestInitializeMergesMultipleSources(t *testing.T) {
	shared := fstest.MapFS{
		"PROMPT.md":                 {Data: []byte("shared\n")},
		"assets/fonts/noto.woff2":   {Data: []byte("font\n")},
	}
	chosen := fstest.MapFS{
		"frame.md":                            {Data: []byte("frame\n")},
		"assets/style-guide/examples/a.svg":   {Data: []byte("svg\n")},
	}
	target := t.TempDir()
	result, err := Initialize(target, shared, chosen)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if result.Created != 4 {
		t.Fatalf("Created = %d，期望 4", result.Created)
	}
	for _, name := range []string{"PROMPT.md", "assets/fonts/noto.woff2", "frame.md", "assets/style-guide/examples/a.svg"} {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(name))); err != nil {
			t.Errorf("缺少 %s: %v", name, err)
		}
	}
}

// 两棵源树写同一路径属于素材组织错误，必须报错而不是让后者静默覆盖前者。
func TestInitializeRejectsOverlappingSources(t *testing.T) {
	a := fstest.MapFS{"frame.md": {Data: []byte("a\n")}}
	b := fstest.MapFS{"frame.md": {Data: []byte("b\n")}}
	if _, err := Initialize(t.TempDir(), a, b); err == nil {
		t.Fatal("源树路径冲突时应报错")
	}
}
```

需要 `import "testing/fstest"`。

- [ ] **Step 4: 跑测试确认失败**

Run: `go test ./internal/project/ -run TestInitializeMerges -v`
Expected: 编译失败，`too many arguments in call to Initialize`

- [ ] **Step 5: 改 `internal/project/init.go` 的 `Initialize` 签名与收集逻辑**

把原来单个 `source fs.FS` 的签名与 walk 循环替换为：

```go
func Initialize(target string, sources ...fs.FS) (InitResult, error) {
	target, _ = filepath.Abs(target)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("创建项目目录: %w", err)
	}
	var assets []asset
	seen := make(map[string]bool)
	for _, source := range sources {
		err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if seen[path] {
				return fmt.Errorf("多棵源树写入同一路径：%s", path)
			}
			seen[path] = true
			body, err := fs.ReadFile(source, path)
			if err != nil {
				return err
			}
			assets = append(assets, asset{name: path, body: body})
			return nil
		})
		if err != nil {
			return InitResult{}, fmt.Errorf("读取内置项目模板: %w", err)
		}
	}
	// ……以下冲突检查、写入、回滚逻辑完全不动
```

原先排除 `assets.go` / `go.mod` 的两行判断可以删掉——源树用 `fs.Sub` 裁过，不再包含仓库自身文件。

- [ ] **Step 6: 跑测试确认通过**

Run: `go test ./internal/project/ -v`
Expected: 全部 PASS

- [ ] **Step 7: 改 `internal/cli/cli.go` 的 init 调用点**

`internal/cli/cli.go:111` 由

```go
result, err := project.Initialize(target, assets.Files)
```

改为

```go
shared, err := assets.Shared()
if err != nil {
    return err
}
chosen, err := assets.Preset(preset.Default().ID)
if err != nil {
    return err
}
result, err := project.Initialize(target, shared, chosen)
if err != nil {
    return err
}
```

并 import `"github.com/chouheiwa/articale-to-motion/internal/preset"`。（`--canvas` 在 Task 5 接上，这里先固定默认预设以保持行为不变。）

- [ ] **Step 8: 改 `RegenerateExamples` 的模板路径**

`internal/validate/validate.go:528` 由

```go
sourcePath := "assets/style-guide/examples/" + name + ".svg"
```

改为

```go
// 预设感知在 Task 7 接上，这里先跟随素材重组指向默认预设，保持行为不变。
sourcePath := "assets/presets/" + preset.Default().ID + "/assets/style-guide/examples/" + name + ".svg"
```

并 import preset 包。

- [ ] **Step 9: 全量回归**

Run: `go build ./cmd/am && go test ./... && go vet ./...`
Expected: 全绿

- [ ] **Step 10: 端到端验证素材重组无行为变化**

```bash
rm -rf /tmp/am-check && ./am init /tmp/am-check --skip-hyperframes
ls /tmp/am-check
```
Expected: 项目根出现 `frame.md`、`PROMPT.md`、`PROMPT-PRODUCTION.md`、`article-to-motion.conf`、`.env.example`、`docs/`、`templates/`、`assets/fonts/`、`assets/style-guide/`，与重组前一致

- [ ] **Step 11: 提交**

```bash
git add -A
git commit -m "refactor: 素材拆成 shared/presets 两棵源树，Initialize 支持多源叠加"
```

---

### Task 3: 生成器与 3:4 逐字节复现

**Files:**
- Create: `internal/preset/gen/main.go`
- Create: `internal/preset/gen/templates/frontmatter.yaml`
- Create: `internal/preset/gen/templates/frame.body.md`
- Create: `internal/preset/gen/templates/style-guide.body.md`
- Create: `internal/preset/gen/templates/PROMPT-PRODUCTION.md`
- Create: `internal/preset/generate.go`（只放 `//go:generate` 指令）
- Test: `internal/preset/gen/main_test.go`

**Interfaces:**
- Consumes: `preset.All()`、`preset.Preset.ResolveSafeArea()`、`preset.Canvas.Label()`（Task 1）
- Produces: `func Render(p preset.Preset, tpl Templates) (map[string][]byte, error)` — 返回「预设内相对路径 → 文件内容」

- [ ] **Step 1: 从现有文件切出模板**

```bash
mkdir -p internal/preset/gen/templates
cd assets/presets/vertical-3x4

# frontmatter（两文件逐字节相同，取其一即可）
awk '/^---$/{n++; next} n==1' frame.md > ../../../internal/preset/gen/templates/frontmatter.yaml
# 正文（含起始的 --- 之后全部）
awk '/^---$/{n++; next} n>=2' frame.md > ../../../internal/preset/gen/templates/frame.body.md
awk '/^---$/{n++; next} n>=2' "docs/清晰系统蓝图-视频风格说明书.md" > ../../../internal/preset/gen/templates/style-guide.body.md
cp PROMPT-PRODUCTION.md ../../../internal/preset/gen/templates/PROMPT-PRODUCTION.md
cd ../../..
```

- [ ] **Step 2: 在 frontmatter 模板里划定两个替换块**

编辑 `internal/preset/gen/templates/frontmatter.yaml`，把开头的

```yaml
canvas:
  width_px: 1080
  height_px: 1440
  fps: 30
  orientation: vertical
safe_area:
  structural: {left_px: 40, right_px: 40, top_px: 96, bottom_px: 60}
  main_content: {left_px: 88, right_px: 88, top_px: 120, bottom_px: 100}
  critical_text: {left_px: 88, right_px: 180, top_px: 120, bottom_px: 260}
  cover_title: {left_px: 88, right_px: 180, top_px: 260, bottom_px: 460}
  subtitles: {left_px: 88, right_px: 180, top_px: 990, bottom_px: 260}
```

整段替换为单行标记：

```yaml
# @@CANVAS_AND_SAFE_AREA@@
```

其余内容一个字符都不动。

- [ ] **Step 3: 在三份正文模板里替换画幅字面量**

对 `frame.body.md`、`style-guide.body.md`、`PROMPT-PRODUCTION.md` 三个文件做全局替换 `1080×1440` → `{{CANVAS}}`（全角乘号）：

```bash
cd internal/preset/gen/templates
for f in frame.body.md style-guide.body.md PROMPT-PRODUCTION.md; do
  python3 - "$f" <<'PY'
import sys, pathlib
p = pathlib.Path(sys.argv[1])
p.write_text(p.read_text(encoding="utf-8").replace("1080×1440", "{{CANVAS}}"), encoding="utf-8")
PY
done
grep -c '{{CANVAS}}' frame.body.md style-guide.body.md PROMPT-PRODUCTION.md
cd ../../../..
```

Expected 计数：`frame.body.md` 为 0，`style-guide.body.md` 为 3，`PROMPT-PRODUCTION.md` 为 4。若 `frame.body.md` 非 0 也无妨——说明正文里还有画幅描述，替换本身是正确的。

- [ ] **Step 4: 写生成器的失败测试**

创建 `internal/preset/gen/main_test.go`：

```go
package main

import (
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/preset"
)

func TestRenderCanvasAndSafeAreaBlock(t *testing.T) {
	p, _ := preset.ByID("vertical-9x16")
	block, err := canvasAndSafeArea(p)
	if err != nil {
		t.Fatalf("canvasAndSafeArea: %v", err)
	}
	want := strings.Join([]string{
		"canvas:",
		"  width_px: 1080",
		"  height_px: 1920",
		"  fps: 30",
		"  orientation: vertical",
		"safe_area:",
		"  structural: {left_px: 40, right_px: 40, top_px: 96, bottom_px: 60}",
		"  main_content: {left_px: 88, right_px: 88, top_px: 120, bottom_px: 100}",
		"  critical_text: {left_px: 88, right_px: 180, top_px: 120, bottom_px: 260}",
		"  cover_title: {left_px: 88, right_px: 180, top_px: 260, bottom_px: 940}",
		"  subtitles: {left_px: 88, right_px: 180, top_px: 1470, bottom_px: 260}",
	}, "\n")
	if block != want {
		t.Errorf("生成块不符：\n实际:\n%s\n期望:\n%s", block, want)
	}
}

// frame.md 与说明书的 frontmatter 必须逐字节相同，validate.Style 会做 DeepEqual。
func TestRenderKeepsFrontmatterIdenticalAcrossFiles(t *testing.T) {
	p, _ := preset.ByID("vertical-9x16")
	files, err := Render(p, loadTemplates())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	frame := frontmatterOf(t, files["frame.md"])
	guide := frontmatterOf(t, files["docs/清晰系统蓝图-视频风格说明书.md"])
	if frame != guide {
		t.Error("frame.md 与说明书的 frontmatter 不一致")
	}
}

func frontmatterOf(t *testing.T, body []byte) string {
	t.Helper()
	parts := strings.SplitN(string(body), "---", 3)
	if len(parts) != 3 {
		t.Fatalf("缺少 frontmatter")
	}
	return parts[1]
}

func TestRenderSubstitutesCanvasLabel(t *testing.T) {
	p, _ := preset.ByID("vertical-9x16")
	files, err := Render(p, loadTemplates())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := string(files["PROMPT-PRODUCTION.md"])
	if strings.Contains(body, "1080×1440") {
		t.Error("PROMPT-PRODUCTION.md 仍含 3:4 画幅")
	}
	if !strings.Contains(body, "1080×1920") {
		t.Error("PROMPT-PRODUCTION.md 未写入 9:16 画幅")
	}
	if strings.Contains(body, "{{CANVAS}}") {
		t.Error("占位符未被替换")
	}
}
```

- [ ] **Step 5: 跑测试确认失败**

Run: `go test ./internal/preset/gen/ -v`
Expected: 编译失败，`undefined: canvasAndSafeArea`

- [ ] **Step 6: 写 `internal/preset/gen/main.go`**

```go
// Command gen 由 go generate 驱动，把 internal/preset 的预设表渲染成
// assets/presets/<id>/ 下的完整项目模板。
//
// 生成器刻意不做 YAML 重序列化：frame.md 的 frontmatter 用了 flow 映射、
// 保留尾零的浮点写法和人工排定的键序，yaml.Marshal 一律还原不出来，会让
// 「3:4 产物与迁移前逐字节相同」这条红线失守。这里只整体替换 canvas 与
// safe_area 两个块，其余原样透传。
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/preset"
)

//go:embed templates
var templateFS embed.FS

const canvasMarker = "# @@CANVAS_AND_SAFE_AREA@@"

type Templates struct {
	Frontmatter string
	FrameBody   string
	GuideBody   string
	Production  string
}

func loadTemplates() Templates {
	read := func(name string) string {
		body, err := templateFS.ReadFile("templates/" + name)
		if err != nil {
			panic(err)
		}
		return string(body)
	}
	return Templates{
		Frontmatter: read("frontmatter.yaml"),
		FrameBody:   read("frame.body.md"),
		GuideBody:   read("style-guide.body.md"),
		Production:  read("PROMPT-PRODUCTION.md"),
	}
}

// canvasAndSafeArea 渲染 frontmatter 里唯一随画幅变化的两个块。
// 输出格式必须与手写的 frame.md 完全一致：safe_area 用 flow 映射单行表示。
func canvasAndSafeArea(p preset.Preset) (string, error) {
	boxes, err := p.ResolveSafeArea()
	if err != nil {
		return "", err
	}
	lines := []string{
		"canvas:",
		fmt.Sprintf("  width_px: %d", p.Canvas.WidthPx),
		fmt.Sprintf("  height_px: %d", p.Canvas.HeightPx),
		fmt.Sprintf("  fps: %d", p.Canvas.FPS),
		"  orientation: " + p.Canvas.Orientation,
		"safe_area:",
	}
	for _, item := range boxes {
		lines = append(lines, fmt.Sprintf("  %s: {left_px: %d, right_px: %d, top_px: %d, bottom_px: %d}",
			item.Name, item.Box.LeftPx, item.Box.RightPx, item.Box.TopPx, item.Box.BottomPx))
	}
	return strings.Join(lines, "\n"), nil
}

// Render 返回预设内相对路径到文件内容的映射。
func Render(p preset.Preset, tpl Templates) (map[string][]byte, error) {
	block, err := canvasAndSafeArea(p)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(tpl.Frontmatter, canvasMarker) {
		return nil, fmt.Errorf("frontmatter 模板缺少标记 %s", canvasMarker)
	}
	frontmatter := strings.Replace(tpl.Frontmatter, canvasMarker, block, 1)
	label := p.Canvas.Label()
	subst := func(s string) string { return strings.ReplaceAll(s, "{{CANVAS}}", label) }

	compose := func(body string) []byte {
		return []byte("---\n" + frontmatter + "---" + subst(body))
	}
	return map[string][]byte{
		"frame.md": compose(tpl.FrameBody),
		"docs/清晰系统蓝图-视频风格说明书.md": compose(tpl.GuideBody),
		"PROMPT-PRODUCTION.md":         []byte(subst(tpl.Production)),
	}, nil
}
```

`compose` 的 `"---\n" + frontmatter + "---"` 拼法要与原文件一致。原文件结构是 `---\n<frontmatter>---\n<body>`，而 `awk` 切出的 `frontmatter.yaml` 末尾带换行、`frame.body.md` 开头带换行——先按此实现，Step 8 的逐字节比对会立刻暴露任何偏差，届时按实际差异调整拼接。

- [ ] **Step 7: 写 `main()` 与 SVG/PNG 处理**

在 `main.go` 追加：

```go
func main() {
	root, err := repoRoot()
	if err != nil {
		fatal(err)
	}
	tpl := loadTemplates()
	for _, p := range preset.All() {
		dir := filepath.Join(root, "assets", "presets", p.ID)
		files, err := Render(p, tpl)
		if err != nil {
			fatal(fmt.Errorf("渲染预设 %s: %w", p.ID, err))
		}
		for name, body := range files {
			path := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				fatal(err)
			}
			if err := os.WriteFile(path, body, 0o644); err != nil {
				fatal(err)
			}
			fmt.Println("generated", filepath.ToSlash(filepath.Join("assets", "presets", p.ID, name)))
		}
		if err := renderExamples(dir, p); err != nil {
			fatal(fmt.Errorf("预设 %s 的示例图: %w", p.ID, err))
		}
	}
}

// renderExamples 把该预设手写的 4 张 SVG 转成 PNG 并拼 contact sheet。
// SVG 是手工绘制的排版基准，生成器不改动它们的内容。
func renderExamples(dir string, p preset.Preset) error {
	magick, err := exec.LookPath("magick")
	if err != nil {
		return fmt.Errorf("需要 ImageMagick 的 `magick` 命令")
	}
	examples := filepath.Join(dir, "assets", "style-guide", "examples")
	names := []string{"proposition", "comparison", "process", "capability_deck"}
	pngPaths := make([]string, 0, len(names))
	for _, name := range names {
		svgPath := filepath.Join(examples, name+".svg")
		if _, err := os.Stat(svgPath); err != nil {
			return fmt.Errorf("缺少手写示例图 %s: %w", name+".svg", err)
		}
		pngPath := filepath.Join(examples, name+".png")
		if out, err := exec.Command(magick, "-background", "none", svgPath, pngPath).CombinedOutput(); err != nil {
			return fmt.Errorf("渲染 %s 失败: %w: %s", name, err, strings.TrimSpace(string(out)))
		}
		pngPaths = append(pngPaths, pngPath)
	}
	thumbWidth := 405
	thumbHeight := thumbWidth * p.Canvas.HeightPx / p.Canvas.WidthPx
	geometry := fmt.Sprintf("%dx%d", thumbWidth, thumbHeight)
	args := append([]string{"montage"}, pngPaths...)
	args = append(args, "-thumbnail", geometry, "-tile", "4x1", "-geometry", geometry+"+10+10",
		"-background", "#D5DEEB", filepath.Join(examples, "contact-sheet.png"))
	if out, err := exec.Command(magick, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("生成 contact sheet 失败: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// repoRoot 从当前工作目录向上找到含 go.mod 的目录。
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("找不到仓库根目录（向上未发现 go.mod）")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}

var _ = fs.ValidPath // 占位，避免误删 io/fs import
```

删掉最后那行占位并同时删掉 `io/fs` import——它没被用到。

创建 `internal/preset/generate.go`：

```go
package preset

//go:generate go run ./gen
```

- [ ] **Step 8: 跑生成器并逐字节比对 3:4**

```bash
git stash list >/dev/null  # 确认工作区干净
go generate ./internal/preset/
git diff --stat assets/presets/vertical-3x4/
```

Expected: **无输出**（3:4 的三个文件逐字节未变）

若有差异，看 `git diff` 定位到是拼接换行还是块格式问题，改 `Render` 或模板直到 diff 为空。**这一步不通过不得进入下一任务。**

- [ ] **Step 9: 跑测试**

Run: `go test ./internal/preset/... -v && go vet ./...`
Expected: 全部 PASS。`TestRenderKeepsFrontmatterIdenticalAcrossFiles` 与 `TestRenderSubstitutesCanvasLabel` 会因为 9:16 预设的 SVG 还不存在而**只测 Render 不测 renderExamples**，属预期

- [ ] **Step 10: 提交**

```bash
git add -A
git commit -m "feat: 新增预设生成器，3:4 产物与手写版本逐字节一致"
```

---

### Task 4: 9:16 示例图与预设产物

**Files:**
- Create: `assets/presets/vertical-9x16/assets/style-guide/examples/proposition.svg`
- Create: `assets/presets/vertical-9x16/assets/style-guide/examples/comparison.svg`
- Create: `assets/presets/vertical-9x16/assets/style-guide/examples/process.svg`
- Create: `assets/presets/vertical-9x16/assets/style-guide/examples/capability_deck.svg`
- Generated: `assets/presets/vertical-9x16/**`（其余由 Task 3 的生成器产出）

**Interfaces:**
- Consumes: Task 3 的 `go generate`
- Produces: 完整可用的 `vertical-9x16` 素材树

- [ ] **Step 1: 以 3:4 版本为参照手绘 4 张 9:16 SVG**

对每张图：把 `width` / `height` / `viewBox` 改为 `1080` / `1920` / `0 0 1080 1920`；背景 `<rect>` 高度改 1920；**重新排布内容以填满多出的 480px，而不是留空**。

各图的锚定语义（对齐 `internal/preset/table.go` 的 anchor 表）：

| 元素 | 处理 |
|---|---|
| 顶部角标、类型标签、标题分隔线 | 贴顶不动（`y` 不变） |
| 底部角标（3:4 里 `y=1340/1380`） | 平移 +480 → `y=1820/1860` |
| 底部装饰椭圆（`cy=1450`） | 平移 +480 → `cy=1930` |
| 主内容区（3:4 的 `y≈294..1134`） | 贴顶起始不变，纵向节奏放宽以占满至 `y≈1660` |

`proposition.svg` 的具体改动（其余三张同理，逐张按上表处理）：

```
width="1080" height="1440" viewBox="0 0 1080 1440"
  → width="1080" height="1920" viewBox="0 0 1080 1920"

<rect width="1080" height="1440" fill="#F5F7FB"/>
<rect width="1080" height="1440" fill="url(#grid)"/>
  → 两处 height 改 1920

<ellipse cx="540" cy="1450" rx="620" ry="420" .../>
  → cy="1930"

<path d="M40 140v-44h44 M996 96h44v44 M40 1340v40h44 M996 1380h44v-40"/>
  → <path d="M40 140v-44h44 M996 96h44v44 M40 1820v40h44 M996 1860h44v-40"/>

主标题三行 y="480" / "586" / "692"
  → 行距从 106 放宽到 128：y="520" / "648" / "776"
下划线 rect y="750" → y="846"
说明文字 text y="820" → y="928"
底部卡片 <g transform="translate(88 980)"> → translate(88 1320)，卡片高度 154 → 190
```

改完确认：卡片底边 `1320+190=1510`，距画布底 410px，落在 `subtitles` 安全区（top 1470）之上，不与字幕带冲突。

- [ ] **Step 2: 校验 SVG 尺寸**

```bash
for f in assets/presets/vertical-9x16/assets/style-guide/examples/*.svg; do
  head -1 "$f" | grep -q 'width="1080" height="1920"' || echo "尺寸错误: $f"
done
```
Expected: 无输出

- [ ] **Step 3: 跑生成器**

Run: `go generate ./internal/preset/`
Expected: 打印 `generated assets/presets/vertical-9x16/frame.md` 等，并产出 4 张 PNG + contact-sheet

- [ ] **Step 4: 校验 9:16 产物**

```bash
grep -A4 '^canvas:' assets/presets/vertical-9x16/frame.md
grep '^  subtitles:' assets/presets/vertical-9x16/frame.md
for f in assets/presets/vertical-9x16/assets/style-guide/examples/*.png; do
  magick identify -format '%f %wx%h\n' "$f"
done
git diff --stat assets/presets/vertical-3x4/
```

Expected:
- canvas 为 `1080 / 1920 / 30 / vertical`
- `subtitles: {left_px: 88, right_px: 180, top_px: 1470, bottom_px: 260}`
- 4 张示例 PNG 均为 `1080x1920`
- 3:4 目录**无 diff**

- [ ] **Step 5: 提交**

```bash
git add -A
git commit -m "feat: 新增 vertical-9x16 预设的手绘示例图与生成产物"
```

---

### Task 5: `--canvas` 参数

**Files:**
- Create: `internal/cli/canvas.go`
- Modify: `internal/cli/cli.go`（init 命令）
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `preset.ByID`、`preset.IDs`、`preset.Default`、`assets.Shared`、`assets.Preset`
- Produces: `func resolveCanvas(flag string, stdin *os.File, stdout io.Writer) (preset.Preset, error)`（Task 6 补 TTY 分支）

- [ ] **Step 1: 写失败测试**

在 `internal/cli/cli_test.go` 追加：

```go
func TestInitRejectsUnknownCanvas(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Execute([]string{"init", t.TempDir(), "--canvas", "vertical-4x5", "--skip-hyperframes"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("未知画幅应报错")
	}
	if !strings.Contains(err.Error(), "vertical-9x16") {
		t.Errorf("错误信息应列出可选画幅，实际：%v", err)
	}
}

func TestInitWritesChosenCanvas(t *testing.T) {
	target := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := Execute([]string{"init", target, "--canvas", "vertical-9x16", "--skip-hyperframes"}, &stdout, &stderr); err != nil {
		t.Fatalf("init: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(target, "frame.md"))
	if err != nil {
		t.Fatalf("读取 frame.md: %v", err)
	}
	if !strings.Contains(string(body), "height_px: 1920") {
		t.Error("frame.md 未写入 9:16 画幅")
	}
	if _, err := os.Stat(filepath.Join(target, "assets", "fonts", "noto-sans-sc-400.woff2")); err != nil {
		t.Errorf("共享字体未拷入：%v", err)
	}
}
```

按 `cli_test.go` 现有的 `Execute` 调用签名调整——若签名不同，照现有测试的写法改。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/cli/ -run TestInit -v`
Expected: FAIL，`unknown flag: --canvas`

- [ ] **Step 3: 写 `internal/cli/canvas.go`**

```go
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/preset"
)

// canvasOptions 供错误信息与选择框共用的可读清单。
func canvasOptions() string {
	lines := make([]string, 0, len(preset.All()))
	for _, p := range preset.All() {
		lines = append(lines, fmt.Sprintf("  %s  %s", p.ID, p.Label))
	}
	return strings.Join(lines, "\n")
}

// resolveCanvas 决定本次 init 使用哪套画幅预设。
//
// 未显式传入且不在终端里时直接报错，不静默取默认值：画幅一旦选错，
// 整个项目的排版基准、安全区和成片规格都是错的，静默降级的代价远高于报错。
func resolveCanvas(flag string, stdin *os.File, stdout io.Writer) (preset.Preset, error) {
	if flag != "" {
		p, ok := preset.ByID(flag)
		if !ok {
			return preset.Preset{}, fmt.Errorf("未知画幅 %q，可选：\n%s", flag, canvasOptions())
		}
		return p, nil
	}
	if !isTerminal(stdin) {
		return preset.Preset{}, fmt.Errorf("非交互环境必须显式传入 --canvas，可选：\n%s", canvasOptions())
	}
	return pickCanvas(stdout)
}
```

暂时给 `isTerminal` 与 `pickCanvas` 写占位实现（Task 6 替换）：

```go
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// pickCanvas 的交互实现在 Task 6 接入 bubbletea。
func pickCanvas(io.Writer) (preset.Preset, error) {
	return preset.Preset{}, fmt.Errorf("交互选择框尚未接入，请显式传入 --canvas，可选：\n%s", canvasOptions())
}
```

- [ ] **Step 4: 接进 init 命令**

在 `internal/cli/cli.go` 的 `var skipHyperframes bool` 旁加 `var canvasID string`，`RunE` 里把 Task 2 写的 `preset.Default().ID` 换成解析结果：

```go
chosenPreset, err := resolveCanvas(canvasID, os.Stdin, stdout)
if err != nil {
    return err
}
shared, err := assets.Shared()
if err != nil {
    return err
}
chosen, err := assets.Preset(chosenPreset.ID)
if err != nil {
    return err
}
result, err := project.Initialize(target, shared, chosen)
if err != nil {
    return err
}
fmt.Fprintf(stdout, "项目已初始化：%s（画幅 %s，新增 %d，未变 %d）\n", target, chosenPreset.Label, result.Created, result.Unchanged)
```

注册 flag：

```go
initCmd.Flags().StringVar(&canvasID, "canvas", "", "画幅预设：vertical-3x4 | vertical-9x16")
```

- [ ] **Step 5: 跑测试**

Run: `go test ./internal/cli/ -v && go vet ./...`
Expected: PASS。`go test` 下 stdin 不是字符设备，`TestInitWritesChosenCanvas` 走显式 flag 分支

- [ ] **Step 6: 手工验证非交互失败关闭**

```bash
go build ./cmd/am
./am init /tmp/am-nc --skip-hyperframes < /dev/null; echo "退出码 $?"
```
Expected: 报错「非交互环境必须显式传入 --canvas」并列出两个 id，退出码非 0

- [ ] **Step 7: 提交**

```bash
git add -A
git commit -m "feat: am init 支持 --canvas 选择画幅预设"
```

---

### Task 6: 交互选择框

**Files:**
- Modify: `internal/cli/canvas.go`
- Modify: `go.mod`, `go.sum`
- Test: `internal/cli/canvas_test.go`

**Interfaces:**
- Consumes: `preset.All()`、`preset.Default()`
- Produces: `pickCanvas` 的 bubbletea 实现

- [ ] **Step 1: 装依赖**

```bash
go get github.com/charmbracelet/bubbletea@latest
go mod tidy
```

- [ ] **Step 2: 写模型的失败测试**

创建 `internal/cli/canvas_test.go`：

```go
package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPickerStartsOnDefaultPreset(t *testing.T) {
	m := newCanvasPicker()
	if m.cursor != 0 {
		t.Errorf("初始光标 = %d，期望 0", m.cursor)
	}
	if m.choices[0].ID != "vertical-3x4" {
		t.Errorf("首项 = %s，期望 vertical-3x4", m.choices[0].ID)
	}
}

func TestPickerMovesAndSelects(t *testing.T) {
	m := newCanvasPicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(canvasPicker)
	if m.cursor != 1 {
		t.Fatalf("下移后光标 = %d，期望 1", m.cursor)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(canvasPicker)
	if !m.confirmed || m.Selected().ID != "vertical-9x16" {
		t.Errorf("确认后 = %v %s", m.confirmed, m.Selected().ID)
	}
}

func TestPickerClampsAtBoundaries(t *testing.T) {
	m := newCanvasPicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if next.(canvasPicker).cursor != 0 {
		t.Error("首项上移应停在 0")
	}
}

// Ctrl+C / Esc 必须走取消路径，不能当成选中默认值。
func TestPickerAbort(t *testing.T) {
	m := newCanvasPicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !next.(canvasPicker).aborted {
		t.Error("Ctrl+C 应标记为取消")
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/cli/ -run TestPicker -v`
Expected: 编译失败，`undefined: newCanvasPicker`

- [ ] **Step 4: 实现 picker**

在 `internal/cli/canvas.go` 追加，并删掉 Step 3 的 `pickCanvas` 占位：

```go
type canvasPicker struct {
	choices   []preset.Preset
	cursor    int
	confirmed bool
	aborted   bool
}

func newCanvasPicker() canvasPicker {
	return canvasPicker{choices: preset.All()}
}

func (m canvasPicker) Selected() preset.Preset { return m.choices[m.cursor] }

func (m canvasPicker) Init() tea.Cmd { return nil }

func (m canvasPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(m.choices)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		m.confirmed = true
		return m, tea.Quit
	case tea.KeyCtrlC, tea.KeyEsc:
		m.aborted = true
		return m, tea.Quit
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "q":
			m.aborted = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m canvasPicker) View() string {
	var b strings.Builder
	b.WriteString("选择画幅\n\n")
	for i, p := range m.choices {
		marker := "  "
		if i == m.cursor {
			marker = "❯ "
		}
		b.WriteString(marker + p.Label + "\n")
	}
	b.WriteString("\n↑/↓ 移动　Enter 确认　Esc 取消\n")
	return b.String()
}

func pickCanvas(stdout io.Writer) (preset.Preset, error) {
	program := tea.NewProgram(newCanvasPicker(), tea.WithOutput(stdout))
	final, err := program.Run()
	if err != nil {
		return preset.Preset{}, fmt.Errorf("画幅选择框启动失败：%w", err)
	}
	model := final.(canvasPicker)
	if model.aborted || !model.confirmed {
		return preset.Preset{}, fmt.Errorf("已取消初始化")
	}
	return model.Selected(), nil
}
```

import 补 `tea "github.com/charmbracelet/bubbletea"`。

- [ ] **Step 5: 跑测试**

Run: `go test ./internal/cli/ -v && go vet ./...`
Expected: 全部 PASS

- [ ] **Step 6: 手工验证交互路径**

```bash
go build ./cmd/am
rm -rf /tmp/am-tui && ./am init /tmp/am-tui --skip-hyperframes
```
Expected: 出现方向键选择框，选 9:16 后 `/tmp/am-tui/frame.md` 含 `height_px: 1920`

- [ ] **Step 7: 提交**

```bash
git add -A
git commit -m "feat: am init 无参数时进入画幅选择框"
```

---

### Task 7: `validate` 改为向预设反查

**Files:**
- Modify: `internal/validate/validate.go`
- Test: `internal/validate/validate_test.go`

**Interfaces:**
- Consumes: `preset.ByCanvas`、`preset.Preset.ResolveSafeArea`、`preset.IDs`
- Produces: 无新导出

- [ ] **Step 1: 写失败测试**

在 `internal/validate/validate_test.go` 追加：

```go
func TestStyleAcceptsNineBySixteenProject(t *testing.T) {
	root := newProjectFromPreset(t, "vertical-9x16")
	if err := Style(root); err != nil {
		t.Fatalf("9:16 项目应通过校验：%v", err)
	}
}

// 手改安全区在今天查不出来。改完必须能查出。
func TestStyleRejectsTamperedSafeArea(t *testing.T) {
	root := newProjectFromPreset(t, "vertical-3x4")
	tamper(t, root, "top_px: 990", "top_px: 1000")
	err := Style(root)
	if err == nil {
		t.Fatal("被篡改的 safe_area 应被拒绝")
	}
	if !strings.Contains(err.Error(), "subtitles") {
		t.Errorf("错误应指出字段名，实际：%v", err)
	}
}

func TestStyleRejectsUnknownCanvas(t *testing.T) {
	root := newProjectFromPreset(t, "vertical-3x4")
	tamper(t, root, "height_px: 1440", "height_px: 1600")
	err := Style(root)
	if err == nil {
		t.Fatal("未内置的画幅应被拒绝")
	}
	if !strings.Contains(err.Error(), "vertical-9x16") {
		t.Errorf("错误应列出可选画幅，实际：%v", err)
	}
}

// newProjectFromPreset 用内置素材铺一个临时项目。
func newProjectFromPreset(t *testing.T, id string) string {
	t.Helper()
	root := t.TempDir()
	src, err := assets.Preset(id)
	if err != nil {
		t.Fatalf("取预设素材：%v", err)
	}
	shared, err := assets.Shared()
	if err != nil {
		t.Fatalf("取共享素材：%v", err)
	}
	if _, err := project.Initialize(root, shared, src); err != nil {
		t.Fatalf("铺项目：%v", err)
	}
	return root
}

// tamper 同时改 frame.md 与说明书，避免先被 DeepEqual 拦下而测不到目标断言。
func tamper(t *testing.T, root, from, to string) {
	t.Helper()
	for _, name := range []string{"frame.md", filepath.Join("docs", "清晰系统蓝图-视频风格说明书.md")} {
		path := filepath.Join(root, name)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读 %s: %v", name, err)
		}
		replaced := strings.Replace(string(body), from, to, 1)
		if replaced == string(body) {
			t.Fatalf("%s 里找不到 %q", name, from)
		}
		if err := os.WriteFile(path, []byte(replaced), 0o644); err != nil {
			t.Fatalf("写 %s: %v", name, err)
		}
	}
}
```

`internal/validate` import `project` 会形成依赖环吗？不会——`internal/project` 不 import `internal/validate`。若 `go vet` 报环，改用 `t.TempDir()` + 手工 `fs.WalkDir` 拷贝，不要调 `project.Initialize`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/validate/ -run TestStyle -v`
Expected: `TestStyleAcceptsNineBySixteenProject` FAIL，报 `canvas 必须为 1080x1440、30fps、vertical`

- [ ] **Step 3: 改 canvas 校验**

`internal/validate/validate.go:206-212` 的

```go
	canvas, err := requireMap(tokens["canvas"], "canvas")
	if err != nil {
		return err
	}
	if canvas["width_px"] != 1080 || canvas["height_px"] != 1440 || canvas["fps"] != 30 || canvas["orientation"] != "vertical" {
		return fmt.Errorf("canvas 必须为 1080x1440、30fps、vertical")
	}
```

替换为

```go
	canvas, err := requireMap(tokens["canvas"], "canvas")
	if err != nil {
		return err
	}
	width, widthOK := canvas["width_px"].(int)
	height, heightOK := canvas["height_px"].(int)
	fps, fpsOK := canvas["fps"].(int)
	orientation, orientationOK := canvas["orientation"].(string)
	if !widthOK || !heightOK || !fpsOK || !orientationOK {
		return fmt.Errorf("canvas 必须包含整数 width_px、height_px、fps 与字符串 orientation")
	}
	active, ok := preset.ByCanvas(width, height, fps, orientation)
	if !ok {
		return fmt.Errorf("canvas %dx%d、%dfps、%s 不是内置画幅，可选：%s",
			width, height, fps, orientation, strings.Join(preset.IDs(), " "))
	}
```

- [ ] **Step 4: 把 safe_area 校验升级为 anchor 断言**

`validate.go:213-228` 的循环替换为

```go
	safeArea, err := requireMap(tokens["safe_area"], "safe_area")
	if err != nil {
		return err
	}
	expected, err := active.ResolveSafeArea()
	if err != nil {
		return err
	}
	if len(safeArea) != len(expected) {
		return fmt.Errorf("safe_area 必须恰好包含 %d 个安全区", len(expected))
	}
	for _, item := range expected {
		box, err := requireMap(safeArea[item.Name], "safe_area."+item.Name)
		if err != nil {
			return err
		}
		if err := requireKeys(box, []string{"left_px", "right_px", "top_px", "bottom_px"}, "safe_area."+item.Name); err != nil {
			return err
		}
		for field, want := range map[string]int{
			"left_px": item.Box.LeftPx, "right_px": item.Box.RightPx,
			"top_px": item.Box.TopPx, "bottom_px": item.Box.BottomPx,
		} {
			got, ok := box[field].(int)
			if !ok {
				return fmt.Errorf("safe_area.%s.%s 必须是整数", item.Name, field)
			}
			if got != want {
				return fmt.Errorf("safe_area.%s.%s = %d，按 %s 画幅应为 %d", item.Name, field, got, active.ID, want)
			}
		}
	}
```

`validateNumericMap` 的调用在这里删掉——逐值相等是更强的断言，已覆盖非负检查。

`validateStyleSchema` 需要把解析出的 `active` 传给调用方，改签名为 `func validateStyleSchema(tokens map[string]any) (preset.Preset, error)`，两处调用点（`Style` 里对 frame 与 guide 各一次）相应接收并忽略 guide 那次的返回值。

- [ ] **Step 5: 示例 PNG 尺寸按 canvas 断言**

`validate.go:144-187` 的 `Style` 里，`validateStyleSchema(frame)` 改为接收 `active`，然后把 `:182` 的

```go
		if width != 1080 || height != 1440 {
			return fmt.Errorf("风格示例必须为 1080x1440：%s", path)
		}
```

替换为

```go
		if width != active.Canvas.WidthPx || height != active.Canvas.HeightPx {
			return fmt.Errorf("风格示例必须为 %dx%d，实际 %dx%d：%s",
				active.Canvas.WidthPx, active.Canvas.HeightPx, width, height, path)
		}
```

- [ ] **Step 6: `RegenerateExamples` 按项目画幅取模板**

`validate.go:497` 起，在读到 `frame` 之后加画幅反查，并把 Task 2 里写死的 `preset.Default().ID` 换掉：

```go
	canvas, ok := frame["canvas"].(map[string]any)
	if !ok {
		return fmt.Errorf("frame.md 的 canvas 必须是对象")
	}
	width, _ := canvas["width_px"].(int)
	height, _ := canvas["height_px"].(int)
	fps, _ := canvas["fps"].(int)
	orientation, _ := canvas["orientation"].(string)
	active, ok := preset.ByCanvas(width, height, fps, orientation)
	if !ok {
		return fmt.Errorf("frame.md 的画幅不是内置画幅，无法定位示例模板")
	}
```

再把循环里的

```go
		sourcePath := "assets/presets/" + preset.Default().ID + "/assets/style-guide/examples/" + name + ".svg"
```

改为

```go
		sourcePath := "assets/presets/" + active.ID + "/assets/style-guide/examples/" + name + ".svg"
```

contact sheet 的缩略图几何也要跟画幅走，把

```go
	args = append(args, "-thumbnail", "405x540", "-tile", "4x1", "-geometry", "405x540+10+10", ...)
```

改为

```go
	thumb := fmt.Sprintf("%dx%d", 405, 405*active.Canvas.HeightPx/active.Canvas.WidthPx)
	args = append(args, "-thumbnail", thumb, "-tile", "4x1", "-geometry", thumb+"+10+10", ...)
```

- [ ] **Step 7: 修既有测试**

`validate_test.go:78` 与 `:91` 用 `width_px: 1080 → 999` 制造非法值。现在 999 会走「不是内置画幅」分支，报错信息变了。把这两处的断言从匹配旧文案改为匹配 `不是内置画幅`，或改用 `height_px: 1440 → 1600`。按实际报错调整。

- [ ] **Step 8: 跑测试**

Run: `go test ./... -v && go vet ./...`
Expected: 全部 PASS

- [ ] **Step 9: 端到端校验 9:16 项目**

```bash
go build ./cmd/am
rm -rf /tmp/am-916 && ./am init /tmp/am-916 --canvas vertical-9x16 --skip-hyperframes
./am validate style --project-root /tmp/am-916
```
Expected: `风格规范校验通过`

- [ ] **Step 10: 提交**

```bash
git add -A
git commit -m "feat: validate 改为向预设表反查画幅并逐值断言安全区"
```

---

### Task 8: 修复渲染提示词的画幅硬编码

**Files:**
- Modify: `internal/scene/scene.go:130-180`
- Test: `internal/scene/scene_test.go`

**Interfaces:**
- Consumes: 无（直接解析 `style_guide` 文件的 frontmatter，不引入 preset 依赖——镜头目录可能脱离项目独立运行）
- Produces: 无新导出

- [ ] **Step 1: 写失败测试**

在 `internal/scene/scene_test.go` 追加：

```go
func TestBuildPromptUsesCanvasFromStyleGuide(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "frame.md"), "---\ncanvas:\n  width_px: 1080\n  height_px: 1920\n  fps: 30\n  orientation: vertical\n---\n正文\n")
	s := Scene{Directory: dir, ID: "scene-001", DurationSeconds: 3, Output: "out.mp4", Transcript: "t.srt", Text: "文本", StyleGuide: "frame.md"}
	prompt, err := BuildPrompt(s, "")
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(prompt, "1080x1920") {
		t.Error("提示词未使用 style_guide 声明的画幅")
	}
	if strings.Contains(prompt, "1080x1440") {
		t.Error("提示词仍含硬编码的 1080x1440")
	}
}

// 无 style_guide 时保持今天的行为，存量镜头目录不受影响。
func TestBuildPromptFallsBackWithoutStyleGuide(t *testing.T) {
	dir := t.TempDir()
	s := Scene{Directory: dir, ID: "scene-001", DurationSeconds: 3, Output: "out.mp4", Transcript: "t.srt", Text: "文本"}
	prompt, err := BuildPrompt(s, "")
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(prompt, "1080x1440") {
		t.Error("缺 style_guide 时应回退到 1080x1440")
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写 %s: %v", path, err)
	}
}
```

若 `scene_test.go` 已有同名 `writeFile` 辅助函数，复用现有的，不要重复定义。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/scene/ -run TestBuildPrompt -v`
Expected: `TestBuildPromptUsesCanvasFromStyleGuide` FAIL，提示词含 `1080x1440`

- [ ] **Step 3: 加 canvas 解析辅助函数**

在 `internal/scene/scene.go` 追加：

```go
// canvasSpec 从镜头目录里的视觉规范文件读出画布规格。
//
// 这里刻意不依赖 internal/preset：镜头目录可以脱离项目独立运行
// （am scene run <目录>），能拿到的只有目录内的文件。
func canvasSpec(directory, styleGuide string) (string, error) {
	const fallback = "1080x1440、30fps"
	if styleGuide == "" {
		return fallback, nil
	}
	path, err := contained(directory, styleGuide, "style_guide")
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取视觉规范失败：%w", err)
	}
	parts := strings.SplitN(string(body), "---", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("视觉规范 %s 缺少 YAML frontmatter", styleGuide)
	}
	var parsed struct {
		Canvas struct {
			WidthPx  int `yaml:"width_px"`
			HeightPx int `yaml:"height_px"`
			FPS      int `yaml:"fps"`
		} `yaml:"canvas"`
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &parsed); err != nil {
		return "", fmt.Errorf("视觉规范 %s 的 frontmatter 解析失败：%w", styleGuide, err)
	}
	c := parsed.Canvas
	if c.WidthPx <= 0 || c.HeightPx <= 0 || c.FPS <= 0 {
		return "", fmt.Errorf("视觉规范 %s 的 canvas 缺少 width_px/height_px/fps", styleGuide)
	}
	return fmt.Sprintf("%dx%d、%dfps", c.WidthPx, c.HeightPx, c.FPS), nil
}
```

import 补 `"gopkg.in/yaml.v3"`。

- [ ] **Step 4: 改 `BuildPrompt`**

在 `BuildPrompt` 里 `style := ""` 之前插入：

```go
	canvas, err := canvasSpec(s.Directory, s.StyleGuide)
	if err != nil {
		return "", err
	}
```

把提示词格式串里的

```
- 制作 1080x1440、30fps、静音、无音轨的 HyperFrames 动画。
```

改为

```
- 制作 %s、静音、无音轨的 HyperFrames 动画。
```

并在 `fmt.Sprintf` 的参数列表最前面插入 `canvas`（它在 `s.ID` 之前）。核对参数顺序与格式串占位符一一对应。

- [ ] **Step 5: 跑测试**

Run: `go test ./internal/scene/ -v && go test ./... && go vet ./...`
Expected: 全部 PASS

- [ ] **Step 6: 端到端验证**

```bash
rm -rf /tmp/am-916 && ./am init /tmp/am-916 --canvas vertical-9x16 --skip-hyperframes
mkdir -p /tmp/am-916/scenes/scene-001
cp /tmp/am-916/frame.md /tmp/am-916/scenes/scene-001/
cat > /tmp/am-916/scenes/scene-001/scene.json <<'JSON'
{"id":"scene-001","duration_seconds":3.0,"output":"scene-001.mp4","transcript":"transcription.srt","text":"测试文本","style_guide":"frame.md"}
JSON
touch /tmp/am-916/scenes/scene-001/transcription.srt
```

用一段临时 Go 程序或既有测试打印提示词，确认含 `1080x1920`、不含 `1080x1440`。

- [ ] **Step 7: 提交**

```bash
git add -A
git commit -m "fix: 渲染提示词的画幅改从 style_guide 读取，不再硬编码"
```

---

### Task 9: CI 幂等校验与文档

**Files:**
- Modify: `.github/workflows/*.yml`
- Modify: `README.md`, `README.en.md`
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: 前八个任务的全部成果
- Produces: 无

- [ ] **Step 1: 看现有 workflow**

Run: `ls .github/workflows/ && cat .github/workflows/*.yml`

- [ ] **Step 2: 加生成物幂等 job**

在 workflow 里追加（按现有文件的缩进与 runner 版本对齐）：

```yaml
  generated-assets:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      # 生成器要用 magick 把手写 SVG 转 PNG；日常 build/test 不需要
      - run: sudo apt-get update && sudo apt-get install -y imagemagick
      - run: go generate ./internal/preset/
      - name: 生成物必须与仓库一致
        run: git diff --exit-code
```

- [ ] **Step 3: 更新 README 的创建项目一节**

`README.md` 的「## 创建项目」把

````markdown
```bash
am init my-video
cd my-video
```
````

改为

````markdown
```bash
am init my-video          # 交互选择画幅
am init my-video --canvas vertical-9x16   # 或显式指定
cd my-video
```

画幅在初始化时一次性选定，写入项目的 `frame.md` 后不再更改。当前内置：

| 预设 | 画幅 | 用途 |
|---|---|---|
| `vertical-3x4` | 1080×1440，30fps | 默认，知识与科技类竖屏解说 |
| `vertical-9x16` | 1080×1920，30fps | 抖音等全屏竖屏平台 |

非交互环境（CI、管道）必须显式传入 `--canvas`，不会静默取默认值。
````

`README.en.md` 同步等价内容。

- [ ] **Step 4: 在 AGENTS.md 记录生成物约定**

追加一节：

```markdown
## 画幅预设

`assets/presets/<id>/` 全部由 `go generate ./internal/preset/` 产出，不要手改。
真相源是 `internal/preset/table.go` 的预设表与 `internal/preset/gen/templates/`
下的模板。安全区用 anchor 声明，构建期推导成绝对像素写进 `frame.md`。

改完预设表后必须重跑 `go generate` 并提交生成物，CI 的 `generated-assets`
job 会校验二者一致。生成器需要 ImageMagick。
```

- [ ] **Step 5: 全量验收**

```bash
go generate ./internal/preset/ && git diff --exit-code
go build ./cmd/am
go test ./... && go test -race ./... && go vet ./...
rm -rf /tmp/am-a /tmp/am-b
./am init /tmp/am-a --canvas vertical-3x4 --skip-hyperframes && ./am validate style --project-root /tmp/am-a
./am init /tmp/am-b --canvas vertical-9x16 --skip-hyperframes && ./am validate style --project-root /tmp/am-b
./am init /tmp/am-c --skip-hyperframes < /dev/null; echo "非交互退出码 $?（应非 0）"
```
Expected: 全绿，两个项目都通过风格校验，非交互无 flag 报错

- [ ] **Step 6: 提交**

```bash
git add -A
git commit -m "docs: 补画幅预设文档与生成物幂等 CI 校验"
```

---

## Self-Review

**Spec 覆盖**

| Spec 章节 | 落地任务 |
|---|---|
| §3.1 anchor 推导 | Task 1 |
| §3.2 宽度不变的简化 | Task 1（两套预设共用 `baseSafeArea`） |
| §3.3 go generate 预生成 | Task 3、Task 9 Step 2 |
| §3.4 示例图手写 | Task 4 |
| §4.1 `internal/preset` 接口 | Task 1 |
| §4.2 素材重组 | Task 2 |
| §4.3 生成器 | Task 3 |
| §4.4 init 交互 | Task 5、Task 6 |
| §4.5 validate 改造 | Task 7 |
| §4.6 scene 一致性 | Task 8 |
| §5 错误处理表 | Task 5 Step 3、Task 7 Step 3-5、Task 3 Step 7 |
| §6 测试矩阵 | 各任务 Step 1 |
| §7 验收标准 | Task 3 Step 8（#1）、Task 7 Step 9（#2）、Task 8 Step 6（#3）、Task 5 Step 6 + Task 6 Step 6（#4）、Task 9 Step 5（#5） |

**类型一致性**：`ResolvedBox` 字段名（`LeftPx`/`RightPx`/`TopPx`/`BottomPx`）在 Task 1 定义，Task 3 的 `canvasAndSafeArea` 与 Task 7 的安全区断言均按此使用。`NamedBox{Name, Box}` 同理。`preset.ByCanvas(width, height, fps int, orientation string)` 的参数顺序在 Task 1、Task 7 Step 3、Step 6 三处一致。`Initialize(target string, sources ...fs.FS)` 在 Task 2 定义，Task 5 与 Task 7 测试按此调用。

**已知风险**：Task 3 Step 8 的逐字节比对是全局最高风险点。`awk` 切分 frontmatter 与正文时的换行归属可能与原文件不完全对应，第一次跑大概率有 diff。处理办法写在该步里——按 `git diff` 的实际差异调 `compose` 的拼接，不要改 `assets/presets/vertical-3x4/` 下的产物去迁就代码。
