package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

const UserMessageMarker = "[[USER_MESSAGE]]"

func RendererInvocation(tool, prompt string, unsafe bool) ([]string, error) {
	switch tool {
	case "codex":
		if unsafe {
			return []string{"codex", "exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "--json", prompt}, nil
		}
		return []string{"codex", "--ask-for-approval", "never", "exec", "--skip-git-repo-check", "--sandbox", "workspace-write", "--json", prompt}, nil
	case "claude":
		mode := []string{"--permission-mode", "acceptEdits"}
		if unsafe {
			mode = []string{"--dangerously-skip-permissions"}
		}
		return append([]string{"claude", "-p"}, append(mode, "--verbose", "--output-format", "stream-json", "--prompt-suggestions", "false", prompt)...), nil
	case "qoder":
		mode := []string{"--permission-mode", "dont_ask"}
		if unsafe {
			mode = []string{"--dangerously-skip-permissions"}
		}
		return append([]string{"qoderclicn", "-p", prompt}, append(mode, "--output-format", "stream-json")...), nil
	case "codebuddy":
		mode := []string{"--permission-mode", "acceptEdits"}
		if unsafe {
			mode = []string{"-y"}
		}
		return append([]string{"codebuddy", "-p", "--verbose", "--output-format", "stream-json"}, append(mode, prompt)...), nil
	case "opencode":
		args := []string{"opencode", "run"}
		if unsafe {
			args = append(args, "--auto")
		}
		return append(args, "--format", "json", prompt), nil
	default:
		return nil, fmt.Errorf("未知工具：%s", tool)
	}
}

func OrchestratorInvocation(tool, workdir, prompt string, unsafe bool) ([]string, string, error) {
	switch tool {
	case "codex":
		if unsafe {
			return []string{"codex", "exec", "--skip-git-repo-check", "--cd", workdir, "--dangerously-bypass-approvals-and-sandbox", "-"}, prompt, nil
		}
		return []string{"codex", "--ask-for-approval", "never", "exec", "--skip-git-repo-check", "--cd", workdir, "--sandbox", "workspace-write", "-"}, prompt, nil
	case "claude":
		if unsafe {
			return []string{"claude", "-p", prompt, "--dangerously-skip-permissions"}, "", nil
		}
		return []string{"claude", "-p", prompt, "--permission-mode", "acceptEdits"}, "", nil
	case "qoder":
		if unsafe {
			return []string{"qoderclicn", "-p", prompt, "--dangerously-skip-permissions", "--output-format", "stream-json"}, "", nil
		}
		return []string{"qoderclicn", "-p", prompt, "--permission-mode", "dont_ask", "--output-format", "stream-json"}, "", nil
	case "codebuddy":
		if unsafe {
			return []string{"codebuddy", "-p", prompt, "-y"}, "", nil
		}
		return []string{"codebuddy", "-p", prompt, "--permission-mode", "acceptEdits"}, "", nil
	case "opencode":
		args := []string{"opencode", "run", "--dir", workdir}
		if unsafe {
			args = append(args, "--auto")
		}
		return append(args, prompt), "", nil
	default:
		return nil, "", fmt.Errorf("未知工具：%s", tool)
	}
}

// ProjectRulesFilename 返回该工具会自动从工作目录向上递归读取的项目级指令文件名。
//
// 名字必须逐个工具核对，不能统一成 AGENTS.md：Claude Code 官方文档明写
// "Claude Code reads CLAUDE.md, not AGENTS.md"，CodeBuddy 读的是产品名派生的
// CODEBUDDY.md。写错文件名不会报错，只会让规则静默失效。
func ProjectRulesFilename(tool string) (string, error) {
	switch tool {
	case "codex", "qoder", "opencode":
		return "AGENTS.md", nil
	case "claude":
		return "CLAUDE.md", nil
	case "codebuddy":
		return "CODEBUDDY.md", nil
	default:
		return "", fmt.Errorf("未知工具：%s", tool)
	}
}

func ExtractUserMessages(tool, line string) []string {
	var event map[string]any
	if json.Unmarshal([]byte(line), &event) != nil {
		return nil
	}
	var texts []string
	switch tool {
	case "codex":
		item, _ := event["item"].(map[string]any)
		if event["type"] == "item.completed" && item["type"] == "agent_message" {
			if text, ok := item["text"].(string); ok {
				texts = append(texts, text)
			}
		}
	case "claude", "qoder", "codebuddy":
		message, _ := event["message"].(map[string]any)
		content, _ := message["content"].([]any)
		if event["type"] == "assistant" {
			for _, raw := range content {
				block, _ := raw.(map[string]any)
				if block["type"] == "text" {
					if text, ok := block["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
	case "opencode":
		part, _ := event["part"].(map[string]any)
		if event["type"] == "text" {
			if text, ok := part["text"].(string); ok {
				texts = append(texts, text)
			}
		}
	}
	var messages []string
	for _, text := range texts {
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, UserMessageMarker) {
				messages = append(messages, strings.TrimPrefix(line, UserMessageMarker))
			}
		}
	}
	return messages
}
