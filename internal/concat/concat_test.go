package concat

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/envutil"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
)

func toolchain(t *testing.T) mediaprobe.Toolchain {
	t.Helper()
	for _, tool := range []string{"ffprobe", "ffmpeg"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("本机没有 %s，跳过", tool)
		}
	}
	tc, err := mediaprobe.New(envutil.EnvMap())
	if err != nil {
		t.Fatal(err)
	}
	return tc
}

func target() Target {
	return Target{WidthPx: 320, HeightPx: 240, FPS: 30, Codec: DefaultCodec, PixelFormat: DefaultPixelFormat}
}

// clip 生成一段测试素材。extra 用于制造与目标规格的差异。
func clip(t *testing.T, dir, name, filter string, extra ...string) string {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("本机没有 ffmpeg")
	}
	path := filepath.Join(dir, name)
	argv := []string{"-v", "error", "-y", "-f", "lavfi", "-i", filter}
	argv = append(argv, extra...)
	if len(extra) == 0 {
		argv = append(argv, "-c:v", "libx264", "-pix_fmt", "yuv420p")
	}
	argv = append(argv, path)
	if output, err := exec.Command(ffmpeg, argv...).CombinedOutput(); err != nil {
		t.Fatalf("生成 %s 失败：%v\n%s", name, err, output)
	}
	return path
}

func TestAnalyzeAcceptsUniformClips(t *testing.T) {
	tc := toolchain(t)
	dir := t.TempDir()
	inputs := []Input{
		{ID: "scene-001", Path: clip(t, dir, "a.mp4", "testsrc=size=320x240:rate=30:duration=1")},
		{ID: "scene-002", Path: clip(t, dir, "b.mp4", "testsrc=size=320x240:rate=30:duration=2")},
	}
	plan, err := Analyze(tc, inputs, target())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Blocking) != 0 {
		t.Errorf("规格一致却报了阻断问题：%v", plan.Blocking)
	}
	if len(plan.Normalize) != 0 {
		t.Errorf("规格一致不该需要规范化：%v", plan.Normalize)
	}
	if math.Abs(plan.TotalSeconds-3) > 0.1 {
		t.Errorf("总时长 = %v，期望约 3", plan.TotalSeconds)
	}
}

// TestAnalyzeRefusesToRescaleOrRetime 是这个包最重要的一条约束。
//
// PROMPT 明令「规范化不得承担语义重定时，禁止通过 setpts 或改变帧率让镜头
// 追赶配音」。分辨率和帧率不符必须回去重渲染，而不是在这里悄悄缩放或改帧率——
// 提示词里的禁令可以被忽略，不存在的能力不能。
func TestAnalyzeRefusesToRescaleOrRetime(t *testing.T) {
	tc := toolchain(t)
	dir := t.TempDir()

	wrongSize := []Input{{ID: "scene-001", Path: clip(t, dir, "small.mp4", "testsrc=size=160x120:rate=30:duration=1")}}
	plan, err := Analyze(tc, wrongSize, target())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Normalize) != 0 {
		t.Error("分辨率不符绝不能靠规范化解决")
	}
	if len(plan.Blocking) != 1 || !strings.Contains(plan.Blocking[0], "重渲染") {
		t.Errorf("分辨率不符应当阻断并要求重渲染：%v", plan.Blocking)
	}

	wrongFPS := []Input{{ID: "scene-002", Path: clip(t, dir, "slow.mp4", "testsrc=size=320x240:rate=15:duration=1")}}
	plan, err = Analyze(tc, wrongFPS, target())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Normalize) != 0 {
		t.Error("帧率不符绝不能靠规范化解决")
	}
	if len(plan.Blocking) != 1 || !strings.Contains(plan.Blocking[0], "时间轴") {
		t.Errorf("帧率不符应当阻断并说明理由：%v", plan.Blocking)
	}
}

