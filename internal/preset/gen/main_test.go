package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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
	if !strings.Contains(frame, "height_px: 1920") {
		t.Error("frontmatter 未写入 9:16 画布高度")
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

// 模板标记丢失时必须报错，否则会静默产出没有 canvas 段的 frame.md。
func TestRenderRejectsTemplateWithoutMarker(t *testing.T) {
	p, _ := preset.ByID("vertical-3x4")
	tpl := loadTemplates()
	tpl.Frontmatter = "schema_version: 1\n"
	if _, err := Render(p, tpl); err == nil {
		t.Fatal("缺少标记时应报错")
	}
}

func TestRepoRootWalksUpToGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deep)
	got, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot: %v", err)
	}
	// macOS 上 t.TempDir() 位于 /var，是 /private/var 的符号链接。
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Errorf("repoRoot = %s，期望 %s", gotResolved, wantResolved)
	}
}

func TestRepoRootFailsWithoutGoMod(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := repoRoot(); err == nil {
		t.Fatal("找不到 go.mod 时应报错")
	}
}

// 缺手写 SVG 时必须报错并点名文件，而不是产出一套残缺示例。
func TestRenderExamplesReportsMissingSVG(t *testing.T) {
	p, _ := preset.ByID("vertical-3x4")
	err := renderExamples(t.TempDir(), p, io.Discard)
	if err == nil {
		t.Fatal("缺少 SVG 时应报错")
	}
	if !strings.Contains(err.Error(), "proposition.svg") {
		t.Errorf("错误应点名缺失文件，实际：%v", err)
	}
}

func TestCanvasAndSafeAreaRejectsBrokenAnchor(t *testing.T) {
	p := preset.Preset{
		ID:       "broken",
		Canvas:   preset.Canvas{WidthPx: 1080, HeightPx: 1440, FPS: 30, Orientation: "vertical"},
		SafeArea: []preset.Box{{Name: "x", Anchor: preset.Anchor("middle")}},
	}
	if _, err := canvasAndSafeArea(p); err == nil {
		t.Fatal("非法 anchor 应报错")
	}
}

// 生成的 frame.md 必须能被 validate 的 frontmatter 切分逻辑正确解析。
func TestRenderProducesParseableFrontmatter(t *testing.T) {
	for _, p := range preset.All() {
		t.Run(p.ID, func(t *testing.T) {
			files, err := Render(p, loadTemplates())
			if err != nil {
				t.Fatal(err)
			}
			body := string(files["frame.md"])
			if !strings.HasPrefix(body, "---\n") {
				t.Error("frame.md 未以 --- 开头")
			}
			if strings.Count(body, "\n---") < 1 {
				t.Error("frame.md 缺少 frontmatter 结束标记")
			}
			if strings.Contains(body, canvasMarker) {
				t.Error("canvas 标记未被替换")
			}
		})
	}
}

func TestGenerateWritesEveryPresetIntoRoot(t *testing.T) {
	root := t.TempDir()
	var log bytes.Buffer
	if err := generate(root, preset.All(), loadTemplates(), false, &log); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, p := range preset.All() {
		for _, name := range []string{"frame.md", "PROMPT-PRODUCTION.md", filepath.Join("docs", "清晰系统蓝图-视频风格说明书.md")} {
			path := filepath.Join(root, "assets", "presets", p.ID, name)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("缺少 %s/%s: %v", p.ID, name, err)
			}
			if !strings.Contains(string(body), p.Canvas.Label()) && name == "PROMPT-PRODUCTION.md" {
				t.Errorf("%s/%s 未写入画幅 %s", p.ID, name, p.Canvas.Label())
			}
		}
	}
	// 日志按路径排序，便于 diff 比对。
	if !strings.Contains(log.String(), "generated assets/presets/vertical-9x16/frame.md") {
		t.Errorf("日志缺少 9:16 产物：%s", log.String())
	}
}

// generate 的产物必须与仓库里签入的一致——这正是 CI 幂等门要保证的事。
func TestGenerateMatchesCommittedAssets(t *testing.T) {
	root := t.TempDir()
	if err := generate(root, preset.All(), loadTemplates(), false, io.Discard); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, p := range preset.All() {
		for _, name := range []string{"frame.md", "PROMPT-PRODUCTION.md", filepath.Join("docs", "清晰系统蓝图-视频风格说明书.md")} {
			fresh, err := os.ReadFile(filepath.Join(root, "assets", "presets", p.ID, name))
			if err != nil {
				t.Fatal(err)
			}
			committed, err := os.ReadFile(filepath.Join("..", "..", "..", "assets", "presets", p.ID, name))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(fresh, committed) {
				t.Errorf("%s/%s 与签入产物不一致，需重跑 go generate", p.ID, name)
			}
		}
	}
}

func TestSelectTargets(t *testing.T) {
	all, err := selectTargets("")
	if err != nil || len(all) != len(preset.All()) {
		t.Fatalf("空 id 应返回全部：%v %v", len(all), err)
	}
	one, err := selectTargets("vertical-9x16")
	if err != nil || len(one) != 1 || one[0].ID != "vertical-9x16" {
		t.Fatalf("单选失败：%v %v", one, err)
	}
	if _, err := selectTargets("landscape-16x9"); err == nil {
		t.Fatal("未知 id 应报错")
	}
}
