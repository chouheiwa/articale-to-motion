//go:build darwin || linux

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/dialogue"
)

// fakeRenderer 装一个假渲染工具：它不真的渲染，只把预先准备好的 mp4 拷到
// scene.json 声明的产物路径，并输出一条阶段消息。
//
// 用假渲染器是唯一能在 CI 里跑通整条链路的办法——真实渲染要一个已登录的
// AI CLI 加一台无头 Chrome。被替换掉的只是"画面从哪来"，am 自己的那部分
// （契约、调度、校验、拼接）全部是真的在跑。
func fakeRenderer(t *testing.T, binDir, source string) {
	t.Helper()
	script := "#!/bin/sh\nset -eu\n" +
		"printf '%s\\n' '{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\"," +
		"\"text\":\"[[USER_MESSAGE]]视频已渲染完成\"}}'\n" +
		// 用绝对路径：安全模式下子进程的 PATH 是受控白名单，/bin 不在里面。
		"/bin/cp '" + source + "' out.mp4\n"
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestEndToEndRenderVerifyConcatValidate 把 am 自己负责的四段串起来跑一遍：
// 调度 → 产物校验 → 抽帧 → 拼接 → 成片验收。
//
// 单元测试各自覆盖了这些环节，但它们之间的接缝没有任何测试守着：
// 镜头产物的规格能不能被拼接接受、拼出的母版能不能通过成片验收、
// 覆盖校验读的字幕跨度和镜头声明是不是同一套单位。
func TestEndToEndRenderVerifyConcatValidate(t *testing.T) {
	requireFFmpeg(t)

	project := t.TempDir()
	binDir := t.TempDir()

	// 一段合规素材，供假渲染器拷贝：320x240、30fps、1 秒、无音轨。
	source := filepath.Join(t.TempDir(), "source.mp4")
	makeClip(t, source, "testsrc=size=320x240:rate=30:duration=1")
	fakeRenderer(t, binDir, source)

	// 项目级视觉规范，让画幅从 frame.md 反查而不是靠命令行传。
	if err := os.WriteFile(filepath.Join(project, "frame.md"),
		[]byte("---\ncanvas:\n  width_px: 320\n  height_px: 240\n  fps: 30\n---\n视觉规范正文\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "article-to-motion.conf"),
		[]byte("ORCHESTRATOR=codex\nRENDERER=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 两条字幕，跨度 2 秒，正好等于两个 1 秒镜头之和。
	srtPath := filepath.Join(project, "transcription.srt")
	if err := os.WriteFile(srtPath, []byte(
		"1\n00:00:00,000 --> 00:00:01,000\n第一句\n\n2\n00:00:01,000 --> 00:00:02,000\n第二句\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scenes := filepath.Join(project, "scenes")
	for _, id := range []string{"scene-001", "scene-002"} {
		dir := filepath.Join(scenes, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"transcription.srt", "frame.md"} {
			body, err := os.ReadFile(filepath.Join(project, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		body := `{"id":"` + id + `","duration_seconds":1.0,"output":"out.mp4",` +
			`"transcript":"transcription.srt","text":"内容","style_guide":"frame.md"}`
		if err := os.WriteFile(filepath.Join(dir, "scene.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(project)
	ffmpegDir := filepath.Dir(mustLookPath(t, "ffmpeg"))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+ffmpegDir)
	t.Setenv("HOME", t.TempDir())

	// 1. 调度：带覆盖校验，两镜合计 2 秒对上字幕跨度 2 秒。
	var out, errOut bytes.Buffer
	reportPath := filepath.Join(project, "run-report.json")
	code := Execute([]string{"scene", "run-all", "scenes/",
		"--srt", "transcription.srt", "--strict-coverage",
		"--jobs", "2", "--retries", "0", "--report-json", reportPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("调度应当成功，实际退出 %d\nstdout: %s\nstderr: %s", code, out.String(), errOut.String())
	}
	var report struct {
		Counts map[string]int `json:"counts"`
	}
	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("执行报告没有落盘：%v", err)
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatal(err)
	}
	if report.Counts["succeeded"] != 2 {
		t.Fatalf("应当两镜都成功：%s", body)
	}

	// 2. 抽帧：产物有内容，不该被判空白。
	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"scene", "frames", "scenes/scene-001",
		"--at", "0,end", "--check-blank"}, &out, &errOut); code != 0 {
		t.Fatalf("抽帧应当通过，实际退出 %d：%s", code, errOut.String())
	}

	// 3. 拼接：画幅从项目 frame.md 反查，不显式传 --canvas。
	master := filepath.Join(project, "production", "silent-master.mp4")
	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"concat", "scenes/", "--out", master}, &out, &errOut); code != 0 {
		t.Fatalf("拼接应当成功，实际退出 %d\nstdout: %s\nstderr: %s", code, out.String(), errOut.String())
	}

	// 4. 成片验收：母版应当是 60 帧、无音轨、可完整解码。
	out.Reset()
	errOut.Reset()
	if code := Execute([]string{"validate", "video", master,
		"--silent", "--decode", "--expect-frames", "60"}, &out, &errOut); code != 0 {
		t.Fatalf("成片验收应当通过，实际退出 %d：%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "校验通过") {
		t.Errorf("缺少通过结论：%s", out.String())
	}
}

// TestEndToEndCoverageMismatchStopsBeforeRendering 证明覆盖校验真的在渲染前拦住。
//
// 这是覆盖校验唯一的价值所在：拦晚了每一镜都已经烧掉一次完整的 AI CLI 调用。
// 用一个会写入标记文件的假渲染器来证明它一次都没被调起来。
func TestEndToEndCoverageMismatchStopsBeforeRendering(t *testing.T) {
	requireFFmpeg(t)
	project := t.TempDir()
	binDir := t.TempDir()

	marker := filepath.Join(project, "renderer-was-called")
	script := "#!/bin/sh\nset -eu\n: > '" + marker + "'\n"
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "article-to-motion.conf"),
		[]byte("ORCHESTRATOR=codex\nRENDERER=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 字幕跨度 10 秒，镜头只声明了 1 秒——少了 9 秒。
	if err := os.WriteFile(filepath.Join(project, "transcription.srt"),
		[]byte("1\n00:00:00,000 --> 00:00:10,000\n很长的一句\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(project, "scenes", "scene-001")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "transcription.srt"), []byte("1\n00:00:00,000 --> 00:00:10,000\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scene.json"), []byte(
		`{"id":"scene-001","duration_seconds":1.0,"output":"out.mp4","transcript":"transcription.srt","text":"内容"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(project)
	t.Setenv("PATH", binDir)
	t.Setenv("HOME", t.TempDir())

	var out, errOut bytes.Buffer
	code := Execute([]string{"scene", "run-all", "scenes/",
		"--srt", "transcription.srt", "--strict-coverage"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("覆盖不足应当退出 1，实际 %d：%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "字幕跨度") {
		t.Errorf("错误信息应当点明字幕跨度：%s", errOut.String())
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("渲染器被调起来了：覆盖校验必须在渲染之前拦住")
	}

	// 不加 --strict-coverage 时只警告，仍然会去渲染。
	out.Reset()
	errOut.Reset()
	// --retries 0：这里只关心渲染器有没有被调起来，不必真的等两轮退避（10 秒 + 30 秒）。
	Execute([]string{"scene", "run-all", "scenes/", "--srt", "transcription.srt", "--retries", "0"}, &out, &errOut)
	if !strings.Contains(errOut.String(), "警告") {
		t.Errorf("默认应当打印警告：%s", errOut.String())
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("不加 --strict-coverage 时应当照常渲染")
	}
}

// TestEndToEndCastNarrationValidate 把多角色叙事项目的整条链路串起来跑一遍：
// 初始化 → 建角色 → 校验角色包 → 装配对白 → 声明镜头节拍 → 项目级一致性校验。
//
// 单元测试各自覆盖了每一环（cast.Load、dialogue.Assemble、scene.Load、
// CastProblems 本身），但没有任何测试证明这些产物真的能首尾相接：cast new
// 写出的骨架能不能被 dialogue assemble 的说话人字段对上、装配出的
// dialogue.json 时间戳能不能被镜头 cast.beats 精确覆盖、am validate cast
// 读到的是不是这一整条链路的真实产物而不是手造的 fixture。
func TestEndToEndCastNarrationValidate(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("需要 ffmpeg，本机未安装")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("需要 ffprobe，本机未安装")
	}

	project := t.TempDir()

	// 1. 初始化多角色项目：必须传 --skip-hyperframes，否则会联网安装。
	if out, err := runCLI(t, project, "init", "--canvas", "vertical-3x4",
		"--narration", "cast", "--skip-hyperframes"); err != nil {
		t.Fatalf("init 失败：%v（%s）", err, out)
	}

	// 2. 生成角色骨架。
	if out, err := runCLI(t, project, "cast", "new", "heiwa"); err != nil {
		t.Fatalf("cast new 失败：%v（%s）", err, out)
	}

	// 手填 voiceId：cast new 留空的骨架过得了自洽校验，但过不了本任务新增的
	// 音色校验（VoiceFor 要求 voiceId 非空）。
	characterYAMLPath := filepath.Join(project, "cast", "heiwa", "character.yaml")
	body, err := os.ReadFile(characterYAMLPath)
	if err != nil {
		t.Fatal(err)
	}
	filled := strings.ReplaceAll(string(body), `voiceId: ""`, `voiceId: "v-heiwa"`)
	if filled == string(body) {
		t.Fatal("骨架里找不到待填的 voiceId 占位符")
	}
	if err := os.WriteFile(characterYAMLPath, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}

	// cast new 只落地角色包目录，不登记进项目班底——登记是 cast add 的职责。
	// 这里手工登记，模拟人工编辑 cast.yaml 把角色纳入班底的真实操作。
	rosterPath := filepath.Join(project, "cast.yaml")
	rosterBody, err := os.ReadFile(rosterPath)
	if err != nil {
		t.Fatal(err)
	}
	registered := strings.Replace(string(rosterBody), "packs: []", "packs: [cast/heiwa]", 1)
	if registered == string(rosterBody) {
		t.Fatalf("cast.yaml 里找不到待替换的空 packs：%s", rosterBody)
	}
	if err := os.WriteFile(rosterPath, []byte(registered), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. 校验角色包：改完 character.yaml 必须重新跑一次，否则 character.json
	// 仍是旧的（cast.go 的文档注释就是这么写的）。
	if out, err := runCLI(t, project, "cast", "validate"); err != nil {
		t.Fatalf("cast validate 失败：%v（%s）", err, out)
	}

	// 4. 造两段音频与 plan.json：都是 heiwa 说的，一段 1.0 秒、一段 0.5 秒，
	// 段间静音 200ms。
	audioDir := filepath.Join(project, "production", "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dialogueTone(t, filepath.Join(audioDir, "seg-001-heiwa.wav"), 1.0)
	dialogueTone(t, filepath.Join(audioDir, "seg-002-heiwa.wav"), 0.5)
	plan := dialogue.Plan{
		Schema: dialogue.SchemaVersion,
		Segments: []dialogue.PlanSegment{
			{Index: 1, Speaker: "heiwa", VoiceID: "v-heiwa", Audio: "production/audio/seg-001-heiwa.wav",
				GapAfterMs: 200, Lines: []dialogue.PlanLine{{Text: "第一句", StartSeconds: 0, EndSeconds: 1.0}}},
			{Index: 2, Speaker: "heiwa", VoiceID: "v-heiwa", Audio: "production/audio/seg-002-heiwa.wav",
				GapAfterMs: 0, Lines: []dialogue.PlanLine{{Text: "第二句", StartSeconds: 0, EndSeconds: 0.5}}},
		},
	}
	planBody, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(audioDir, "plan.json"), planBody, 0o644); err != nil {
		t.Fatal(err)
	}

	// 5. 装配对白：产出 production/dialogue.json，全局时间轴是
	// [0, 1.0] 和 [1.2, 1.7]（段间 200ms 静音）。
	if out, err := runCLI(t, project, "dialogue", "assemble"); err != nil {
		t.Fatalf("dialogue assemble 失败：%v（%s）", err, out)
	}

	// 6. 写一个带 cast 块、但不含 beats 的 scene.json：节拍不由 agent 手算，
	// 而是下一步交给 am dialogue beats 从 dialogue.json 切出来。pack_dir 指向
	// 镜头目录内自带的一份角色包拷贝——scene.Load 不允许 pack_dir 逃出镜头
	// 目录，项目根的 cast/heiwa 不能直接引用。
	sceneDir := filepath.Join(project, "scenes", "scene-001")
	if err := os.MkdirAll(sceneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sceneDir, "transcript.txt"), []byte("占位字幕"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(filepath.Join(project, "cast", "heiwa"), filepath.Join(sceneDir, "cast", "heiwa")); err != nil {
		t.Fatalf("拷贝角色包到镜头目录失败：%v", err)
	}
	sceneJSON := `{
  "id": "scene-001",
  "duration_seconds": 1.7,
  "output": "out.mp4",
  "transcript": "transcript.txt",
  "text": "黑娃说两句话",
  "cast": {
    "pack_dir": "cast",
    "ground_y": 0.78,
    "on_stage": [
      {"id": "heiwa", "x": 0.5, "pose": "idle", "facing": "right"}
    ]
  }
}
`
	if err := os.WriteFile(filepath.Join(sceneDir, "scene.json"), []byte(sceneJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// 7. 重算镜头节拍：把 dialogue.json 的全局时间切成镜头本地时间写回
	// scene.json。这一步之前 scene.json 里根本没有 beats，validate cast 必然
	// 报"没有被任何镜头的 cast.beats 覆盖"——先证明这一点，再证明跑完
	// 命令就通过了，否则下面那个"通过"说明不了是这条命令的功劳。
	if out, err := runCLI(t, project, "validate", "cast"); err == nil {
		t.Fatalf("还没重算节拍时 validate cast 不该通过：%s", out)
	} else if !strings.Contains(out, "没有被任何镜头的 cast.beats 覆盖") {
		t.Fatalf("期望报台词行未被覆盖，得到：%s", out)
	}

	if out, err := runCLI(t, project, "dialogue", "beats"); err != nil {
		t.Fatalf("dialogue beats 失败：%v（%s）", err, out)
	}

	var written struct {
		Cast struct {
			PackDir string `json:"pack_dir"`
			Beats   []struct {
				Speaker string  `json:"speaker"`
				Start   float64 `json:"start"`
				End     float64 `json:"end"`
			} `json:"beats"`
		} `json:"cast"`
	}
	sceneBody, err := os.ReadFile(filepath.Join(sceneDir, "scene.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(sceneBody, &written); err != nil {
		t.Fatalf("重算后的 scene.json 不是合法 JSON：%v（%s）", err, sceneBody)
	}
	if written.Cast.PackDir != "cast" {
		t.Errorf("pack_dir 不该被这条命令碰，得到 %q", written.Cast.PackDir)
	}
	if len(written.Cast.Beats) != 2 {
		t.Fatalf("两段配音应切出两拍，得到 %+v", written.Cast.Beats)
	}
	// 全局时间轴是 [0, 1.0] 与 [1.2, 1.7]（段间 200ms 静音），镜头起点为 0，
	// 所以本地时间与全局时间相同。实测时长与声明时长有几十毫秒出入，用 30ms
	// 容差比对。
	for i, want := range [][2]float64{{0, 1.0}, {1.2, 1.7}} {
		got := written.Cast.Beats[i]
		if got.Speaker != "heiwa" {
			t.Errorf("第 %d 拍的说话人应是 heiwa，得到 %q", i+1, got.Speaker)
		}
		if math.Abs(got.Start-want[0]) > 0.03 || math.Abs(got.End-want[1]) > 0.03 {
			t.Errorf("第 %d 拍应约为 [%v, %v]，得到 [%v, %v]", i+1, want[0], want[1], got.Start, got.End)
		}
	}

	// 8. 项目级一致性校验：班底、对白时间线、镜头节拍三者必须互相吻合。
	out, err := runCLI(t, project, "validate", "cast")
	if err != nil {
		t.Fatalf("validate cast 应当通过，实际失败：%v（%s）", err, out)
	}
	if !strings.Contains(out, "通过") {
		t.Errorf("缺少通过结论：%s", out)
	}

	// 9. 死锁回归：把 scene-001 的时长改小、另起一个 scene-002 吸收剩下的
	// 时间——scene-001 上一步算出来的节拍随即越界，scene.Load 拒绝加载它，
	// am validate cast 跟着失败。am dialogue beats 必须仍然跑得通（beats 是
	// 它的输出、不是它的输入），跑完之后校验重新通过。这条链路曾经是死的：
	// 唯一的出路是手改 JSON 把 beats 清成 []。
	total := written.Cast.Beats[1].End
	shrunk := strings.Replace(string(sceneBody), `"duration_seconds": 1.7`, `"duration_seconds": 1`, 1)
	if shrunk == string(sceneBody) {
		t.Fatalf("scene.json 里找不到待改小的 duration_seconds：%s", sceneBody)
	}
	if err := os.WriteFile(filepath.Join(sceneDir, "scene.json"), []byte(shrunk), 0o644); err != nil {
		t.Fatal(err)
	}
	secondDir := filepath.Join(project, "scenes", "scene-002")
	if err := os.MkdirAll(secondDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "transcript.txt"), []byte("占位字幕"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(filepath.Join(project, "cast", "heiwa"), filepath.Join(secondDir, "cast", "heiwa")); err != nil {
		t.Fatal(err)
	}
	secondJSON := fmt.Sprintf(`{
  "id": "scene-002",
  "duration_seconds": %v,
  "output": "out.mp4",
  "transcript": "transcript.txt",
  "text": "黑娃把话说完",
  "cast": {
    "pack_dir": "cast",
    "ground_y": 0.78,
    "on_stage": [
      {"id": "heiwa", "x": 0.5, "pose": "idle", "facing": "right"}
    ]
  }
}
`, total-1)
	if err := os.WriteFile(filepath.Join(secondDir, "scene.json"), []byte(secondJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := runCLI(t, project, "validate", "cast"); err == nil {
		t.Fatalf("旧节拍越界时 validate cast 不该通过：%s", out)
	} else if !strings.Contains(out, "超出镜头时长") {
		t.Fatalf("期望报节拍超出镜头时长，得到：%s", out)
	}

	if out, err := runCLI(t, project, "dialogue", "beats"); err != nil {
		t.Fatalf("越界的旧节拍不该挡住重算：%v（%s）", err, out)
	}

	if out, err := runCLI(t, project, "validate", "cast"); err != nil {
		t.Fatalf("重算之后 validate cast 应当通过：%v（%s）", err, out)
	}
}

func mustLookPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("本机没有 %s", name)
	}
	return path
}

// TestConcurrentScenesDoNotRaceOnSharedOutput 钉住一个被 e2e 测试抓出来的真 bug。
//
// RunAll 并发跑 N 个镜头，每个 scene.Run 都往同一个 writer 写阶段消息。
// 过去这里没有任何同步：写 os.Stdout 时表现为几镜的进度输出互相交错，
// 写任何带缓冲的 writer 则是实打实的数据竞争。
//
// 这条测试只有在 -race 下才有判别力，CI 的 test job 正是用 -race 跑的。
func TestConcurrentScenesDoNotRaceOnSharedOutput(t *testing.T) {
	requireFFmpeg(t)
	project := t.TempDir()
	binDir := t.TempDir()

	source := filepath.Join(t.TempDir(), "source.mp4")
	makeClip(t, source, "testsrc=size=320x240:rate=30:duration=1")
	fakeRenderer(t, binDir, source)

	if err := os.WriteFile(filepath.Join(project, "frame.md"),
		[]byte("---\ncanvas:\n  width_px: 320\n  height_px: 240\n  fps: 30\n---\n正文\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "article-to-motion.conf"),
		[]byte("ORCHESTRATOR=codex\nRENDERER=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scenes := filepath.Join(project, "scenes")
	// 镜头数明显多于并发数，确保 worker 之间真的会同时写。
	for _, id := range []string{"scene-001", "scene-002", "scene-003", "scene-004", "scene-005", "scene-006"} {
		dir := filepath.Join(scenes, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "transcription.srt"), []byte("1\n00:00:00,000 --> 00:00:01,000\nx\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// 必须带 style_guide：不声明就按默认画幅 1080x1440 校验，
		// 而素材是 320x240——那会被产物校验正确地拦下，测不到并发写入。
		guide, err := os.ReadFile(filepath.Join(project, "frame.md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "frame.md"), guide, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "scene.json"), []byte(
			`{"id":"`+id+`","duration_seconds":1.0,"output":"out.mp4",`+
				`"transcript":"transcription.srt","text":"内容","style_guide":"frame.md"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(project)
	ffmpegDir := filepath.Dir(mustLookPath(t, "ffmpeg"))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+ffmpegDir)
	t.Setenv("HOME", t.TempDir())

	// bytes.Buffer 不是并发安全的：没有同步的话 -race 会在这里报 DATA RACE。
	var out, errOut bytes.Buffer
	code := Execute([]string{"scene", "run-all", "scenes/", "--jobs", "6", "--retries", "0"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("并发渲染应当成功，实际退出 %d：%s", code, errOut.String())
	}
	// 每镜一条阶段消息，一条不能丢也不能被切开。
	if got := strings.Count(out.String(), "视频已渲染完成"); got != 6 {
		t.Errorf("阶段消息 %d 条，期望 6 条——少了说明输出被覆盖或截断：\n%s", got, out.String())
	}
}
