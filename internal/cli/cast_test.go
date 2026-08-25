package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/dialogue"
)

// runCLI 在 root 目录下执行一次 am 命令。
//
// 本包已有的用例（见 cli_test.go 的 initProject/TestConfigGetUsesProjectRoot）
// 都是先 os.Chdir 到项目根，再直接调用 Execute——本包目前没有名为 runCLI 的
// 现成辅助函数，这里按同样的“chdir + Execute”方式补一个，专供 cast 命令的
// 测试使用：cast 命令都按当前工作目录解析 cast/、cast.yaml，与 scene 系列
// 命令的相对路径约定一致。
func runCLI(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	var out bytes.Buffer
	if code := Execute(args, &out, &out); code != 0 {
		return out.String(), fmt.Errorf("退出码 %d：%s", code, out.String())
	}
	return out.String(), nil
}

func TestCastNewThenValidate(t *testing.T) {
	root := t.TempDir()
	out, err := runCLI(t, root, "cast", "new", "heiwa")
	if err != nil {
		t.Fatalf("cast new 失败：%v（%s）", err, out)
	}
	dir := filepath.Join(root, "cast", "heiwa")
	for _, name := range []string{"character.yaml", "rig.svg", "dna.md", "character.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("骨架缺 %s：%v", name, err)
		}
	}
	if out, err := runCLI(t, root, "cast", "validate"); err != nil {
		t.Fatalf("新生成的骨架应当自洽：%v（%s）", err, out)
	}
}

func TestCastNewRejectsExistingNonEmptyDir(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err == nil {
		t.Fatal("期望拒绝覆盖已存在的角色包目录")
	}
}

func TestCastValidateReportsRigProblem(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	rig := filepath.Join(root, "cast", "heiwa", "rig.svg")
	body, err := os.ReadFile(rig)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(body), "</svg>", `<animate attributeName="opacity"/></svg>`, 1)
	if err := os.WriteFile(rig, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, root, "cast", "validate")
	if err == nil {
		t.Fatal("期望校验失败")
	}
	if !strings.Contains(out+err.Error(), "animate") {
		t.Errorf("输出 = %q，期望指出 animate", out)
	}
}

func TestCastAddRejectsInvalidPackAndLeavesNothing(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "character.yaml"), []byte("schema: cast/v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, root, "cast", "add", source); err == nil {
		t.Fatal("期望拒绝不合规的包")
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "cast")); len(entries) != 0 {
		t.Errorf("不合规的包不应落地，实际留下 %d 项", len(entries))
	}
}

func TestCastAddCopiesRegistersAndWritesJSON(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	// 把 new 生成的骨架当作“外部角色包”，改个 id 后用 add 引入到另一个项目。
	source := filepath.Join(root, "cast", "heiwa")
	body, err := os.ReadFile(filepath.Join(source, "character.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	renamed := strings.Replace(string(body), "id: heiwa", "id: xiaoming", 1)
	renamed = strings.Replace(renamed, "name: heiwa", "name: xiaoming", 1)
	if err := os.WriteFile(filepath.Join(source, "character.yaml"), []byte(renamed), 0o644); err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	out, err := runCLI(t, project, "cast", "add", source)
	if err != nil {
		t.Fatalf("cast add 失败：%v（%s）", err, out)
	}
	dest := filepath.Join(project, "cast", "xiaoming")
	for _, name := range []string{"character.yaml", "rig.svg", "dna.md", "character.json"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Errorf("拷贝后缺 %s：%v", name, err)
		}
	}
	roster, err := os.ReadFile(filepath.Join(project, "cast.yaml"))
	if err != nil {
		t.Fatalf("cast.yaml 应已生成：%v", err)
	}
	if !strings.Contains(string(roster), "cast/xiaoming") {
		t.Errorf("cast.yaml 应登记 cast/xiaoming，实际：%s", roster)
	}
	if !strings.Contains(string(roster), "ground_y") {
		t.Errorf("cast.yaml 应带默认 defaults，实际：%s", roster)
	}
}

func TestCastAddRejectsSymlinkInSource(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "cast", "heiwa")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "evil-link")); err != nil {
		t.Skipf("本机不支持创建符号链接：%v", err)
	}
	project := t.TempDir()
	if _, err := runCLI(t, project, "cast", "add", source); err == nil {
		t.Fatal("期望拒绝含符号链接的源目录")
	}
	if entries, _ := os.ReadDir(filepath.Join(project, "cast")); len(entries) != 0 {
		t.Errorf("拒绝符号链接后不应留下任何文件，实际留下 %d 项", len(entries))
	}
}