func TestAnalyzeRejectsClipsWithAudio(t *testing.T) {
	tc := toolchain(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "noisy.mp4")
	ffmpeg, _ := exec.LookPath("ffmpeg")
	cmd := exec.Command(ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成素材失败：%v\n%s", err, output)
	}
	plan, err := Analyze(tc, []Input{{ID: "scene-001", Path: path}}, target())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Blocking) != 1 || !strings.Contains(plan.Blocking[0], "音轨") {
		t.Errorf("静音母版不允许带音轨：%v", plan.Blocking)
	}
}

func TestAnalyzeFlagsPixelFormatForNormalization(t *testing.T) {
	tc := toolchain(t)
	dir := t.TempDir()
	// yuv444p：编码和画幅都对，只有像素格式不同——这才是规范化该处理的情况。
	path := clip(t, dir, "444.mp4", "testsrc=size=320x240:rate=30:duration=1",
		"-c:v", "libx264", "-pix_fmt", "yuv444p")
	plan, err := Analyze(tc, []Input{{ID: "scene-001", Path: path}}, target())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Blocking) != 0 {
		t.Errorf("像素格式差异不该阻断：%v", plan.Blocking)
	}
	if len(plan.Normalize) != 1 {
		t.Errorf("像素格式差异应当走规范化：%v", plan.Normalize)
	}
}

func TestNormalizePreservesTimelineAndDropsAudio(t *testing.T) {
	tc := toolchain(t)
	runner, err := NewRunner(envutil.EnvMap(), tc)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source := clip(t, dir, "444.mp4", "testsrc=size=320x240:rate=30:duration=2",
		"-c:v", "libx264", "-pix_fmt", "yuv444p")
	before, err := tc.CountFrames(source)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "normalized", "out.mp4")
	if err := runner.Normalize(context.Background(), source, dest, target()); err != nil {
		t.Fatal(err)
	}
	after, err := tc.CountFrames(dest)
	if err != nil {
		t.Fatal(err)
	}
	// 帧数一帧不多一帧不少：规范化只统一格式，不碰时间轴。
	if before != after {
		t.Errorf("规范化改变了帧数：%d -> %d", before, after)
	}
	media, err := tc.Probe(dest)
	if err != nil {
		t.Fatal(err)
	}
	if media.Video.PixelFormat != "yuv420p" {
		t.Errorf("像素格式 = %s，期望 yuv420p", media.Video.PixelFormat)
	}
	if media.Audio != nil {
		t.Error("规范化产物不该有音轨")
	}
	if _, err := os.Stat(source); err != nil {
		t.Error("原始文件必须保留")
	}
}

