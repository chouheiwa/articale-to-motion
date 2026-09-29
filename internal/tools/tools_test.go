package tools

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRendererInvocationSafeByDefault(t *testing.T) {
	tests := []struct {
		tool string
		want []string
	}{
		{"codex", []string{"codex", "--ask-for-approval", "never", "exec", "--skip-git-repo-check", "--sandbox", "workspace-write", "--json", "prompt"}},
		{"claude", []string{"claude", "-p", "--permission-mode", "acceptEdits", "--settings", `{"permissions":{"additionalDirectories":[],"allow":[],"deny":["Read(**/.env)","Read(**/.env.*)"]}}`, "--verbose", "--output-format", "stream-json", "--prompt-suggestions", "false", "prompt"}},
		{"qoder", []string{"qoderclicn", "-p", "prompt", "--permission-mode", "dont_ask", "--output-format", "stream-json"}},
		{"codebuddy", []string{"codebuddy", "-p", "--verbose", "--output-format", "stream-json", "--permission-mode", "acceptEdits", "prompt"}},
		{"opencode", []string{"opencode", "run", "--format", "json", "prompt"}},
	}
	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			got, err := RendererInvocation(tc.tool, "prompt", false, RendererAccess{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
		})
	}
}

func TestRendererInvocationUnsafeIsExplicit(t *testing.T) {
	got, err := RendererInvocation("codex", "prompt", true, RendererAccess{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "--json", "prompt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestExtractUserMessages(t *testing.T) {
	line := `{"type":"item.completed","item":{"type":"agent_message","text":"ignore\n[[USER_MESSAGE]]完成"}}`
	got := ExtractUserMessages("codex", line)
	if !reflect.DeepEqual(got, []string{"完成"}) {
		t.Fatalf("unexpected messages: %#v", got)
	}
}

func TestUnknownToolFailsClosed(t *testing.T) {
	if _, err := RendererInvocation("unknown", "prompt", false, RendererAccess{}); err == nil {
		t.Fatal("expected unknown tool error")
	}
}

func TestOrchestratorInvocationUsesSafeWorkspaceMode(t *testing.T) {
	got, stdin, err := OrchestratorInvocation("codex", "/work", "prompt", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "--ask-for-approval", "never", "exec", "--skip-git-repo-check", "--cd", "/work", "--sandbox", "workspace-write", "-"}
	if !reflect.DeepEqual(got, want) || stdin != "prompt" {
		t.Fatalf("got=%#v stdin=%q", got, stdin)
	}
}

func TestOrchestratorInvocationUnsafeSkipsGitCheck(t *testing.T) {
	got, stdin, err := OrchestratorInvocation("codex", "/work", "prompt", true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "exec", "--skip-git-repo-check", "--cd", "/work", "--dangerously-bypass-approvals-and-sandbox", "-"}
	if !reflect.DeepEqual(got, want) || stdin != "prompt" {
		t.Fatalf("got=%#v stdin=%q", got, stdin)
	}
}

func TestAllOrchestratorInvocations(t *testing.T) {
	for _, tool := range []string{"codex", "claude", "qoder", "codebuddy", "opencode"} {
		for _, unsafe := range []bool{false, true} {
			t.Run(tool+map[bool]string{false: "-safe", true: "-unsafe"}[unsafe], func(t *testing.T) {
				argv, _, err := OrchestratorInvocation(tool, "/work", "prompt", unsafe)
				if err != nil || len(argv) == 0 || argv[0] == "" {
					t.Fatalf("argv=%v err=%v", argv, err)
				}
			})
		}
	}
	if _, _, err := OrchestratorInvocation("bad", "/work", "prompt", false); err == nil {
		t.Fatal("expected unknown tool failure")
	}
}

func TestExtractMessagesForEveryStreamShape(t *testing.T) {
	tests := map[string]string{
		"claude":    `{"type":"assistant","message":{"content":[{"type":"text","text":"[[USER_MESSAGE]]claude"}]}}`,
		"qoder":     `{"type":"assistant","message":{"content":[{"type":"text","text":"[[USER_MESSAGE]]qoder"}]}}`,
		"codebuddy": `{"type":"assistant","message":{"content":[{"type":"text","text":"[[USER_MESSAGE]]codebuddy"}]}}`,
		"opencode":  `{"type":"text","part":{"text":"[[USER_MESSAGE]]opencode"}}`,
	}
	for tool, line := range tests {
		got := ExtractUserMessages(tool, line)
		if len(got) != 1 || got[0] != tool {
			t.Fatalf("%s: %#v", tool, got)
		}
	}
	if got := ExtractUserMessages("codex", "not-json"); len(got) != 0 {
		t.Fatal(got)
	}
}

// 文件名写错不会报错，只会让项目规则静默失效，所以逐个工具钉死。
func TestProjectRulesFilenamePerTool(t *testing.T) {
	tests := map[string]string{
		"codex":     "AGENTS.md",
		"qoder":     "AGENTS.md",
		"opencode":  "AGENTS.md",
		"claude":    "CLAUDE.md",
		"codebuddy": "CODEBUDDY.md",
	}
	for tool, want := range tests {
		t.Run(tool, func(t *testing.T) {
			got, err := ProjectRulesFilename(tool)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		})
	}
}

func TestProjectRulesFilenameRejectsUnknownTool(t *testing.T) {
	if _, err := ProjectRulesFilename("cursor"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

// claude 在 acceptEdits 下不能跑 Bash：实测渲染器执行不了
// npx hyperframes render/snapshot，镜头根本渲染不出来。安全模式要用 --settings
// 显式放行这几条命令、开放技能目录读取，并继续挡住 .env。
func TestClaudeRendererSafeModeGrantsMinimalCommands(t *testing.T) {
	access := RendererAccess{
		Commands: []string{"npx --yes hyperframes@0.8.14", "ffprobe"},
		ReadDirs: []string{"/proj/.agents/skills"},
	}
	got, err := RendererInvocation("claude", "prompt", false, access)
	if err != nil {
		t.Fatal(err)
	}
	if got[len(got)-1] != "prompt" {
		t.Fatalf("提示词必须是最后一个参数：%#v", got)
	}
	index := -1
	for i, arg := range got {
		if arg == "--settings" {
			index = i
		}
	}
	if index < 0 || index+1 >= len(got) {
		t.Fatalf("安全模式缺少 --settings：%#v", got)
	}
	var settings struct {
		Permissions struct {
			Allow                 []string `json:"allow"`
			Deny                  []string `json:"deny"`
			AdditionalDirectories []string `json:"additionalDirectories"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(got[index+1]), &settings); err != nil {
		t.Fatalf("--settings 不是合法 JSON：%v", err)
	}
	wantAllow := []string{"Bash(npx --yes hyperframes@0.8.14:*)", "Bash(ffprobe:*)", "Read(//proj/.agents/skills/**)"}
	if !reflect.DeepEqual(settings.Permissions.Allow, wantAllow) {
		t.Errorf("allow = %#v，期望 %#v", settings.Permissions.Allow, wantAllow)
	}
	if !reflect.DeepEqual(settings.Permissions.AdditionalDirectories, []string{"/proj/.agents/skills"}) {
		t.Errorf("additionalDirectories = %#v", settings.Permissions.AdditionalDirectories)
	}
	if !reflect.DeepEqual(settings.Permissions.Deny, []string{"Read(**/.env)", "Read(**/.env.*)"}) {
		t.Errorf("deny = %#v", settings.Permissions.Deny)
	}
}

// 不安全模式已经跳过全部权限检查，不需要也不应该再叠一层 settings。
func TestClaudeRendererUnsafeModeSkipsSettings(t *testing.T) {
	got, err := RendererInvocation("claude", "prompt", true, RendererAccess{Commands: []string{"ls"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range got {
		if arg == "--settings" {
			t.Fatalf("不安全模式不应带 --settings：%#v", got)
		}
	}
}
