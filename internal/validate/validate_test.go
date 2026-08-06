package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPublishTemplatePasses(t *testing.T) {
	if _, err := Publish(filepath.Join("..", "..", "templates", "publish.md"), ".", true); err != nil {
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
	template, _ := os.ReadFile(filepath.Join("..", "..", "templates", "publish.md"))
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

func TestStyleGuidePairPasses(t *testing.T) {
	if err := Style(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
}

func TestPublishRejectsUnsafeYAMLStatusAndMissingHeading(t *testing.T) {
	template, _ := os.ReadFile(filepath.Join("..", "..", "templates", "publish.md"))
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
	frame, _ := os.ReadFile(filepath.Join("..", "..", "frame.md"))
	guide, _ := os.ReadFile(filepath.Join("..", "..", "docs", "清晰系统蓝图-视频风格说明书.md"))
	os.WriteFile(filepath.Join(root, "frame.md"), frame, 0o644)
	os.WriteFile(filepath.Join(root, "docs", "清晰系统蓝图-视频风格说明书.md"), []byte(replaceOnce(string(guide), "width_px: 1080", "width_px: 999")), 0o644)
	if err := Style(root); err == nil {
		t.Fatal("token drift should fail")
	}
}

func TestStyleRejectsInvalidSchemaSections(t *testing.T) {
	frame, _ := os.ReadFile(filepath.Join("..", "..", "frame.md"))
	guide, _ := os.ReadFile(filepath.Join("..", "..", "docs", "清晰系统蓝图-视频风格说明书.md"))
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
	frame, _ := os.ReadFile(filepath.Join("..", "..", "frame.md"))
	guide, _ := os.ReadFile(filepath.Join("..", "..", "docs", "清晰系统蓝图-视频风格说明书.md"))
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
	template, _ := os.ReadFile(filepath.Join("..", "..", "templates", "publish.md"))
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
