package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/preset"
)

func TestPublishTemplatePasses(t *testing.T) {
	if _, err := Publish(sharedSource("templates", "publish.md"), ".", true); err != nil {
		t.Fatal(err)
	}
}

func TestPublishRejectsMissingRequiredKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "publish.md")
	os.WriteFile(path, []byte("---\nschema_version: 1\n---\n"), 0o644)
	if _, err := Publish(path, ".", true); err == nil {
		t.Fatal("expected missing key error")
	}
}

func TestPublishRejectsSecretsAndEscapingPaths(t *testing.T) {
	template, _ := os.ReadFile(sharedSource("templates", "publish.md"))
	for name, replacement := range map[string]string{"secret": "introduction: sk-abcdefghijklmnop", "escape": "path: ../final.mp4"} {
		t.Run(name, func(t *testing.T) {
			body := string(template)
			if name == "secret" {
				body = replaceOnce(body, "introduction: \"\"", replacement)
			} else {
				body = replaceOnce(body, "path: final.mp4", replacement)
			}
			path := filepath.Join(t.TempDir(), "publish.md")
			os.WriteFile(path, []byte(body), 0o644)
			if _, err := Publish(path, filepath.Dir(path), true); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

// 素材拆成两棵源树后仓库根不再是一个可校验的项目，改为在铺好的夹具上验。
func TestStyleGuidePairPasses(t *testing.T) {
	frame, _ := os.ReadFile(presetSource("frame.md"))
	guide, _ := os.ReadFile(presetSource("docs", "清晰系统蓝图-视频风格说明书.md"))
	if err := Style(styleFixture(t, string(frame), string(guide))); err != nil {
		t.Fatal(err)
	}
}

func TestPublishRejectsUnsafeYAMLStatusAndMissingHeading(t *testing.T) {
	template, _ := os.ReadFile(sharedSource("templates", "publish.md"))
	cases := map[string]string{
		"unsafe-tag":      replaceOnce(string(template), "schema_version: 1", "schema_version: !custom 1"),
		"bad-status":      replaceOnce(string(template), "publish_status: draft", "publish_status: published"),
		"missing-heading": replaceOnce(string(template), "## 发布平台", "## removed"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "publish.md")
			os.WriteFile(path, []byte(body), 0o644)
			if _, err := Publish(path, filepath.Dir(path), true); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestStyleRejectsTokenDriftAndMissingDocuments(t *testing.T) {
	root := t.TempDir()
	if err := Style(root); err == nil {
		t.Fatal("missing style files should fail")
	}
	os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	frame, _ := os.ReadFile(presetSource("frame.md"))
	guide, _ := os.ReadFile(presetSource("docs", "清晰系统蓝图-视频风格说明书.md"))
	os.WriteFile(filepath.Join(root, "frame.md"), frame, 0o644)
	os.WriteFile(filepath.Join(root, "docs", "清晰系统蓝图-视频风格说明书.md"), []byte(replaceOnce(string(guide), "width_px: 1080", "width_px: 999")), 0o644)
	if err := Style(root); err == nil {
		t.Fatal("token drift should fail")
	}
}

func TestStyleRejectsInvalidSchemaSections(t *testing.T) {
	frame, _ := os.ReadFile(presetSource("frame.md"))
	guide, _ := os.ReadFile(presetSource("docs", "清晰系统蓝图-视频风格说明书.md"))
	cases := map[string][2]string{
		"schema":     {"schema_version: 1", "schema_version: 2"},
		"identity":   {"style_id: clear-system-blueprint-v1", "style_id: [invalid]"},
		"scope":      {"  - knowledge_explainer", "  []"},
		"canvas":     {"width_px: 1080", "width_px: 999"},
		"safe-area":  {"left_px: 40", "left_px: -1"},
		"colors":     {`canvas: "#F5F7FB"`, `canvas: "not-a-color"`},
		"typography": {"    cover_min: 78", "    cover_min: 0"},
		"spacing":    {"  content_left_px: 88", "  content_left_px: -1"},
		"radius":     {"  pill_px: 999", "  pill_px: 10"},
		"archetypes": {"  - id: proposition", "  - id: unknown"},
		"motion":     {"phases: {build: 0.30, breathe: 0.40, resolve: 0.30}", "phases: {build: 0.30, breathe: 0.40, resolve: 0.10}"},
		"audio":      {"  sample_rate_hz: 48000", "  sample_rate_hz: 44100"},
		"subtitles":  {"  required: false", "  removed: false"},
		"cover":      {"  stable_frames: 18", "  stable_frames: 1"},
		"forbidden":  {"forbidden:\n  - black_or_blank_frame_zero", "forbidden: []\n  # black_or_blank_frame_zero"},
	}
	for name, replacement := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			os.MkdirAll(filepath.Join(root, "docs"), 0o755)
			modified := replaceOnce(string(frame), replacement[0], replacement[1])
			if modified == string(frame) {
				t.Fatalf("fixture replacement did not match: %q", replacement[0])
			}
			os.WriteFile(filepath.Join(root, "frame.md"), []byte(modified), 0o644)
			os.WriteFile(filepath.Join(root, "docs", "清晰系统蓝图-视频风格说明书.md"), guide, 0o644)
			if err := Style(root); err == nil {
				t.Fatal("expected schema rejection")
			}
		})
	}
}

// styleFixture 复制出一份完整的风格资产树（含 assets/），这样 Style 的失败一定来自
// 被测的那处改动，而不是缺文件——否则用例会空过。
func styleFixture(t *testing.T, frameBody, guideBody string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "frame.md"), []byte(frameBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "清晰系统蓝图-视频风格说明书.md"), []byte(guideBody), 0o644); err != nil {
		t.Fatal(err)
	}
	// 源在两棵素材树里，目标是项目内的扁平路径——两边不再同名，得分别给出。
	for _, pair := range []struct{ source, destination string }{
		{presetSource("assets", "style-guide", "examples"), filepath.Join("assets", "style-guide", "examples")},
		{sharedSource("assets", "fonts"), filepath.Join("assets", "fonts")},
	} {
		entries, err := os.ReadDir(pair.source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, pair.destination), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			body, err := os.ReadFile(filepath.Join(pair.source, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, pair.destination, entry.Name()), body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// 素材已按画幅拆进 assets/presets 与 assets/shared 两棵源树，
// 测试夹具统一走这两个辅助函数，避免相对路径散落各处。
func presetSource(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "assets", "presets", preset.Default().ID}, parts...)...)
}

func sharedSource(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "assets", "shared"}, parts...)...)
}

