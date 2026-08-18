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

// recordSleeps 把重试退避换成记录，测试既不真的等待，又能断言等了多久。
// 返回的读取函数带锁：退避发生在 worker goroutine 上。
func recordSleeps(t *testing.T) func() []time.Duration {
	t.Helper()
	original := waitFunc
	var mu sync.Mutex
	var slept []time.Duration
	waitFunc = func(_ context.Context, d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		slept = append(slept, d)
	}
	t.Cleanup(func() { waitFunc = original })
	return func() []time.Duration {
		mu.Lock()
		defer mu.Unlock()
		return append([]time.Duration(nil), slept...)
	}
}

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
	recordSleeps(t)
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
	if maximum > 2 || report.ExitCode() != 0 || report.CountValues[Succeeded] != 3 || attempts["scene-002"] != 2 {
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
	if report.CountValues[Skipped] != 1 || report.Scenes[0].Attempts != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

// TestFatalErrorsAreNotRetried 守着下发 PROMPT 里的承诺：
// 「产物校验失败不会重试」。每次重试都是一次完整的 AI CLI 调用，
// 而规格不符是确定性的，重跑只会得到同样的产物。
func TestFatalErrorsAreNotRetried(t *testing.T) {
	recordSleeps(t)
	root := t.TempDir()
	scenes := []scene.Scene{testScene(t, root, "scene-001")}
	calls := 0
	report := RunAll(context.Background(), scenes, 1, 5, func(context.Context, scene.Scene) error {
		calls++
		return Fatal(errors.New("产物时长不符：期望 2.833 秒，实测 5.000 秒"))
	})
	if calls != 1 {
		t.Errorf("不可重试的失败被重试了 %d 次", calls)
	}
	if report.CountValues[Failed] != 1 {
		t.Errorf("状态应当是 failed：%+v", report)
	}
	if report.Scenes[0].Attempts != 1 {
		t.Errorf("尝试次数应当如实记为 1，实际 %d", report.Scenes[0].Attempts)
	}
	if !strings.Contains(report.Scenes[0].Reason, "产物时长不符") {
		t.Errorf("原始错误信息丢失：%q", report.Scenes[0].Reason)
	}
}

// TestOrdinaryErrorsStillRetry 是上一条的反面：渲染器非零退出仍要重试，
// 否则一次网络抖动就会判定镜头失败。
func TestOrdinaryErrorsStillRetry(t *testing.T) {
	recordSleeps(t)
	root := t.TempDir()
	scenes := []scene.Scene{testScene(t, root, "scene-001")}
	calls := 0
	RunAll(context.Background(), scenes, 1, 2, func(context.Context, scene.Scene) error {
		calls++
		return errors.New("渲染器退出码 3")
	})
	if calls != 3 {
		t.Errorf("普通失败应当重试到 1+2 次，实际 %d 次", calls)
	}
}

// TestRetriesBackOffBetweenAttempts 守着 PROMPT 里写的「退避 10 秒与 30 秒」。
// 失败后立刻重启 AI CLI 通常只会撞上同一个瞬时故障。
func TestRetriesBackOffBetweenAttempts(t *testing.T) {
	slept := recordSleeps(t)
	root := t.TempDir()
	scenes := []scene.Scene{testScene(t, root, "scene-001")}
	RunAll(context.Background(), scenes, 1, 2, func(context.Context, scene.Scene) error {
		return errors.New("渲染器退出码 3")
	})
	got := slept()
	want := []time.Duration{10 * time.Second, 30 * time.Second}
	if len(got) != len(want) {
		t.Fatalf("退避次数 = %d，期望 %d：%v", len(got), len(want), got)
	}
	for i, d := range want {
		if got[i] != d {
			t.Errorf("第 %d 次退避 = %v，期望 %v", i+1, got[i], d)
		}
	}
}

// TestBackoffDoesNotDelayCancellation：中断时不能还坐等 30 秒退避走完。
// 用真实的 waitFunc，只把退避时长调长，确保测的是取消真的能打断等待。
func TestBackoffDoesNotDelayCancellation(t *testing.T) {
	original := retryBackoff
	retryBackoff = []time.Duration{time.Hour}
	t.Cleanup(func() { retryBackoff = original })

	root := t.TempDir()
	scenes := []scene.Scene{testScene(t, root, "scene-001")}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Report, 1)
	go func() {
		done <- RunAll(ctx, scenes, 1, 3, func(context.Context, scene.Scene) error {
			return errors.New("渲染器退出码 3")
		})
	}()
	// 让第一次失败进入退避，再取消。
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case report := <-done:
		if !report.Interrupted {
			t.Error("取消后报告应当标记 interrupted")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("取消没有打断退避等待：RunAll 卡住了")
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
