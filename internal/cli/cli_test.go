package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHelpListsPublicCommands(t *testing.T) {
	var out bytes.Buffer
	code := Execute([]string{"--help"}, &out, &out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	for _, command := range []string{"init", "run", "scene", "archive", "validate", "config"} {
		if !strings.Contains(out.String(), command) {
			t.Errorf("help missing %s", command)
		}
	}
}

func TestInitCreatesProjectWithoutNetworkWhenSkipped(t *testing.T) {
	target := filepath.Join(t.TempDir(), "project")
	var out bytes.Buffer
	code := Execute([]string{"init", target, "--skip-hyperframes"}, &out, &out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	if _, err := os.Stat(filepath.Join(target, "PROMPT.md")); err != nil {
		t.Fatal(err)
	}
}

func TestConfigGetUsesProjectRoot(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "article-to-motion.conf"), []byte("ORCHESTRATOR=codex\nRENDERER=claude\n"), 0o644)
	old, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(old)
	var out bytes.Buffer
	if code := Execute([]string{"config", "get", "RENDERER"}, &out, &out); code != 0 || strings.TrimSpace(out.String()) != "claude" {
		t.Fatalf("code=%d output=%q", code, out.String())
	}
}

func TestUnknownCommandUsesExitOne(t *testing.T) {
	if code := Execute([]string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Fatalf("got exit %d", code)
	}
}

func TestValidatePublishCommand(t *testing.T) {
	var out bytes.Buffer
	path := filepath.Join("..", "..", "templates", "publish.md")
	if code := Execute([]string{"validate", "publish", path, "--template", "--project-root", filepath.Join("..", "..")}, &out, &out); code != 0 {
		t.Fatalf("code=%d output=%s", code, out.String())
	}
}

func TestRunStartsConfiguredOrchestrator(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	os.WriteFile(filepath.Join(root, "article-to-motion.conf"), []byte("ORCHESTRATOR=codex\nRENDERER=claude\n"), 0o644)
	os.WriteFile(filepath.Join(root, "PROMPT.md"), []byte("hello orchestrator"), 0o644)
	script := "#!/bin/sh\ncat > " + filepath.Join(root, "received.txt") + "\n"
	os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0o755)
	old, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(old)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ORCHESTRATOR", "codex")
	t.Setenv("RENDERER", "claude")
	var out bytes.Buffer
	if code := Execute([]string{"run"}, &out, &out); code != 0 {
		t.Fatalf("code=%d output=%s", code, out.String())
	}
	body, err := os.ReadFile(filepath.Join(root, "received.txt"))
	if err != nil || string(body) != "hello orchestrator" {
		t.Fatalf("received=%q err=%v", body, err)
	}
}

func TestStyleAndEmptyRunAllCommands(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	var out bytes.Buffer
	if code := Execute([]string{"validate", "style", "--project-root", root}, &out, &out); code != 0 {
		t.Fatalf("style code=%d output=%s", code, out.String())
	}
	project := t.TempDir()
	os.WriteFile(filepath.Join(project, "article-to-motion.conf"), []byte("ORCHESTRATOR=codex\nRENDERER=claude\n"), 0o644)
	os.Mkdir(filepath.Join(project, "scenes"), 0o755)
	old, _ := os.Getwd()
	os.Chdir(project)
	defer os.Chdir(old)
	out.Reset()
	if code := Execute([]string{"scene", "run-all", "scenes", "--jobs", "1", "--retries", "0"}, &out, &out); code != 0 {
		t.Fatalf("run-all code=%d output=%s", code, out.String())
	}
}

func TestInitConflictAndInvalidConfigFailCleanly(t *testing.T) {
	target := t.TempDir()
	os.WriteFile(filepath.Join(target, "PROMPT.md"), []byte("custom"), 0o644)
	if code := Execute([]string{"init", target, "--skip-hyperframes"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Fatalf("conflict exit=%d", code)
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "article-to-motion.conf"), []byte("ORCHESTRATOR=codex\nRENDERER=codex\n"), 0o644)
	old, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(old)
	if code := Execute([]string{"config", "get", "RENDERER"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Fatalf("invalid config exit=%d", code)
	}
}

func TestArchiveDryRunDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	git("init", "-b", "main")
	os.WriteFile(filepath.Join(root, "README.md"), []byte("base"), 0o644)
	git("add", ".")
	git("commit", "-m", "base")
	git("switch", "-c", "video/test")
	os.WriteFile(filepath.Join(root, "scene.txt"), []byte("scene"), 0o644)
	old, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(old)
	var out bytes.Buffer
	if code := Execute([]string{"archive", "--dry-run", "--archive-root", filepath.Join(t.TempDir(), "archive")}, &out, &out); code != 0 {
		t.Fatalf("code=%d output=%s", code, out.String())
	}
	if _, err := os.Stat(filepath.Join(root, "scene.txt")); err != nil {
		t.Fatal("dry run mutated project")
	}
}

func TestRunAllRejectsNaNToleranceEvenWhenNoScenesExist(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "article-to-motion.conf"), []byte("ORCHESTRATOR=codex\nRENDERER=claude\n"), 0o644)
	os.Mkdir(filepath.Join(root, "scenes"), 0o755)
	old, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(old)
	if code := Execute([]string{"scene", "run-all", "scenes", "--duration-tolerance", "nan"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Fatalf("nan tolerance exit=%d", code)
	}
}

func TestValidateStyleRegenerateExamplesReportsMissingImageMagick(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := Execute([]string{"validate", "style", "--project-root", root, "--regenerate-examples"}, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "ImageMagick") {
		t.Fatalf("missing actionable error: %s", stderr.String())
	}
}

func TestRunCancellationReturns130AndTerminatesProcessGroup(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	marker := filepath.Join(root, "started")
	os.WriteFile(filepath.Join(root, "article-to-motion.conf"), []byte("ORCHESTRATOR=codex\nRENDERER=claude\n"), 0o644)
	os.WriteFile(filepath.Join(root, "PROMPT.md"), []byte("cancel me"), 0o644)
	script := "#!/bin/sh\ntouch \"" + marker + "\"\nsleep 30 &\nwait\n"
	os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0o755)
	old, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(old)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- ExecuteContext(ctx, []string{"run"}, &bytes.Buffer{}, &bytes.Buffer{}) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("orchestrator did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case code := <-done:
		if code != 130 {
			t.Fatalf("cancel exit=%d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("orchestrator process group was not terminated")
	}
}