func TestConcatProducesContinuousMaster(t *testing.T) {
	tc := toolchain(t)
	runner, err := NewRunner(envutil.EnvMap(), tc)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a := clip(t, dir, "a.mp4", "testsrc=size=320x240:rate=30:duration=1")
	b := clip(t, dir, "b.mp4", "testsrc=size=320x240:rate=30:duration=2")
	dest := filepath.Join(dir, "production", "silent-master.mp4")
	if err := runner.Concat(context.Background(), []string{a, b}, dest); err != nil {
		t.Fatal(err)
	}
	frames, err := tc.CountFrames(dest)
	if err != nil {
		t.Fatal(err)
	}
	if frames != 90 {
		t.Errorf("母版帧数 = %d，期望 90（30 + 60）", frames)
	}
	media, err := tc.Probe(dest)
	if err != nil {
		t.Fatal(err)
	}
	if media.Audio != nil {
		t.Error("静音母版不该有音轨")
	}
	// 清单文件是中间产物，不能留在交付目录里。清单文件名现在是 CreateTemp
	// 生成的随机名（见并发测试），用通配符匹配而不是旧的固定名。
	leftovers, err := filepath.Glob(filepath.Join(dir, "production", ".am-concat-list-*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Errorf("拼接清单没有清理：%v", leftovers)
	}
}

// TestConcatConcurrentRunsDoNotClobberEachOther 守住临时清单文件名的并发安全：
// 固定文件名 .am-concat-list.txt 在同一目录下并发跑两个 Concat 会互相覆盖对方
// 还没读完的清单，产出错误的拼接结果（缺一段、内容错乱）或直接失败。用
// os.CreateTemp 拿唯一文件名后，两次并发调用各自读到自己的清单，互不干扰。
func TestConcatConcurrentRunsDoNotClobberEachOther(t *testing.T) {
	tc := toolchain(t)
	runner, err := NewRunner(envutil.EnvMap(), tc)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a := clip(t, dir, "a.mp4", "testsrc=size=320x240:rate=30:duration=1")
	b := clip(t, dir, "b.mp4", "testsrc=size=320x240:rate=30:duration=2")

	destA := filepath.Join(dir, "out-a.mp4")
	destB := filepath.Join(dir, "out-b.mp4")

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		// out-a 只拼 a（1 秒/30 帧）。
		if err := runner.Concat(context.Background(), []string{a}, destA); err != nil {
			errs <- fmt.Errorf("并发拼接 A 失败：%w", err)
		}
	}()
	go func() {
		defer wg.Done()
		// out-b 拼 a+b（1+2 秒/90 帧），两个 goroutine 同时往 dir 下写清单。
		if err := runner.Concat(context.Background(), []string{a, b}, destB); err != nil {
			errs <- fmt.Errorf("并发拼接 B 失败：%w", err)
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	framesA, err := tc.CountFrames(destA)
	if err != nil {
		t.Fatal(err)
	}
	if framesA != 30 {
		t.Errorf("并发场景下 out-a 帧数 = %d，期望 30——清单被另一次 Concat 覆盖会产出错误的帧数", framesA)
	}
	framesB, err := tc.CountFrames(destB)
	if err != nil {
		t.Fatal(err)
	}
	if framesB != 90 {
		t.Errorf("并发场景下 out-b 帧数 = %d，期望 90（30 + 60）——清单被另一次 Concat 覆盖会产出错误的帧数", framesB)
	}
}

func TestConcatHandlesPathsWithQuotes(t *testing.T) {
	tc := toolchain(t)
	runner, err := NewRunner(envutil.EnvMap(), tc)
	if err != nil {
		t.Fatal(err)
	}
	// concat demuxer 的清单里单引号必须转义，否则路径会被截断。
	dir := filepath.Join(t.TempDir(), "it's a dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := clip(t, dir, "a.mp4", "testsrc=size=320x240:rate=30:duration=1")
	dest := filepath.Join(t.TempDir(), "out.mp4")
	if err := runner.Concat(context.Background(), []string{a}, dest); err != nil {
		t.Fatalf("含单引号的路径拼接失败：%v", err)
	}
	if _, err := tc.Probe(dest); err != nil {
		t.Errorf("产物不可读：%v", err)
	}
}

func TestConcatRejectsEmptyInput(t *testing.T) {
	tc := toolchain(t)
	runner, err := NewRunner(envutil.EnvMap(), tc)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Concat(context.Background(), nil, filepath.Join(t.TempDir(), "x.mp4")); err == nil {
		t.Error("空片段列表应当报错")
	}
}

func TestAnalyzeReportsEmptyInput(t *testing.T) {
	tc := toolchain(t)
	plan, err := Analyze(tc, nil, target())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Blocking) != 1 {
		t.Errorf("空输入应当报出阻断问题：%v", plan.Blocking)
	}
}

// mediaprobe.Probe 放行纯音频文件后，Video 可能为 nil。混进拼接输入的纯音频文件
// 说明上游镜头渲染出了错，这里要落成一条 Blocking 记录而不是对 nil 的 Video 取
// 字段导致 panic。
func TestAnalyzeReportsAudioOnlyClipAsBlocking(t *testing.T) {
	tc := toolchain(t)
	dir := t.TempDir()
	ffmpeg, _ := exec.LookPath("ffmpeg")
	path := filepath.Join(dir, "audio-only.wav")
	cmd := exec.Command(ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成素材失败：%v\n%s", err, output)
	}
	plan, err := Analyze(tc, []Input{{ID: "scene-001", Path: path}}, target())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Blocking) != 1 || !strings.Contains(plan.Blocking[0], "scene-001") || !strings.Contains(plan.Blocking[0], "没有视频流") {
		t.Errorf("纯音频输入应当报出阻断问题并点名 ID：%v", plan.Blocking)
	}
}