func TestStyleFixtureItselfPasses(t *testing.T) {
	frame, _ := os.ReadFile(presetSource("frame.md"))
	guide, _ := os.ReadFile(presetSource("docs", "清晰系统蓝图-视频风格说明书.md"))
	if err := Style(styleFixture(t, string(frame), string(guide))); err != nil {
		t.Fatalf("unmodified fixture must pass, otherwise every rejection case passes vacuously: %v", err)
	}
}

// 渲染机是一台干净的无头 Chrome。字体栈里出现只存在于本机的系统字体时，
// 本地渲染会因为回退而"看着正常"，云端 / CI 上排版却是错的——必须在 token 层挡住。
func TestStyleRejectsFontsThatDoNotShipWithTheProject(t *testing.T) {
	frame, _ := os.ReadFile(presetSource("frame.md"))
	guide, _ := os.ReadFile(presetSource("docs", "清晰系统蓝图-视频风格说明书.md"))
	cases := map[string][2]string{
		"system-font-in-primary": {
			`primary_stack: '"Inter", "Noto Sans SC", sans-serif'`,
			`primary_stack: '"Inter", "PingFang SC", sans-serif'`,
		},
		"system-font-in-mono": {
			`mono_stack: '"JetBrains Mono", "Noto Sans SC", monospace'`,
			`mono_stack: '"JetBrains Mono", "Hiragino Sans GB", monospace'`,
		},
		"declaration-without-use": {
			`    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}`,
			`    - {family: "Unused Face", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}`,
		},
		"missing-font-files": {
			"  font_files:",
			"  unused_key:",
		},
	}
	for name, replacement := range cases {
		t.Run(name, func(t *testing.T) {
			// 两份文件一起改，否则失败原因会是 token 漂移而不是字体规则。
			modifiedFrame := replaceOnce(string(frame), replacement[0], replacement[1])
			modifiedGuide := replaceOnce(string(guide), replacement[0], replacement[1])
			if modifiedFrame == string(frame) || modifiedGuide == string(guide) {
				t.Fatalf("fixture replacement did not match: %q", replacement[0])
			}
			err := Style(styleFixture(t, modifiedFrame, modifiedGuide))
			if err == nil {
				t.Fatal("expected the font stack to be rejected")
			}
			if !strings.Contains(err.Error(), "typography") {
				t.Fatalf("rejection must come from the font rule, got %v", err)
			}
		})
	}
}

