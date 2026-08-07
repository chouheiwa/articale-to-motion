package scene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installSkill 在 dir 下伪造一份完整的 hyperframes-animation 技能安装。
func installSkill(t *testing.T, dir string) string {
	t.Helper()
	skill := filepath.Join(dir, AnimationSkillName)
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", rulesIndexFile} {
		if err := os.WriteFile(filepath.Join(skill, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestResolveSkillsDirPrefersExplicitEnvironment(t *testing.T) {
	home := t.TempDir()
	installSkill(t, filepath.Join(home, ".claude", "skills"))
	explicit := installSkill(t, filepath.Join(t.TempDir(), "custom"))

	got, err := ResolveSkillsDir("claude", t.TempDir(), map[string]string{"HOME": home, SkillsDirEnv: explicit})
	if err != nil {
		t.Fatal(err)
	}
	if got != explicit {
		t.Fatalf("want %s, got %s", explicit, got)
	}
}

func TestResolveSkillsDirRejectsInvalidExplicitEnvironment(t *testing.T) {
	home := t.TempDir()
	installSkill(t, filepath.Join(home, ".claude", "skills"))

	// 显式配置错了必须报错，而不是悄悄回退到家目录里那份能用的安装。
	_, err := ResolveSkillsDir("claude", t.TempDir(), map[string]string{"HOME": home, SkillsDirEnv: t.TempDir()})
	if err == nil {
		t.Fatal("expected an error for an explicitly configured but invalid skills directory")
	}
	if !strings.Contains(err.Error(), SkillsDirEnv) {
		t.Fatalf("error should name the offending variable, got %v", err)
	}
}

func TestResolveSkillsDirFindsProjectScopeByWalkingUp(t *testing.T) {
	root := t.TempDir()
	project := installSkill(t, filepath.Join(root, ".claude", "skills"))
	sceneDir := filepath.Join(root, "episodes", "episode-03", "scenes", "scene-002")
	if err := os.MkdirAll(sceneDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveSkillsDir("claude", sceneDir, map[string]string{"HOME": t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if got != project {
		t.Fatalf("want %s, got %s", project, got)
	}
}

func TestResolveSkillsDirProjectScopeBeatsHomeScope(t *testing.T) {
	home := t.TempDir()
	installSkill(t, filepath.Join(home, ".claude", "skills"))
	root := t.TempDir()
	project := installSkill(t, filepath.Join(root, ".claude", "skills"))

	got, err := ResolveSkillsDir("claude", root, map[string]string{"HOME": home})
	if err != nil {
		t.Fatal(err)
	}
	if got != project {
		t.Fatalf("project scope must win: want %s, got %s", project, got)
	}
}

func TestResolveSkillsDirPerRendererHomeLocation(t *testing.T) {
	cases := map[string]string{
		"claude":    ".claude/skills",
		"codex":     ".codex/skills",
		"qoder":     ".qoder/skills",
		"codebuddy": ".codebuddy/skills",
		"opencode":  ".config/opencode/skills",
	}
	for renderer, relative := range cases {
		t.Run(renderer, func(t *testing.T) {
			home := t.TempDir()
			want := installSkill(t, filepath.Join(home, filepath.FromSlash(relative)))
			// 另一个渲染器的目录也存在，确保按 renderer 而不是按存在性挑。
			installSkill(t, filepath.Join(home, ".cursor", "skills"))

			got, err := ResolveSkillsDir(renderer, t.TempDir(), map[string]string{"HOME": home})
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("want %s, got %s", want, got)
			}
		})
	}
}

func TestResolveSkillsDirFallsBackToPortableLocation(t *testing.T) {
	home := t.TempDir()
	want := installSkill(t, filepath.Join(home, ".agents", "skills"))

	got, err := ResolveSkillsDir("codex", t.TempDir(), map[string]string{"HOME": home})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestResolveSkillsDirPrefersRendererDirWhenProjectSitsInsideHome(t *testing.T) {
	// 本项目就长这样：项目根在 $HOME 之下，且家目录同时装了通用的 .agents/skills。
	// 向上遍历必须不能在 $HOME 那一层先命中通用目录。
	home := t.TempDir()
	installSkill(t, filepath.Join(home, ".agents", "skills"))
	want := installSkill(t, filepath.Join(home, ".config", "opencode", "skills"))
	sceneDir := filepath.Join(home, "work", "video", "episodes", "scene-002")
	if err := os.MkdirAll(sceneDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveSkillsDir("opencode", sceneDir, map[string]string{"HOME": home})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("renderer-specific dir must win over the portable one: want %s, got %s", want, got)
	}
}

func TestResolveSkillsDirIgnoresIncompleteInstall(t *testing.T) {
	home := t.TempDir()
	// 只有 SKILL.md 没有 rules-index.md：提示词引用的文件不存在，必须当作未找到。
	partial := filepath.Join(home, ".claude", "skills", AnimationSkillName)
	if err := os.MkdirAll(partial, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(partial, "SKILL.md"), []byte("x"), 0o644)

	got, err := ResolveSkillsDir("claude", t.TempDir(), map[string]string{"HOME": home})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("want empty, got %s", got)
	}
}

func TestResolveSkillsDirReturnsEmptyWhenAbsent(t *testing.T) {
	got, err := ResolveSkillsDir("claude", t.TempDir(), map[string]string{"HOME": t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("want empty, got %s", got)
	}
}

func TestResolveSkillsDirUnknownRendererStillUsesPortableLocation(t *testing.T) {
	home := t.TempDir()
	want := installSkill(t, filepath.Join(home, ".agents", "skills"))

	got, err := ResolveSkillsDir("some-future-tool", t.TempDir(), map[string]string{"HOME": home})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestSkillsEnvironmentPrefersProcessEnvOverDotenv(t *testing.T) {
	base := map[string]string{"HOME": "/home/a", SkillsDirEnv: "/from/shell"}
	overlay := map[string]string{SkillsDirEnv: "/from/dotenv"}

	if got := SkillsEnvironment(base, overlay)[SkillsDirEnv]; got != "/from/shell" {
		t.Fatalf("process env must win, got %s", got)
	}
}

func TestSkillsEnvironmentFallsBackToDotenv(t *testing.T) {
	base := map[string]string{"HOME": "/home/a"}
	overlay := map[string]string{SkillsDirEnv: "/from/dotenv"}

	merged := SkillsEnvironment(base, overlay)
	if got := merged[SkillsDirEnv]; got != "/from/dotenv" {
		t.Fatalf("want /from/dotenv, got %s", got)
	}
	if _, polluted := base[SkillsDirEnv]; polluted {
		t.Fatal("SkillsEnvironment must not mutate its input")
	}
}

func TestBuildPromptEmbedsResolvedSkillPath(t *testing.T) {
	dir := writeScene(t, `{"id":"scene-001","duration_seconds":1,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	skills := installSkill(t, filepath.Join(t.TempDir(), "skills"))

	prompt, err := BuildPrompt(s, skills)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		filepath.Join(skills, AnimationSkillName),
		rulesIndexFile,
		blueprintsIndexFile,
		"动效要求（强制）",
		"3 个不同分类",
		"decorative",
		// 只约束属性种类会让渲染器"用新属性做更少的动作"：实测最后 3 秒画面完全冻结。
		// 时间覆盖必须一起约束。
		"贯穿整个镜头时长",
		"45 帧",
		"不等于画面冻结",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestBuildPromptFallsBackToSkillNameWithoutPath(t *testing.T) {
	dir := writeScene(t, `{"id":"scene-001","duration_seconds":1,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	prompt, err := BuildPrompt(s, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, AnimationSkillName) || !strings.Contains(prompt, rulesIndexFile) {
		t.Fatalf("fallback prompt must still name the skill and its index: %s", prompt)
	}
	if strings.Contains(prompt, "本机该技能目录为") {
		t.Fatal("fallback prompt must not claim a machine-local path")
	}
}
