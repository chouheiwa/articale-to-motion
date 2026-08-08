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
