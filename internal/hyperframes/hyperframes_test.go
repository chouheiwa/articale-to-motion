package hyperframes

import (
	"bytes"
	"context"
	"os"
	"os/exec"
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

	result, err := copySkills([]string{source}, dest, nil)
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
	if _, err := copySkills([]string{source}, dest, nil); err != nil {
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

	result, err := copySkills([]string{source}, dest, map[string]bool{"text-to-lottie": true})
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

	if _, err := copySkills([]string{source}, dest, nil); err != nil {
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
	result, err := copySkills([]string{source}, dest, nil)
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
	if _, err := copySkills([]string{source}, dest, nil); err == nil {
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
	if _, err := copySkills([]string{source}, dest, nil); err == nil {
		t.Error("含符号链接的技能应当报错，而不是搬进去一个失效链接")
	}
}

// TestInstallerEnvIsolatesHomeButKeepsNpmReachable 锁住这次改动的核心取舍。
func TestInstallWritesIntoProjectAndNotHome(t *testing.T) {
	url, commit := fixtureRepo(t, "v9.9.9", []string{"hyperframes-animation"}, []string{"hyperframes-core"})
	project := t.TempDir()
	home := t.TempDir()
	result, err := Install(context.Background(), Options{
		ProjectDir: project, Version: "9.9.9", RepoURL: url, ExpectCommit: commit, ExpectSkills: 2,
		Env:    map[string]string{"PATH": os.Getenv("PATH"), "HOME": home},
		Output: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("安装失败：%v", err)
	}
	if len(result.Installed) != 2 {
		t.Errorf("应当装入 2 个技能：%v", result.Installed)
	}
	for _, name := range result.Installed {
		if _, err := os.Stat(filepath.Join(project, SkillsSubdir, name, manifestFile)); err != nil {
			t.Errorf("技能 %s 没有落进项目：%v", name, err)
		}
	}
	// 核心保证：用户 HOME 一个字节都没动。git 会读 ~/.gitconfig、写 ~/.gitcredentials，
	// 所以这条断言在换成 git 之后仍然承重，不是历史遗留。
	if entries, err := os.ReadDir(home); err == nil && len(entries) != 0 {
		t.Errorf("用户 HOME 被写入了：%v", entries)
	}
}

func TestInstallLeavesNoTempDirectoryBehind(t *testing.T) {
	url, commit := fixtureRepo(t, "v9.9.9", []string{"hyperframes-core"}, nil)
	before, _ := filepath.Glob(filepath.Join(os.TempDir(), "am-hyperframes-*"))
	if _, err := Install(context.Background(), Options{
		ProjectDir: t.TempDir(), Version: "9.9.9", RepoURL: url, ExpectCommit: commit, ExpectSkills: 1,
		Env:    map[string]string{"PATH": os.Getenv("PATH")},
		Output: &bytes.Buffer{},
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
		Version:    "9.9.9",
		Env:        map[string]string{"PATH": t.TempDir()},
		Output:     &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("PATH 里没有 git 应当报错")
	}
	// 退出码 127 靠这个前缀判定。
	if !strings.Contains(err.Error(), "缺少必需工具") {
		t.Errorf("错误信息缺少退出码 127 的判定前缀：%v", err)
	}
}

func TestInstallReportsCloneFailure(t *testing.T) {
	_, err := Install(context.Background(), Options{
		ProjectDir: t.TempDir(), Version: "9.9.9",
		RepoURL: "file://" + filepath.Join(t.TempDir(), "does-not-exist"),
		Env:     map[string]string{"PATH": os.Getenv("PATH")},
		Output:  &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "v9.9.9") {
		t.Errorf("克隆失败应当报错并点名 tag：%v", err)
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
	url, commit := fixtureRepo(t, "v9.9.9", []string{"hyperframes-animation"}, []string{"hyperframes-core"})
	project := t.TempDir()
	if _, err := Install(context.Background(), Options{
		ProjectDir: project, Version: "9.9.9", RepoURL: url, ExpectCommit: commit, ExpectSkills: 2,
		Env:    map[string]string{"PATH": os.Getenv("PATH")},
		Output: &bytes.Buffer{},
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
	url, commit := fixtureRepo(t, "v9.9.9", []string{"hyperframes-core"}, nil)
	project := t.TempDir()
	opts := Options{
		ProjectDir: project, Version: "9.9.9", RepoURL: url, ExpectCommit: commit, ExpectSkills: 1,
		Env:    map[string]string{"PATH": os.Getenv("PATH")},
		Output: &bytes.Buffer{},
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

// fixtureRepo 造一个本地上游仓库：skills/ 与 .agents/skills/ 两处各放几个技能，
// 打上 tag。测试用 file:// 克隆它，走的是与真实安装完全相同的代码路径，只是
// 换了个源——比伪造 git 二进制更接近真实，也完全离线。
func fixtureRepo(t *testing.T, tag string, topSkills, agentSkills []string) (url, commit string) {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t",
			"GIT_COMMITTER_EMAIL=t@t", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	for _, name := range topSkills {
		writeSkill(t, filepath.Join(repo, "skills"), name, nil)
	}
	for _, name := range agentSkills {
		writeSkill(t, filepath.Join(repo, ".agents", "skills"), name, nil)
	}
	// 两处各放一个不是技能的条目，验证筛选按 SKILL.md 而不是"目录就算"。
	for dir, name := range map[string]string{
		filepath.Join(repo, "skills"):            "notes.test.mjs",
		filepath.Join(repo, ".agents", "skills"): "README.md",
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("//\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-A")
	run("commit", "-qm", "fixture")
	run("tag", tag)
	return "file://" + repo, run("rev-parse", "HEAD")
}

// TestInstallClonesPinnedCommit 是可复现安装的主用例。
//
// 上游 `hyperframes skills` 只会 git clone 仓库默认分支，没有任何指定 ref 的
// 口子——同一个 CLI 版本号隔几天装出来的技能可以不同。这里改成 am 自己按
// tag 克隆，技能来源因此完全由 PinnedVersion + PinnedSkillsCommit 决定。
func TestInstallClonesPinnedCommit(t *testing.T) {
	url, commit := fixtureRepo(t, "v9.9.9", []string{"alpha", "beta"}, []string{"gamma"})
	project := t.TempDir()
	result, err := Install(context.Background(), Options{
		ProjectDir: project, Version: "9.9.9", RepoURL: url, ExpectCommit: commit,
		ExpectSkills: 3, Env: map[string]string{"PATH": os.Getenv("PATH")},
		Output: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("安装失败：%v", err)
	}
	if got := strings.Join(result.Installed, ","); got != "alpha,beta,gamma" {
		t.Errorf("装入的技能 = %q，期望 alpha,beta,gamma（两处来源合并，非技能条目剔除）", got)
	}
	m, err := ReadManifest(project)
	if err != nil {
		t.Fatal(err)
	}
	if m.UpstreamCommit != commit {
		t.Errorf("来源记录的 commit = %q，期望 %q", m.UpstreamCommit, commit)
	}
}

// TestInstallRefusesMovedTag 守住"完全可复现"这句承诺本身。
//
// tag 是可以被 force-push 移动的。移动之后按 tag 克隆会静默拿到另一份技能，
// 版本号却一个字没变——正是这次改造要消除的失败模式。所以解析出的 commit
// 与固定值不符时必须硬失败，而不是照装。
func TestInstallRefusesMovedTag(t *testing.T) {
	url, commit := fixtureRepo(t, "v9.9.9", []string{"alpha"}, nil)
	_, err := Install(context.Background(), Options{
		ProjectDir: t.TempDir(), Version: "9.9.9", RepoURL: url,
		ExpectCommit: strings.Repeat("0", 40),
		Env:          map[string]string{"PATH": os.Getenv("PATH")},
		Output:       &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("commit 对不上必须失败")
	}
	if !strings.Contains(err.Error(), commit) {
		t.Errorf("错误信息应报出实际 commit 便于排查，实际：%v", err)
	}
}

// TestInstallRefusesUnexpectedSkillCount 让"升级时顺手核对"这件事自己会响。
//
// 上游哪天把 skills/ 挪个位置，按目录拼装的逻辑会静默少装一批技能——渲染
// agent 找不到技能不会报错，只会自己发明写法。数量对不上就拒装。
func TestInstallRefusesUnexpectedSkillCount(t *testing.T) {
	url, commit := fixtureRepo(t, "v9.9.9", []string{"alpha", "beta"}, nil)
	_, err := Install(context.Background(), Options{
		ProjectDir: t.TempDir(), Version: "9.9.9", RepoURL: url, ExpectCommit: commit,
		ExpectSkills: 26, Env: map[string]string{"PATH": os.Getenv("PATH")},
		Output: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "26") {
		t.Fatalf("技能数量对不上必须失败并报出期望值，实际：%v", err)
	}
}
