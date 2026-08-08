// Command gen 由 go generate 驱动，把 internal/preset 的预设表渲染成
// assets/presets/<id>/ 下的完整项目模板。
//
// 生成器刻意不做 YAML 重序列化：frame.md 的 frontmatter 用了 flow 映射、
// 保留尾零的浮点写法和人工排定的键序，yaml.Marshal 一律还原不出来，会让
// 「3:4 产物与手写版本逐字节相同」这条红线失守。这里只整体替换 canvas 与
// safe_area 两个块，其余原样透传。
package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/preset"
	"github.com/chouheiwa/articale-to-motion/internal/styleimage"
)

//go:embed templates
var templateFS embed.FS

const canvasMarker = "# @@CANVAS_AND_SAFE_AREA@@"

// 示例图在 contact sheet 里的缩略宽度，高度按画幅比例推算。
const thumbnailWidthPx = 405

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

	// 原文件结构是 ---\n<frontmatter>---<body>，body 自带前导换行。
	compose := func(body string) []byte {
		return []byte("---\n" + frontmatter + "---" + subst(body))
	}
	return map[string][]byte{
		"frame.md": compose(tpl.FrameBody),
		"docs/清晰系统蓝图-视频风格说明书.md": compose(tpl.GuideBody),
		"PROMPT-PRODUCTION.md":   []byte(subst(tpl.Production)),
	}, nil
}

// withExamples 控制是否重新渲染 PNG。
//
// 默认关闭是刻意的：ImageMagick 不同版本对同一份 SVG 的输出字节不同，若把
// PNG 纳入 go generate，CI 的「重跑后 git diff 为空」这道门就会随 runner 上
// 的 ImageMagick 版本随机失败，失去意义。文本产物是确定性的，才适合当门禁。
//
// SVG 改动后手动跑一次 `go run ./gen -examples` 更新 PNG 并提交。
var withExamples = flag.Bool("examples", false, "同时用 ImageMagick 重新渲染示例 PNG")

// onlyPreset 把本次生成限定在单套预设。
//
// 配合 -examples 使用：不限定时会把所有预设的 PNG 一并重刷，而不同机器的
// ImageMagick 输出字节不同，会给没改过的预设带来无谓的二进制 diff。
var onlyPreset = flag.String("preset", "", "只生成指定预设（默认全部）")

func main() {
	flag.Parse()
	root, err := repoRoot()
	if err != nil {
		fatal(err)
	}
	targets := preset.All()
	if *onlyPreset != "" {
		p, ok := preset.ByID(*onlyPreset)
		if !ok {
			fatal(fmt.Errorf("未知预设 %q，可选：%s", *onlyPreset, strings.Join(preset.IDs(), " ")))
		}
		targets = []preset.Preset{p}
	}
	tpl := loadTemplates()
	for _, p := range targets {
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
		if *withExamples {
			if err := renderExamples(dir, p); err != nil {
				fatal(fmt.Errorf("预设 %s 的示例图: %w", p.ID, err))
			}
		}
	}
}

// renderExamples 把该预设手写的 4 张 SVG 转成 PNG 并拼 contact sheet。
// SVG 是手工绘制的排版基准，生成器不改动它们的内容。
func renderExamples(dir string, p preset.Preset) error {
	examples := filepath.Join(dir, "assets", "style-guide", "examples")
	names := []string{"proposition", "comparison", "process", "capability_deck"}
	pngPaths := make([]string, 0, len(names))
	for _, name := range names {
		svgPath := filepath.Join(examples, name+".svg")
		if _, err := os.Stat(svgPath); err != nil {
			return fmt.Errorf("缺少手写示例图 %s: %w", name+".svg", err)
		}
		pngPath := filepath.Join(examples, name+".png")
		if err := styleimage.RenderSVG(svgPath, pngPath); err != nil {
			return err
		}
		fmt.Println("generated", filepath.ToSlash(filepath.Join("assets", "presets", p.ID, "assets", "style-guide", "examples", name+".png")))
		pngPaths = append(pngPaths, pngPath)
	}
	thumb := fmt.Sprintf("%dx%d", thumbnailWidthPx, thumbnailWidthPx*p.Canvas.HeightPx/p.Canvas.WidthPx)
	if err := styleimage.ContactSheet(pngPaths, filepath.Join(examples, "contact-sheet.png"), thumb, "#D5DEEB"); err != nil {
		return err
	}
	fmt.Println("generated", filepath.ToSlash(filepath.Join("assets", "presets", p.ID, "assets", "style-guide", "examples", "contact-sheet.png")))
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
