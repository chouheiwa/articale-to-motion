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
		if strings.Contains(text, "./am") || strings.Contains(strings.ToLower(text), "agent") {
			t.Errorf("%s still exposes legacy entry or executor identity", name)
		}
	}
}
