package project

import (
	"os"
	"path/filepath"
	"testing"

	assets "github.com/chouheiwa/articale-to-motion"
)

func TestInitializeWritesReusableSkeleton(t *testing.T) {
	target := filepath.Join(t.TempDir(), "video")
	result, err := Initialize(target, assets.Files)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created == 0 {
		t.Fatal("expected created files")
	}
	for _, name := range []string{"PROMPT.md", "PROMPT-PRODUCTION.md", "article-to-motion.conf", ".env.example", "frame.md", "templates/publish.md"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestInitializeIsIdempotentForIdenticalFiles(t *testing.T) {
	target := filepath.Join(t.TempDir(), "video")
	if _, err := Initialize(target, assets.Files); err != nil {
		t.Fatal(err)
	}
	result, err := Initialize(target, assets.Files)
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
	if _, err := Initialize(target, assets.Files); err == nil {
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
	if _, err := Initialize(target, assets.Files); err == nil {
		t.Fatal("expected symlinked template directory rejection")
	}
	if _, err := os.Stat(filepath.Join(outside, "清晰系统蓝图-视频风格说明书.md")); !os.IsNotExist(err) {
		t.Fatal("initializer escaped through symlink")
	}
}
