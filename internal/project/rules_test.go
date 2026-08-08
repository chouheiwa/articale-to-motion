package project

import (
	"os"
	"path/filepath"
	"testing"
)

func seedRulesTemplate(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, RulesTemplate), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteRulesRendersTemplateUnderToolFilename(t *testing.T) {
	root := t.TempDir()
	seedRulesTemplate(t, root, "# 项目规则\n")

	path, err := WriteRules(root, "CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "CLAUDE.md"); path != want {
		t.Fatalf("got %s want %s", path, want)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# 项目规则\n" {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestWriteRulesIsIdempotent(t *testing.T) {
	root := t.TempDir()
	seedRulesTemplate(t, root, "# 项目规则\n")

	if _, err := WriteRules(root, "AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRules(root, "AGENTS.md"); err != nil {
		t.Fatalf("second call must not fail: %v", err)
	}
}

func TestWriteRulesRefusesToOverwriteDifferentContent(t *testing.T) {
	root := t.TempDir()
	seedRulesTemplate(t, root, "# 项目规则\n")
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("我自己写的规则\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := WriteRules(root, "CLAUDE.md"); err == nil {
		t.Fatal("expected refusal to clobber a hand-written instruction file")
	}
	body, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "我自己写的规则\n" {
		t.Fatalf("existing file was modified: %q", body)
	}
}

// 老版本 am init 出来的项目没有这份模板。缺规则不该阻断整条制作流程。
func TestWriteRulesSkipsWhenTemplateMissing(t *testing.T) {
	path, err := WriteRules(t.TempDir(), "AGENTS.md")
	if err != nil {
		t.Fatalf("missing template must not be an error: %v", err)
	}
	if path != "" {
		t.Fatalf("expected empty path, got %s", path)
	}
}

func TestWriteRulesRejectsSymlinkedTarget(t *testing.T) {
	root := t.TempDir()
	seedRulesTemplate(t, root, "# 项目规则\n")
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("别的文件\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	if _, err := WriteRules(root, "AGENTS.md"); err == nil {
		t.Fatal("expected refusal to write through a symlink")
	}
	body, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "别的文件\n" {
		t.Fatalf("symlink target was written through: %q", body)
	}
}

// 模板必须随 am init 下发，否则 am run 永远只会打印「缺模板」。
func TestInitializeShipsRulesTemplate(t *testing.T) {
	target := filepath.Join(t.TempDir(), "video")
	if _, err := Initialize(target, builtinSources(t)...); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, RulesTemplate)); err != nil {
		t.Fatalf("missing %s: %v", RulesTemplate, err)
	}
}
