package scene

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chouheiwa/articale-to-motion/internal/config"
)

func executable(t *testing.T, dir, name, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix-only release target")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRunWritesLogsAndVerifiesDuration(t *testing.T) {
	dir := writeScene(t, `{"id":"scene-001","duration_seconds":1,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`)
	s, _ := Load(dir)
	bin := t.TempDir()
	executable(t, bin, "codex", `
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"[[USER_MESSAGE]]代码已完成，开始渲染"}}'
printf video > out.mp4
`)
	executable(t, bin, "ffprobe", `printf '1.000\n'`)
	cfg := config.Config{Orchestrator: "claude", Renderer: "codex", TTSProvider: "minimax"}
	var user bytes.Buffer
	base := map[string]string{"PATH": bin, "HOME": t.TempDir()}
	if err := Run(context.Background(), s, cfg, false, base, &user, 0.15); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(user.String(), "代码已完成") {
		t.Fatalf("missing user output: %s", user.String())
	}
	for _, path := range []string{s.StreamLog(), s.StderrLog(), s.UserLog()} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing log %s", path)
		}
	}
}

func TestRunCancellationTerminatesProcessGroup(t *testing.T) {
	dir := writeScene(t, `{"id":"scene-001","duration_seconds":1,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`)
	s, _ := Load(dir)
	bin := t.TempDir()
	executable(t, bin, "codex", `/bin/sleep 300`)
	cfg := config.Config{Orchestrator: "claude", Renderer: "codex", TTSProvider: "minimax"}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := Run(ctx, s, cfg, false, map[string]string{"PATH": bin, "HOME": t.TempDir()}, &bytes.Buffer{}, 0.15)
	if err == nil || !strings.Contains(err.Error(), "中断") {
		t.Fatalf("expected interruption, got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("cancellation took too long")
	}
}

func TestVerifyOutputRejectsInvalidArtifacts(t *testing.T) {
	dir := writeScene(t, `{"id":"scene-001","duration_seconds":1,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`)
	s, _ := Load(dir)
	bin := t.TempDir()
	executable(t, bin, "ffprobe", `printf '2.000\n'`)
	env := map[string]string{"PATH": bin, "HOME": t.TempDir()}
	if err := VerifyOutput(s, env, 0.15); err == nil {
		t.Fatal("missing output should fail")
	}
	os.WriteFile(s.OutputPath(), nil, 0o644)
	if err := VerifyOutput(s, env, 0.15); err == nil {
		t.Fatal("empty output should fail")
	}
	os.WriteFile(s.OutputPath(), []byte("video"), 0o644)
	if err := VerifyOutput(s, env, 0.15); err == nil || !strings.Contains(err.Error(), "时长不符") {
		t.Fatalf("expected duration failure, got %v", err)
	}
	if err := VerifyOutput(s, env, -1); err == nil {
		t.Fatal("negative tolerance should fail")
	}
}

func TestRunReportsRendererFailure(t *testing.T) {
	dir := writeScene(t, `{"id":"scene-001","duration_seconds":1,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`)
	s, _ := Load(dir)
	bin := t.TempDir()
	executable(t, bin, "codex", `echo boom >&2; exit 3`)
	cfg := config.Config{Orchestrator: "claude", Renderer: "codex", TTSProvider: "minimax"}
	err := Run(context.Background(), s, cfg, false, map[string]string{"PATH": bin, "HOME": t.TempDir()}, &bytes.Buffer{}, 0.15)
	if err == nil || !strings.Contains(err.Error(), "退出码 3") {
		t.Fatalf("unexpected error: %v", err)
	}
}
