package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPublicTreeContainsNoPrivateOrLegacyContent(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`^transcription\.srt$`),
		regexp.MustCompile(`^transcription-production\.srt$`),
		regexp.MustCompile(`^final\.mp4$`),
		regexp.MustCompile(`^publish\.md$`),
		regexp.MustCompile(`^scenes/`),
		regexp.MustCompile(`^production/`),
		regexp.MustCompile(`^exampleFolder/`),
		regexp.MustCompile(`^auto-hyper-/`),
		regexp.MustCompile(`^readme-assets/`),
		regexp.MustCompile(`^docs/superpowers/`),
		regexp.MustCompile(`^article_to_motion/`),
		regexp.MustCompile(`^auto-test/`),
		regexp.MustCompile(`^pyproject\.toml$`),
		regexp.MustCompile(`^am$`),
	}
	for _, path := range strings.Fields(string(out)) {
		for _, pattern := range forbidden {
			if pattern.MatchString(path) {
				t.Errorf("public tree contains forbidden path %s", path)
			}
		}
	}
}

func TestPromptsUseInstalledCLIAndDoNotExposeExecutorIdentity(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	// 提示词已按「是否含画幅数字」拆进两棵源树：与画幅无关的进 shared，
	// 含画幅的每套预设一份。用 glob 而非写死清单，新增画幅预设自动纳入。
	names := []string{filepath.Join(root, "assets", "shared", "PROMPT.md")}
	production, err := filepath.Glob(filepath.Join(root, "assets", "presets", "*", "PROMPT-PRODUCTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(production) == 0 {
		t.Fatal("assets/presets 下没有任何 PROMPT-PRODUCTION.md，检查素材路径")
	}
	names = append(names, production...)
	for _, name := range names {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if strings.Contains(text, "./am") {
			t.Errorf("%s 引用了 ./am：用户项目里没有这个二进制", name)
		}
		// 内置技能树的路径字面含 "agent"，但它是文件系统路径，不是执行者身份声明，
		// 而提示词必须能点名它——锁定文件要靠这条路径写读取例外。先摘掉这个字面量
		// 再查身份词，其余任何 "agent" 出现仍然失败。
		identity := strings.ReplaceAll(strings.ToLower(text), skillTreeRoot, "")
		if strings.Contains(identity, "agent") {
			t.Errorf("%s 暴露了执行者身份（出现 agent 一词）", name)
		}
	}
}
