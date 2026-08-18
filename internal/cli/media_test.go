package cli

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCanvas(t *testing.T) {
	cases := []struct {
		input  string
		width  int
		height int
		ok     bool
	}{
		{"1080x1440", 1080, 1440, true},
		{"1080X1920", 1080, 1920, true},
		{" 1080 x 1920 ", 1080, 1920, true},
		{"1080", 0, 0, false},
		{"1080x", 0, 0, false},
		{"x1440", 0, 0, false},
		{"0x1440", 0, 0, false},
		{"-1080x1440", 0, 0, false},
		{"axb", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, tc := range cases {
		width, height, err := parseCanvas(tc.input)
		if tc.ok && (err != nil || width != tc.width || height != tc.height) {
			t.Errorf("parseCanvas(%q) = %d, %d, %v；期望 %d, %d, nil", tc.input, width, height, err, tc.width, tc.height)
		}
		if !tc.ok && err == nil {
			t.Errorf("parseCanvas(%q) 应当报错", tc.input)
		}
	}
}

func TestParseTimePointsSupportsSecondsPercentAndEnd(t *testing.T) {
	// 时长 10 秒、30fps：末帧在 10 - 1/30 ≈ 9.9667 秒。
	points, err := parseTimePoints("0,25%,50%,end", 10, 30)
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{0, 2.5, 5, 10 - 1.0/30}
	if len(points) != len(want) {
		t.Fatalf("解析出 %d 个点，期望 %d：%v", len(points), len(want), points)
	}
	for i, value := range want {
		if math.Abs(points[i]-value) > 1e-9 {
			t.Errorf("第 %d 个点 = %v，期望 %v", i, points[i], value)
		}
	}
}

// TestParseTimePointsEndStaysWithinLastFrame：end 直接取时长会超出末帧，
// ffmpeg 会返回成功却什么都不输出——静默产出空文件比报错更危险。
func TestParseTimePointsEndStaysWithinLastFrame(t *testing.T) {
	points, err := parseTimePoints("end", 2, 30)
	if err != nil {
		t.Fatal(err)
	}
	if points[0] >= 2 {
		t.Errorf("end = %v，必须严格小于时长 2", points[0])
	}
	// 100% 同样要被夹到末帧内。
	points, err = parseTimePoints("100%", 2, 30)
	if err != nil {
		t.Fatal(err)
	}
	if points[0] >= 2 {
		t.Errorf("100%% = %v，必须严格小于时长 2", points[0])
	}
}

func TestParseTimePointsRejectsBadInput(t *testing.T) {
	cases := []string{"", "  ", "abc", "-1", "101%", "-5%", "99", "1,abc"}
	for _, input := range cases {
		if _, err := parseTimePoints(input, 10, 30); err == nil {
			t.Errorf("parseTimePoints(%q) 应当报错", input)
		}
	}
}

func TestParseTimePointsToleratesMissingFPS(t *testing.T) {
	// fps 为 0 时退回 30，不能除零。
	points, err := parseTimePoints("end", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if points[0] <= 0 || points[0] >= 1 {
		t.Errorf("end = %v，期望落在 (0, 1)", points[0])
	}
}

// --- 以下需要真实 ffmpeg/ffprobe ---

func requireFFmpeg(t *testing.T) string {
	t.Helper()
	for _, tool := range []string{"ffprobe", "ffmpeg"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("本机没有 %s，跳过", tool)
		}
	}
	path, _ := exec.LookPath("ffmpeg")
	return path
}

func makeClip(t *testing.T, path, filter string, extra ...string) {
	t.Helper()
	ffmpeg := requireFFmpeg(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	argv := append([]string{"-v", "error", "-y", "-f", "lavfi", "-i", filter}, extra...)
	argv = append(argv, "-c:v", "libx264", "-pix_fmt", "yuv420p", path)
	if output, err := exec.Command(ffmpeg, argv...).CombinedOutput(); err != nil {
		t.Fatalf("生成素材失败：%v\n%s", err, output)
	}
}

// makeSceneDir 建一个可被 scene.Load 读取的镜头目录。
func makeSceneDir(t *testing.T, root, id string, duration float64, filter string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "transcript.srt"), []byte("1\n00:00:00,000 --> 00:00:01,000\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `{"id":"` + id + `","duration_seconds":` + trimFloat(duration) +
		`,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`
	if err := os.WriteFile(filepath.Join(dir, "scene.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	makeClip(t, filepath.Join(dir, "out.mp4"), filter)
	return dir
}

func trimFloat(value float64) string {
	text := strings.TrimRight(strings.TrimRight(
		strings.TrimSpace(strings.Trim(jsonNumber(value), `"`)), "0"), ".")
	if text == "" {
		return "0"
	}
	return text
}

func jsonNumber(value float64) string {
	body, _ := json.Marshal(value)
	return string(body)
}

func TestValidateVideoCommandReportsAllMismatches(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp4")
	makeClip(t, path, "testsrc=size=320x240:rate=25:duration=1")

	var out, errOut bytes.Buffer
	reportPath := filepath.Join(dir, "report.json")
	code := Execute([]string{"validate", "video", path,
		"--canvas", "1080x1440", "--fps", "30", "--silent",
		"--report-json", reportPath}, &out, &errOut)
	if code != 1 {
		t.Fatalf("规格不符应当退出 1，实际 %d：%s", code, errOut.String())
	}
	message := errOut.String()
	for _, want := range []string{"分辨率不符", "帧率不符"} {
		if !strings.Contains(message, want) {
			t.Errorf("输出里缺少 %q：%s", want, message)
		}
	}
	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("--report-json 没有落盘：%v", err)
	}
	var report struct {
		OK       bool     `json:"ok"`
		Problems []string `json:"problems"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("报告不是合法 JSON：%v", err)
	}
	if report.OK || len(report.Problems) < 2 {
		t.Errorf("报告应当 ok=false 且列出全部不符项：%+v", report)
	}
}

func TestValidateVideoCommandPasses(t *testing.T) {
	requireFFmpeg(t)
	path := filepath.Join(t.TempDir(), "clip.mp4")
	makeClip(t, path, "testsrc=size=320x240:rate=30:duration=1")
	var out, errOut bytes.Buffer
	code := Execute([]string{"validate", "video", path,
		"--canvas", "320x240", "--fps", "30", "--silent", "--decode"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("合规文件应当退出 0，实际 %d：%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "校验通过") {
		t.Errorf("缺少通过结论：%s", out.String())
	}
}

func TestValidateVideoRejectsContradictoryAudioFlags(t *testing.T) {
	requireFFmpeg(t)
	path := filepath.Join(t.TempDir(), "clip.mp4")
	makeClip(t, path, "testsrc=size=320x240:rate=30:duration=1")
	var out, errOut bytes.Buffer
	if code := Execute([]string{"validate", "video", path, "--canvas", "320x240",
		"--silent", "--audio"}, &out, &errOut); code != 1 {
		t.Errorf("--silent 与 --audio 同时给出应当失败，实际退出 %d", code)
	}
	if !strings.Contains(errOut.String(), "互斥") {
		t.Errorf("错误信息应当说明两者互斥：%s", errOut.String())
	}
}

// TestValidateVideoWithoutProjectExplainsHowToProceed：脱离项目使用时
// frame.md 不存在，错误信息必须告诉调用方改用 --canvas，而不是只说文件读不到。
func TestValidateVideoWithoutProjectExplainsHowToProceed(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp4")
	makeClip(t, path, "testsrc=size=320x240:rate=30:duration=1")
	var out, errOut bytes.Buffer
	if code := Execute([]string{"validate", "video", path, "--project-root", dir}, &out, &errOut); code != 1 {
		t.Fatalf("没有 frame.md 应当失败，实际退出 %d", code)
	}
	if !strings.Contains(errOut.String(), "--canvas") {
		t.Errorf("错误信息应当指出可以改用 --canvas：%s", errOut.String())
	}
}

func TestSceneFramesExtractsAndFlagsBlankFrames(t *testing.T) {
	requireFFmpeg(t)
	root := t.TempDir()

	busy := makeSceneDir(t, root, "scene-001", 1, "testsrc=size=320x240:rate=30:duration=1")
	var out, errOut bytes.Buffer
	reportPath := filepath.Join(root, "qc.json")
	code := Execute([]string{"scene", "frames", busy, "--at", "0,50%,end",
		"--check-blank", "--report-json", reportPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("有内容的镜头不该被判空白，实际退出 %d：%s", code, errOut.String())
	}
	entries, err := os.ReadDir(filepath.Join(busy, "visual-qc"))
	if err != nil {
		t.Fatalf("默认输出目录不存在：%v", err)
	}
	if len(entries) != 3 {
		t.Errorf("应当抽出 3 张 PNG，实际 %d 张", len(entries))
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Errorf("报告没有落盘：%v", err)
	}

	blank := makeSceneDir(t, root, "scene-002", 1, "color=c=black:size=320x240:rate=30:duration=1")
	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"scene", "frames", blank, "--at", "0", "--check-blank"}, &out, &errOut); code != 1 {
		t.Fatalf("纯黑镜头应当退出 1，实际 %d", code)
	}
	if !strings.Contains(errOut.String(), "空白帧") {
		t.Errorf("错误信息应当点明空白帧：%s", errOut.String())
	}

	// 不带 --check-blank 时只记录不改变退出码。
	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"scene", "frames", blank, "--at", "0"}, &out, &errOut); code != 0 {
		t.Errorf("不带 --check-blank 时不该改变退出码，实际 %d", code)
	}
}

func TestConcatCommandBuildsMasterAndRefusesMismatchedScenes(t *testing.T) {
	requireFFmpeg(t)
	root := t.TempDir()
	scenes := filepath.Join(root, "scenes")
	makeSceneDir(t, scenes, "scene-001", 1, "testsrc=size=320x240:rate=30:duration=1")
	makeSceneDir(t, scenes, "scene-002", 1, "testsrc=size=320x240:rate=30:duration=1")
	master := filepath.Join(root, "production", "silent-master.mp4")

	var out, errOut bytes.Buffer
	code := Execute([]string{"concat", scenes, "--out", master,
		"--canvas", "320x240", "--fps", "30", "--dry-run"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("dry-run 应当成功，实际 %d：%s", code, errOut.String())
	}
	if _, err := os.Stat(master); err == nil {
		t.Error("dry-run 不该写出任何文件")
	}

	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"concat", scenes, "--out", master,
		"--canvas", "320x240", "--fps", "30"}, &out, &errOut); code != 0 {
		t.Fatalf("拼接应当成功，实际 %d：%s", code, errOut.String())
	}
	if _, err := os.Stat(master); err != nil {
		t.Fatalf("母版没有生成：%v", err)
	}

	// 画幅不一致的镜头必须阻断，且要求重渲染而不是缩放。
	makeSceneDir(t, scenes, "scene-003", 1, "testsrc=size=160x120:rate=30:duration=1")
	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"concat", scenes, "--out", master,
		"--canvas", "320x240", "--fps", "30"}, &out, &errOut); code != 1 {
		t.Fatalf("画幅不一致应当退出 1，实际 %d", code)
	}
	if !strings.Contains(errOut.String(), "重渲染") {
		t.Errorf("错误信息应当要求回镜头工程重渲染：%s", errOut.String())
	}
}

func TestConcatRequiresOutputPath(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Execute([]string{"concat", t.TempDir()}, &out, &errOut); code != 1 {
		t.Errorf("缺 --out 应当失败，实际退出 %d", code)
	}
	if !strings.Contains(errOut.String(), "--out") {
		t.Errorf("错误信息应当点名 --out：%s", errOut.String())
	}
}
