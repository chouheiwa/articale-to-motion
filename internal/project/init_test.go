package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	assets "github.com/chouheiwa/articale-to-motion"
	"github.com/chouheiwa/articale-to-motion/internal/preset"
)

// builtinSources 返回一次真实 am init 会用到的两棵源树。
func builtinSources(t *testing.T) []fs.FS {
	t.Helper()
	shared, err := assets.Shared()
	if err != nil {
		t.Fatalf("取共享素材：%v", err)
	}
	chosen, err := assets.Preset(preset.Default().ID)
	if err != nil {
		t.Fatalf("取预设素材：%v", err)
	}
	return []fs.FS{shared, chosen}
}

func TestInitializeWritesReusableSkeleton(t *testing.T) {
	target := filepath.Join(t.TempDir(), "video")
	result, err := Initialize(target, builtinSources(t)...)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created == 0 {
		t.Fatal("expected created files")
	}
	for _, name := range []string{
		"PROMPT.md", "PROMPT-PRODUCTION.md", "article-to-motion.conf", ".env.example",
		"frame.md", "templates/publish.md",
		"docs/清晰系统蓝图-视频风格说明书.md",
		"assets/fonts/noto-sans-sc-400.woff2",
		"assets/style-guide/examples/proposition.png",
	} {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(name))); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestInitializeIsIdempotentForIdenticalFiles(t *testing.T) {
	target := filepath.Join(t.TempDir(), "video")
	if _, err := Initialize(target, builtinSources(t)...); err != nil {
		t.Fatal(err)
	}
	result, err := Initialize(target, builtinSources(t)...)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created != 0 || result.Unchanged == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestInitializeRefusesConflictingFileWithoutPartialWrites(t *testing.T) {
	target := t.TempDir()
	os.WriteFile(filepath.Join(target, "PROMPT.md"), []byte("custom"), 0o644)
	if _, err := Initialize(target, builtinSources(t)...); err == nil {
		t.Fatal("expected conflict")
	}
	if _, err := os.Stat(filepath.Join(target, "article-to-motion.conf")); !os.IsNotExist(err) {
		t.Fatal("initializer wrote files before conflict check")
	}
}

func TestInitializeRejectsSymlinkedTemplateDirectory(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(target, "docs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Initialize(target, builtinSources(t)...); err == nil {
		t.Fatal("expected symlinked template directory rejection")
	}
	if _, err := os.Stat(filepath.Join(outside, "清晰系统蓝图-视频风格说明书.md")); !os.IsNotExist(err) {
		t.Fatal("initializer escaped through symlink")
	}
}

func TestInitializeMergesMultipleSources(t *testing.T) {
	shared := fstest.MapFS{
		"PROMPT.md":               {Data: []byte("shared\n")},
		"assets/fonts/noto.woff2": {Data: []byte("font\n")},
	}
	chosen := fstest.MapFS{
		"frame.md":                          {Data: []byte("frame\n")},
		"assets/style-guide/examples/a.svg": {Data: []byte("svg\n")},
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
