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
	for _, name := range []string{"PROMPT.md", "PROMPT-PRODUCTION.md"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if strings.Contains(text, "./am") || strings.Contains(strings.ToLower(text), "agent") {
			t.Errorf("%s still exposes legacy entry or executor identity", name)
		}
	}
}