func TestCastValidateWithExplicitArgs(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	if out, err := runCLI(t, root, "cast", "validate", filepath.Join("cast", "heiwa")); err != nil {
		t.Fatalf("显式指定路径应校验通过：%v（%s）", err, out)
	}
}

func TestCastValidateWithoutRosterOrCastDirSucceeds(t *testing.T) {
	root := t.TempDir()
	if out, err := runCLI(t, root, "cast", "validate"); err != nil {
		t.Fatalf("没有任何角色包时应视为无事可做：%v（%s）", err, out)
	}
}

// TestCastValidateWarnsMissingVoiceWithExplicitProvider 覆盖遗留缺口③：
// am cast validate 此前完全不查音色，只有 am validate cast 查。骨架
// 生成的 voiceId 留空是设计如此（castSkeletonDNA 的待办项），所以这里
// 断言的是"通过但带提醒"而不是"失败"——真正的硬校验仍在 am validate cast，
// 这里只是让作者在做完角色包的第一时间就看到音色缺口。
//
// want 子串挑的是 VoiceFor 的错误文案里独有的"缺少 voiceId"，不是
// "通过"或包名——那两个任何一条校验路径的输出都可能出现，撑不住这个
// 断言要验证的行为（音色检查真的跑了）。
func TestCastValidateWarnsMissingVoiceWithExplicitProvider(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, root, "cast", "validate", "--provider", "minimax")
	if err != nil {
		t.Fatalf("音色缺口不应让命令失败：%v（%s）", err, out)
	}
	if !strings.Contains(out, "缺少 voiceId") {
		t.Errorf("输出应提醒缺少 voiceId，实际：%s", out)
	}
	if !strings.Contains(out, "am validate cast") {
		t.Errorf("提醒应指出发布前的硬校验命令，实际：%s", out)
	}
}

// TestCastValidateSilentOnVoiceWhenProviderUnresolvable 覆盖③的另一半：
// 刚 cast new 完、还没配置 ORCHESTRATOR/RENDERER 的半成品项目（本用例的
// t.TempDir() 就是这种状态）不应该因为解析不出完整项目配置而报错或报出
// 无意义的提醒——音色检查是附带诊断，config 解析不出来就该安静跳过。
func TestCastValidateSilentOnVoiceWhenProviderUnresolvable(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, root, "cast", "validate")
	if err != nil {
		t.Fatalf("解析不出 provider 时不应报错：%v（%s）", err, out)
	}
	if strings.Contains(out, "voiceId") {
		t.Errorf("provider 解析不到时不应提及音色，实际：%s", out)
	}
}

// TestCastValidateDefaultProviderResolvesFromConfig 验证不传 --provider
// 时按 am config get TTS_PROVIDER 的规则解析（默认 minimax），而不是
// 只有显式传参才生效——防止实现只接线 --provider 忘了接默认路径。
func TestCastValidateDefaultProviderResolvesFromConfig(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "article-to-motion.conf"),
		[]byte("ORCHESTRATOR=codex\nRENDERER=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, root, "cast", "validate")
	if err != nil {
		t.Fatalf("音色缺口不应让命令失败：%v（%s）", err, out)
	}
	if !strings.Contains(out, "缺少 voiceId") {
		t.Errorf("默认 provider（minimax）也应触发音色提醒，实际：%s", out)
	}
}

