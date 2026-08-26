package hyperframes

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, name string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if files == nil {
		files = map[string]string{}
	}
	if _, ok := files[manifestFile]; !ok {
		files[manifestFile] = "# " + name + "\n"
	}
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCopySkillsMovesWholeTree(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	writeSkill(t, source, "hyperframes-animation", map[string]string{
		"rules-index.md":      "索引",
		"rules/typography.md": "规则",
		"scripts/run.sh":      "#!/bin/sh\n",
	})
	writeSkill(t, source, "hyperframes-core", nil)

	result, err := copySkills(source, dest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installed) != 2 {
		t.Fatalf("应当装入 2 个技能：%v", result.Installed)
	}
	for _, rel := range []string{
		"hyperframes-animation/SKILL.md",
		"hyperframes-animation/rules-index.md",
		"hyperframes-animation/rules/typography.md",
		"hyperframes-animation/scripts/run.sh",
		"hyperframes-core/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("缺少 %s：%v", rel, err)
		}
	}
}

// TestCopySkillsPreservesExecutableBit：技能自带的脚本丢了执行位就跑不起来，
// 而失败信息会是 permission denied，与「技能没装好」毫无关联，极难排查。
func TestCopySkillsPreservesExecutableBit(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	writeSkill(t, source, "hyperframes-cli", nil)
	script := filepath.Join(source, "hyperframes-cli", "scripts", "render.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := copySkills(source, dest, nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dest, "hyperframes-cli", "scripts", "render.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("执行位丢失：%v", info.Mode())
	}
}

