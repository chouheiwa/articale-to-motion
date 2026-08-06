package tools

import (
	"reflect"
	"testing"
)

func TestRendererInvocationSafeByDefault(t *testing.T) {
	tests := []struct {
		tool string
		want []string
	}{
		{"codex", []string{"codex", "--ask-for-approval", "never", "exec", "--skip-git-repo-check", "--sandbox", "workspace-write", "--json", "prompt"}},
		{"claude", []string{"claude", "-p", "--permission-mode", "acceptEdits", "--verbose", "--output-format", "stream-json", "--prompt-suggestions", "false", "prompt"}},
		{"qoder", []string{"qoderclicn", "-p", "prompt", "--permission-mode", "dont_ask", "--output-format", "stream-json"}},
		{"codebuddy", []string{"codebuddy", "-p", "--verbose", "--output-format", "stream-json", "--permission-mode", "acceptEdits", "prompt"}},
		{"opencode", []string{"opencode", "run", "--format", "json", "prompt"}},
	}
	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			got, err := RendererInvocation(tc.tool, "prompt", false)
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
	got, err := RendererInvocation("codex", "prompt", true)
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
	if _, err := RendererInvocation("unknown", "prompt", false); err == nil {
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