func TestCastPreviewGeneratesContactSheet(t *testing.T) {
	root := t.TempDir()
	if _, err := runCLI(t, root, "cast", "new", "heiwa"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, root, "cast", "preview", filepath.Join("cast", "heiwa"))
	if err != nil {
		if strings.Contains(err.Error(), "rsvg-convert") || strings.Contains(err.Error(), "magick") {
			t.Skipf("本机缺少 rsvg-convert/magick：%v", err)
		}
		t.Fatalf("cast preview 失败：%v（%s）", err, out)
	}
	if !strings.Contains(out, "cast-preview") {
		t.Errorf("输出应指出 contact sheet 路径，实际：%s", out)
	}
}

// writeMultiViewCastPack 在 dir 下手写一个带 views/turns 的三视图角色包：
// 默认视图 front，加 three-quarter、side 两个。三个 rig 文件内容相同，
// 本测试只关心 CLI 输出是否逐视图报了路径，不关心 pivot 取值（那是
// internal/cast 单测的职责）。
func writeMultiViewCastPack(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yamlBody := `schema: cast/v1
id: heiwa
name: 黑娃
summary: 多视图预览的 CLI 测试夹具
voice:
  minimax: { voiceId: "v-1", speed: 0.98, pitch: 0 }
rig:
  file: rig.svg
  viewBox: [0, 0, 400, 520]
  baselineY: 512
  joints:
    head:     { pivot: [200, 168], rotate: [-18, 18] }
    frontLeg: { pivot: [176, 330], rotate: [-40, 55] }
scale:
  heightRatio: [0.22, 0.32]
poses:
  idle: {}
  pointing: { frontLeg: 48, head: -6 }
views:
  three-quarter:
    file: rig-tq.svg
    viewBox: [0, 0, 400, 520]
    baselineY: 512
    joints:
      head:     { pivot: [186, 170], rotate: [-22, 22] }
      frontLeg: { pivot: [168, 332], rotate: [-40, 55] }
  side:
    file: rig-side.svg
    viewBox: [0, 0, 400, 520]
    baselineY: 512
    joints:
      head:     { pivot: [172, 174], rotate: [-26, 26] }
      frontLeg: { pivot: [160, 334], rotate: [-45, 60] }
turns:
  front->side: { via: [three-quarter], durationMs: 200 }
`
	if err := os.WriteFile(filepath.Join(dir, "character.yaml"), []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 520">
  <g id="j-head"><circle cx="200" cy="168" r="60" fill="#fff" stroke="#000" stroke-width="6"/></g>
  <g id="j-frontLeg"><path d="M176 330 L176 420" stroke="#000" stroke-width="6"/></g>
</svg>`
	for _, name := range []string{"rig.svg", "rig-tq.svg", "rig-side.svg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(svg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCastPreviewPrintsEveryView 是修复轮 1 补的用例：am cast preview 的
// 逐视图打印是多视图功能唯一的用户可见面，internal/cast 的单测再充分也
// 盖不住"CLI 把三个视图的路径挤成一行""只打印了第一个视图""打印了但视图名
// 丢了"这几种 CLI 层特有的坏法——它们都不影响 cast.Preview 本身的返回值，
// 只会在 RunE 里把 map 拼接成字符串这一步出岔子。
//
// 断言故意做成"每个视图各占一行、且这一行同时含视图名与它自己的 contact
// sheet 文件名"：把 newCastPreviewCommand 里的 RunE 改成只打印
// viewNames[0] 那一个，或者把三行拼成一行输出，这个测试都必须挂掉。
func TestCastPreviewPrintsEveryView(t *testing.T) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Skip("需要 rsvg-convert，本机未安装")
	}
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("需要 ImageMagick 的 magick 命令，本机未安装")
	}

	root := t.TempDir()
	writeMultiViewCastPack(t, filepath.Join(root, "cast", "heiwa"))

	out, err := runCLI(t, root, "cast", "preview", filepath.Join("cast", "heiwa"))
	if err != nil {
		t.Fatalf("cast preview 失败：%v（%s）", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("输出应恰好 3 行，每个视图一行，实际 %d 行：%q", len(lines), out)
	}

	for _, view := range []string{"front", "three-quarter", "side"} {
		wantSheet := fmt.Sprintf("heiwa-%s-contact-sheet.png", view)
		found := false
		for _, line := range lines {
			if strings.Contains(line, "视图 "+view+"）") && strings.Contains(line, wantSheet) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("输出缺视图 %s 对应的一行（应同时含视图名与 %q）：%s", view, wantSheet, out)
		}
	}
}

// TestDialogueAssembleRequiresPlan 确认 am dialogue assemble 在缺
// plan.json（默认路径 production/audio/plan.json）时失败，且失败信息里
// 能看到 "plan" 字样——直接原样透出 dialogue.Assemble 的错误，不额外包装。
func TestDialogueAssembleRequiresPlan(t *testing.T) {
	root := t.TempDir()
	out, err := runCLI(t, root, "dialogue", "assemble")
	if err == nil {
		t.Fatal("缺 plan.json 时应失败")
	}
	if !strings.Contains(out+err.Error(), "plan") {
		t.Errorf("输出 = %q，期望提到 plan", out)
	}
}

// dialogueTone 生成一段指定秒数的测试音频。与 internal/dialogue 包测试里的
// 同名辅助函数（audio_test.go）逻辑一致，但那个函数未导出、且在 _test.go
// 文件里，本包无法复用，只能照抄一份最小实现。
func dialogueTone(t *testing.T, path string, seconds float64) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%.3f", seconds),
		"-ar", "22050", "-ac", "2", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成测试音频失败：%v（%s）", err, out)
	}
}

// TestDialogueAssembleEndToEnd 走一遍 am dialogue assemble 的完整链路：
// 两段真实音频 + plan.json，装配成功后校验命令输出提到三个产物路径与总
// 时长，且产物文件确实落地。需要真实 ffmpeg/ffprobe，本机没装就跳过，
// 与仓库里其他依赖外部命令的用例（如 TestCastPreviewPrintsEveryView）同一
// 惯例。
func TestDialogueAssembleEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("需要 ffmpeg，本机未安装")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("需要 ffprobe，本机未安装")
	}

	root := t.TempDir()
	audioDir := filepath.Join(root, "production", "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dialogueTone(t, filepath.Join(audioDir, "seg-001-heiwa.wav"), 1.0)
	dialogueTone(t, filepath.Join(audioDir, "seg-002-zhaocai.wav"), 0.5)

	plan := dialogue.Plan{
		Schema: dialogue.SchemaVersion,
		Segments: []dialogue.PlanSegment{
			{Index: 1, Speaker: "heiwa", VoiceID: "v-1", Audio: "production/audio/seg-001-heiwa.wav",
				GapAfterMs: 200, Lines: []dialogue.PlanLine{{Text: "第一句", StartSeconds: 0, EndSeconds: 1.0}}},
			{Index: 2, Speaker: "zhaocai", VoiceID: "v-2", Audio: "production/audio/seg-002-zhaocai.wav",
				GapAfterMs: 0, Lines: []dialogue.PlanLine{{Text: "等一下", StartSeconds: 0, EndSeconds: 0.5}}},
		},
	}
	body, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(audioDir, "plan.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, root, "dialogue", "assemble")
	if err != nil {
		t.Fatalf("dialogue assemble 失败：%v（%s）", err, out)
	}

	for _, rel := range []string{"production/audio/voice.wav", "transcription-production.srt", "production/dialogue.json"} {
		if _, statErr := os.Stat(filepath.Join(root, rel)); statErr != nil {
			t.Errorf("缺产物 %s：%v", rel, statErr)
		}
		if !strings.Contains(out, filepath.FromSlash(rel)) {
			t.Errorf("输出未提到产物路径 %s：%q", rel, out)
		}
	}
	if !strings.Contains(out, "1.7") && !strings.Contains(out, "1.700") {
		t.Errorf("输出应提到总时长（约 1.7 秒）：%q", out)
	}
}
