package schedule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chouheiwa/articale-to-motion/internal/scene"
)

func testScene(t *testing.T, root, id string) scene.Scene {
	t.Helper()
	dir := filepath.Join(root, id)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "transcript.srt"), []byte("test"), 0o644)
	os.WriteFile(filepath.Join(dir, "scene.json"), []byte(`{"id":"`+id+`","duration_seconds":1,"output":"`+id+`.mp4","transcript":"transcript.srt","text":"hello"}`), 0o644)
	s, err := scene.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPlanSortsScenesAndIgnoresHelpers(t *testing.T) {
	root := t.TempDir()
	testScene(t, root, "scene-002")
	testScene(t, root, "scene-001")
	os.Mkdir(filepath.Join(root, "attempts"), 0o755)
	got, err := Plan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "scene-001" || got[1].ID != "scene-002" {
		t.Fatalf("unexpected plan: %+v", got)
	}
}

func TestRunAllHonorsConcurrencyAndRetries(t *testing.T) {
	root := t.TempDir()
	scenes := []scene.Scene{testScene(t, root, "scene-001"), testScene(t, root, "scene-002"), testScene(t, root, "scene-003")}
	var active, maximum int32
	attempts := map[string]int{}
	var mu sync.Mutex
	runner := func(ctx context.Context, s scene.Scene) error {
		current := atomic.AddInt32(&active, 1)
		defer atomic.AddInt32(&active, -1)
		for {
			old := atomic.LoadInt32(&maximum)
			if current <= old || atomic.CompareAndSwapInt32(&maximum, old, current) {
				break
			}
		}
		mu.Lock()
		attempts[s.ID]++
		attempt := attempts[s.ID]
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		if s.ID == "scene-002" && attempt == 1 {
			return errors.New("renderer exit code 3")
		}
		return nil
	}
	report := RunAll(context.Background(), scenes, 2, 1, runner)
	if maximum > 2 || report.ExitCode() != 0 || report.Counts()[Succeeded] != 3 || attempts["scene-002"] != 2 {
		t.Fatalf("bad report=%+v max=%d attempts=%v", report, maximum, attempts)
	}
}

func TestRunAllReturnsPartialReportOnCancellation(t *testing.T) {
	root := t.TempDir()
	scenes := []scene.Scene{testScene(t, root, "scene-001"), testScene(t, root, "scene-002")}
	ctx, cancel := context.WithCancel(context.Background())
	runner := func(ctx context.Context, s scene.Scene) error {
		if s.ID == "scene-001" {
			cancel()
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}
	report := RunAll(ctx, scenes, 1, 0, runner)
	if !report.Interrupted || len(report.Scenes) == 0 {
		t.Fatalf("expected partial interrupted report: %+v", report)
	}
}

func TestRunAllRecordsSkipWithoutRetry(t *testing.T) {
	root := t.TempDir()
	scenes := []scene.Scene{testScene(t, root, "scene-001")}
	report := RunAll(context.Background(), scenes, 1, 5, func(context.Context, scene.Scene) error {
		return Outcome(Skipped, "已有合格产物")
	})
	if report.Counts()[Skipped] != 1 || report.Scenes[0].Attempts != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestReportWritesAtomicJSONAndRenders(t *testing.T) {
	report := Report{SchemaVersion: 1, Seconds: 1.5, CountValues: map[Status]int{Succeeded: 1, Skipped: 0, Stale: 0, Failed: 0}, Scenes: []SceneResult{{SceneID: "scene-001", Status: Succeeded, Attempts: 1}}}
	path := filepath.Join(t.TempDir(), "nested", "report.json")
	if err := report.WriteJSON(path); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(body), `"schema_version": 1`) || !strings.Contains(report.Render(), "成功 1") {
		t.Fatalf("body=%s render=%s err=%v", body, report.Render(), err)
	}
	failed := report
	failed.CountValues[Failed] = 1
	if failed.ExitCode() != 1 {
		t.Fatal("failed report must exit one")
	}
}
