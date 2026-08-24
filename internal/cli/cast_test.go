package cli

import (
	"bytes"
	"fmt"
	"os"
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