// TestCopySkillsProtectsBuiltinSkills：随二进制下发的技能是 fork 过的，
// 被上游同名技能覆盖等于静默丢掉本仓库的改动。
func TestCopySkillsProtectsBuiltinSkills(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	writeSkill(t, source, "hyperframes-animation", nil)
	writeSkill(t, source, "text-to-lottie", map[string]string{manifestFile: "上游版本\n"})

	writeSkill(t, dest, "text-to-lottie", map[string]string{manifestFile: "本仓库 fork 版本\n"})

	result, err := copySkills(source, dest, map[string]bool{"text-to-lottie": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Protected) != 1 || result.Protected[0] != "text-to-lottie" {
		t.Errorf("受保护技能没有被记录：%+v", result)
	}
	body, err := os.ReadFile(filepath.Join(dest, "text-to-lottie", manifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "fork") {
		t.Errorf("内置技能被上游覆盖了：%s", body)
	}
}

// TestCopySkillsReplacesRatherThanMerges：同名技能整体替换。
// 残留上一版的文件会让技能树处于两个版本混合的状态，而那正是固定版本要消除的。
func TestCopySkillsReplacesRatherThanMerges(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	writeSkill(t, source, "hyperframes-animation", map[string]string{"rules/new.md": "新版"})
	writeSkill(t, dest, "hyperframes-animation", map[string]string{"rules/removed-upstream.md": "旧版残留"})

	if _, err := copySkills(source, dest, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "hyperframes-animation", "rules", "removed-upstream.md")); err == nil {
		t.Error("上一版已被上游删除的文件仍然残留，技能树处于混合版本状态")
	}
	if _, err := os.Stat(filepath.Join(dest, "hyperframes-animation", "rules", "new.md")); err != nil {
		t.Errorf("新版文件没有写入：%v", err)
	}
}

func TestCopySkillsIgnoresNonSkillDirectories(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	writeSkill(t, source, "hyperframes-core", nil)
	// 没有 SKILL.md，不是技能目录。
	if err := os.MkdirAll(filepath.Join(source, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := copySkills(source, dest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installed) != 1 || result.Installed[0] != "hyperframes-core" {
		t.Errorf("非技能条目被当成技能：%v", result.Installed)
	}
}

func TestCopySkillsFailsWhenNothingInstalled(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := copySkills(source, dest, nil); err == nil {
		t.Error("产物里没有技能时应当报错，而不是静默成功")
	}
}

// TestCopyTreeRejectsSymlinks：上游在 Unix 上用符号链接把技能镜像到其他 agent
// 目录，链接指回它自己的 home store。搬进项目后临时 HOME 已删除，
// 那些链接会成为静默失效的空技能。
func TestCopyTreeRejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不建符号链接")
	}
	source, dest := t.TempDir(), t.TempDir()
	writeSkill(t, source, "linked", nil)
	if err := os.Symlink("/somewhere/else", filepath.Join(source, "linked", "ref")); err != nil {
		t.Fatal(err)
	}
	if _, err := copySkills(source, dest, nil); err == nil {
		t.Error("含符号链接的技能应当报错，而不是搬进去一个失效链接")
	}
}

// TestInstallerEnvIsolatesHomeButKeepsNpmReachable 锁住这次改动的核心取舍。
func TestInstallerEnvIsolatesHomeButKeepsNpmReachable(t *testing.T) {
	env := map[string]string{"HOME": "/Users/someone", "PATH": "/usr/bin", "HTTPS_PROXY": "http://proxy:8080"}
	got := installerEnv("/tmp/am-hyperframes-123", "/opt/node/bin", env)

	if got["HOME"] != "/tmp/am-hyperframes-123" {
		t.Errorf("HOME 没有被隔离：%s", got["HOME"])
	}
	// 不接缓存的话每个项目都要重新下载整包，几百 MB 的临时占用还拿不到复用。
	if got["npm_config_cache"] != "/Users/someone/.npm" {
		t.Errorf("npm 缓存没有接回真实位置：%s", got["npm_config_cache"])
	}
	// 不接 .npmrc 的话私有 registry、代理和鉴权全部丢失，公司内网直接装不上。
	if got["npm_config_userconfig"] != "/Users/someone/.npmrc" {
		t.Errorf("npm 用户配置没有接回真实位置：%s", got["npm_config_userconfig"])
	}
	if got["HTTPS_PROXY"] != "http://proxy:8080" {
		t.Error("其余环境变量应当原样沿用")
	}
	if !strings.HasPrefix(got["PATH"], "/opt/node/bin") {
		t.Errorf("真实 node 目录应当在 PATH 首位：%s", got["PATH"])
	}
}

// TestInstallerEnvRespectsExplicitNpmSettings：用户显式配过就不要覆盖。
func TestInstallerEnvRespectsExplicitNpmSettings(t *testing.T) {
	env := map[string]string{
		"HOME":                  "/Users/someone",
		"npm_config_cache":      "/mnt/shared/npm-cache",
		"npm_config_userconfig": "/etc/npmrc",
	}
	got := installerEnv("/tmp/x", "", env)
	if got["npm_config_cache"] != "/mnt/shared/npm-cache" {
		t.Errorf("覆盖了用户显式设置的缓存：%s", got["npm_config_cache"])
	}
	if got["npm_config_userconfig"] != "/etc/npmrc" {
		t.Errorf("覆盖了用户显式设置的 npmrc：%s", got["npm_config_userconfig"])
	}
}

// --- 用假 node/npx 走完整条 Install 路径 ---

func fakeToolchain(t *testing.T, skills []string) (binDir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("测试用 sh 脚本模拟工具链")
	}
	binDir = t.TempDir()
	// 假 node：只回答 process.execPath，指向自己所在目录。
	node := filepath.Join(binDir, "node")
	if err := os.WriteFile(node, []byte("#!/bin/sh\nprintf '%s' '"+node+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 假 npx：模拟上游安装器，往 $HOME/.agents/skills 写技能。
	var body strings.Builder
	// 用绝对路径：安装器跑在受控 PATH 下，/bin 不在里面。
	body.WriteString("#!/bin/sh\nset -eu\n/bin/mkdir -p \"$HOME/.agents/skills\"\n")
	for _, name := range skills {
		fmt.Fprintf(&body, "/bin/mkdir -p \"$HOME/.agents/skills/%s\"\n", name)
		fmt.Fprintf(&body, "printf '# %s\\n' > \"$HOME/.agents/skills/%s/SKILL.md\"\n", name, name)
	}
	body.WriteString("printf 'installed\\n'\n")
	if err := os.WriteFile(filepath.Join(binDir, "npx"), []byte(body.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir
}

func TestInstallWritesIntoProjectAndNotHome(t *testing.T) {
	binDir := fakeToolchain(t, []string{"hyperframes-animation", "hyperframes-core"})
	project := t.TempDir()
	home := t.TempDir()

	var out bytes.Buffer
	result, err := Install(context.Background(), Options{
		ProjectDir: project,
		Version:    "0.8.1",
		Env:        map[string]string{"PATH": binDir, "HOME": home},
		Output:     &out,
	})
	if err != nil {
		t.Fatalf("安装失败：%v\n%s", err, out.String())
	}
	if len(result.Installed) != 2 {
		t.Errorf("应当装入 2 个技能：%v", result.Installed)
	}
	for _, name := range result.Installed {
		if _, err := os.Stat(filepath.Join(project, SkillsSubdir, name, manifestFile)); err != nil {
			t.Errorf("技能 %s 没有落进项目：%v", name, err)
		}
	}
	// 核心保证：用户 HOME 一个字节都没动。
	if entries, err := os.ReadDir(home); err == nil && len(entries) != 0 {
		t.Errorf("用户 HOME 被写入了：%v", entries)
	}
}

func TestInstallLeavesNoTempDirectoryBehind(t *testing.T) {
	binDir := fakeToolchain(t, []string{"hyperframes-core"})
	before, _ := filepath.Glob(filepath.Join(os.TempDir(), "am-hyperframes-*"))
	if _, err := Install(context.Background(), Options{
		ProjectDir: t.TempDir(),
		Version:    "0.8.1",
		Env:        map[string]string{"PATH": binDir, "HOME": t.TempDir()},
		Output:     &bytes.Buffer{},
	}); err != nil {
		t.Fatal(err)
	}
	after, _ := filepath.Glob(filepath.Join(os.TempDir(), "am-hyperframes-*"))
	if len(after) > len(before) {
		t.Errorf("临时目录没有清理：before=%d after=%d", len(before), len(after))
	}
}

func TestInstallRejectsMissingVersionAndToolchain(t *testing.T) {
	if _, err := Install(context.Background(), Options{ProjectDir: t.TempDir()}); err == nil {
		t.Error("没有版本号应当报错")
	}
	_, err := Install(context.Background(), Options{
		ProjectDir: t.TempDir(),
		Version:    "0.8.1",
		Env:        map[string]string{"PATH": t.TempDir()},
		Output:     &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("PATH 里没有 node 应当报错")
	}
	// 退出码 127 靠这个前缀判定。
	if !strings.Contains(err.Error(), "缺少必需工具") {
		t.Errorf("错误信息缺少退出码 127 的判定前缀：%v", err)
	}
}

func TestInstallReportsInstallerFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("测试用 sh 脚本")
	}
	binDir := t.TempDir()
	node := filepath.Join(binDir, "node")
	os.WriteFile(node, []byte("#!/bin/sh\nprintf '%s' '"+node+"'\n"), 0o755)
	os.WriteFile(filepath.Join(binDir, "npx"), []byte("#!/bin/sh\necho boom >&2\nexit 3\n"), 0o755)

	_, err := Install(context.Background(), Options{
		ProjectDir: t.TempDir(),
		Version:    "0.8.1",
		Env:        map[string]string{"PATH": binDir, "HOME": t.TempDir()},
		Output:     &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "0.8.1") {
		t.Errorf("安装器失败应当报错并点名版本：%v", err)
	}
}

// TestInstallRecordsUpstreamManifest 守住上游技能的来源记录。
//
// 为什么需要它：PinnedVersion 固定的只是 CLI 二进制。上游 `hyperframes skills`
// 直接 git clone 仓库默认分支取 skills/，没有任何指定 ref 的口子——实测
// 0.8.1 与 0.8.14 装出来的技能逐字节相同，都等于当天 main 的状态。也就是说
// 两次 am init 之间技能内容可能已经变了，而版本号一个字都没动。
//
// 这条记录把"这个项目到底装到了哪一版技能"从无从查证变成可比对的事实：
// 两个项目的 manifest 一比就知道差在哪个技能上。
func TestInstallRecordsUpstreamManifest(t *testing.T) {
	binDir := fakeToolchain(t, []string{"hyperframes-animation", "hyperframes-core"})
	project := t.TempDir()
	if _, err := Install(context.Background(), Options{
		ProjectDir: project,
		Version:    "9.9.9",
		Env:        map[string]string{"PATH": binDir, "HOME": t.TempDir()},
		Output:     &bytes.Buffer{},
	}); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(project)
	if err != nil {
		t.Fatalf("读取来源记录失败：%v", err)
	}
	if m.CLIVersion != "9.9.9" {
		t.Errorf("CLIVersion = %q，期望 9.9.9", m.CLIVersion)
	}
	for _, name := range []string{"hyperframes-animation", "hyperframes-core"} {
		if m.Skills[name] == "" {
			t.Errorf("来源记录缺少技能 %s：%v", name, m.Skills)
		}
	}
	if len(m.Skills) != 2 {
		t.Errorf("来源记录应当只覆盖上游技能，实际 %v", m.Skills)
	}
}

// TestUpstreamManifestHashTracksContent 是上一条的配套：记录里的哈希必须
// 真的跟着内容走。只断言"字段非空"挡不住把常量字符串写进去这种实现。
func TestUpstreamManifestHashTracksContent(t *testing.T) {
	binDir := fakeToolchain(t, []string{"hyperframes-core"})
	project := t.TempDir()
	opts := Options{
		ProjectDir: project,
		Version:    "9.9.9",
		Env:        map[string]string{"PATH": binDir, "HOME": t.TempDir()},
		Output:     &bytes.Buffer{},
	}
	if _, err := Install(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	first, err := ReadManifest(project)
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(project, SkillsSubdir, "hyperframes-core", manifestFile)
	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, append(body, []byte("\n上游改了一行\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := SkillDigest(filepath.Join(project, SkillsSubdir, "hyperframes-core"))
	if err != nil {
		t.Fatal(err)
	}
	if changed == first.Skills["hyperframes-core"] {
		t.Error("技能内容改了，摘要却没变——哈希没有覆盖文件内容")
	}
	if err := os.WriteFile(skill, body, 0o644); err != nil {
		t.Fatal(err)
	}
	restored, err := SkillDigest(filepath.Join(project, SkillsSubdir, "hyperframes-core"))
	if err != nil {
		t.Fatal(err)
	}
	if restored != first.Skills["hyperframes-core"] {
		t.Error("内容还原后摘要没有回到原值——哈希不稳定")
	}
}
