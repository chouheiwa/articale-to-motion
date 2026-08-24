package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