func TestStyleRejectsDeclaredFontFileThatIsMissing(t *testing.T) {
	frame, _ := os.ReadFile(presetSource("frame.md"))
	guide, _ := os.ReadFile(presetSource("docs", "清晰系统蓝图-视频风格说明书.md"))
	const old, replacement = `file: "assets/fonts/noto-sans-sc-400.woff2"`, `file: "assets/fonts/absent.woff2"`
	modifiedFrame := replaceOnce(string(frame), old, replacement)
	modifiedGuide := replaceOnce(string(guide), old, replacement)
	if modifiedFrame == string(frame) || modifiedGuide == string(guide) {
		t.Fatal("fixture replacement did not match")
	}
	err := Style(styleFixture(t, modifiedFrame, modifiedGuide))
	if err == nil {
		t.Fatal("expected the missing font file to be rejected")
	}
	if !strings.Contains(err.Error(), "absent.woff2") {
		t.Fatalf("error should name the missing file, got %v", err)
	}
}

func TestRegenerateExamplesUsesImageMagickAndWritesAllArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	frame, _ := os.ReadFile(presetSource("frame.md"))
	guide, _ := os.ReadFile(presetSource("docs", "清晰系统蓝图-视频风格说明书.md"))
	os.WriteFile(filepath.Join(root, "frame.md"), frame, 0o644)
	os.WriteFile(filepath.Join(root, "docs", "清晰系统蓝图-视频风格说明书.md"), guide, 0o644)
	magick := filepath.Join(bin, "magick")
	os.WriteFile(magick, []byte("#!/bin/sh\nfor last; do :; done\n: > \"$last\"\n"), 0o755)
	t.Setenv("PATH", bin)
	var output bytes.Buffer
	if err := RegenerateExamples(root, &output); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"proposition.svg", "proposition.png", "comparison.svg", "comparison.png", "process.svg", "process.png", "capability_deck.svg", "capability_deck.png", "contact-sheet.png"} {
		if _, err := os.Stat(filepath.Join(root, "assets", "style-guide", "examples", name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if !strings.Contains(output.String(), "contact-sheet.png") {
		t.Fatalf("generation output missing contact sheet: %s", output.String())
	}
}

func TestPNGDimensionsRejectsTruncatedAndNonPNGFiles(t *testing.T) {
	for name, body := range map[string][]byte{
		"truncated":       []byte("short"),
		"wrong-signature": append([]byte("not-png!"), make([]byte, 16)...),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			os.WriteFile(path, body, 0o644)
			if _, _, err := pngDimensions(path); err == nil {
				t.Fatal("expected invalid PNG error")
			}
		})
	}
	if _, _, err := pngDimensions(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Fatal("expected missing PNG error")
	}
	if err := validateNumericMap(map[string]any{"bad": ".nan"}, "numbers", 0); err == nil {
		t.Fatal("expected non-numeric value rejection")
	}
}

func TestPublishRejectsEvidenceSymlinkOutsideProject(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "evidence.txt")
	os.WriteFile(outside, []byte("outside"), 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "evidence.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	template, _ := os.ReadFile(sharedSource("templates", "publish.md"))
	body := replaceOnce(string(template), `approval: ""`, `approval: evidence.txt`)
	path := filepath.Join(root, "publish.md")
	os.WriteFile(path, []byte(body), 0o644)
	if _, err := Publish(path, root, true); err == nil {
		t.Fatal("expected external symlink rejection")
	}
}

func replaceOnce(body, old, replacement string) string {
	for i := 0; i+len(old) <= len(body); i++ {
		if body[i:i+len(old)] == old {
			return body[:i] + replacement + body[i+len(old):]
		}
	}
	return body
}
