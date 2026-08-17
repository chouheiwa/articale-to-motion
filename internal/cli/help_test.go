package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// walk 遍历命令树，跳过 cobra 自动生成的 help / completion。
func walk(cmd *cobra.Command, visit func(*cobra.Command)) {
	for _, child := range cmd.Commands() {
		if child.Name() == "help" || child.Name() == "completion" {
			continue
		}
		visit(child)
		walk(child, visit)
	}
}

// TestEveryCommandHasAgentReadableLongHelp 守着 --help 的可用性下限。
//
// 这个 CLI 的使用者主要是外部 AI CLI：它们读 --help 就要能正确驱动 am，读不到
// 就会去猜。单行 Short 撑不起这件事——画幅一次性选定、Stale 判定、归档拒绝条件、
// 安全模式的环境白名单，这些语义只写在 README 里的话，拿到二进制的调用方看不到。
//
// 门槛刻意定得低（有 Long、且比 Short 长），只拦「新增命令时忘了写」，不评价内容好坏。
func TestEveryCommandHasAgentReadableLongHelp(t *testing.T) {
	root := newRoot(&bytes.Buffer{}, &bytes.Buffer{})
	if strings.TrimSpace(root.Long) == "" {
		t.Error("根命令缺少 Long：am --help 是调用方读到的第一屏")
	}
	walk(root, func(cmd *cobra.Command) {
		path := cmd.CommandPath()
		if strings.TrimSpace(cmd.Short) == "" {
			t.Errorf("%s 缺少 Short", path)
		}
		if strings.TrimSpace(cmd.Long) == "" {
			t.Errorf("%s 缺少 Long：调用方只能看到一行 Short，不足以正确使用", path)
			return
		}
		if len(cmd.Long) <= len(cmd.Short) {
			t.Errorf("%s 的 Long 不比 Short 长，等于没写", path)
		}
	})
}

// TestRunnableCommandsCarryExamples 要求可执行的叶子命令给出可直接照抄的调用样例。
// 纯分组命令（config、scene、validate）只做路由，不要求 Example。
func TestRunnableCommandsCarryExamples(t *testing.T) {
	root := newRoot(&bytes.Buffer{}, &bytes.Buffer{})
	walk(root, func(cmd *cobra.Command) {
		if !cmd.Runnable() {
			return
		}
		if strings.TrimSpace(cmd.Example) == "" {
			t.Errorf("%s 缺少 Example", cmd.CommandPath())
			return
		}
		// 样例必须真的调用这个命令，否则是复制粘贴留下的错样例。
		if !strings.Contains(cmd.Example, cmd.CommandPath()) {
			t.Errorf("%s 的 Example 里没有出现该命令本身：%s", cmd.CommandPath(), cmd.Example)
		}
	})
}

// TestPositionalArgErrorsAreChineseAndNameTheArgument 锁住参数错误的可读性。
//
// cobra 默认给的是 "accepts 1 arg(s), received 0"：既是英文，也没说缺的是哪个参数。
// 调用方拿到这行应当能直接知道补什么，不必再去读 --help。
func TestPositionalArgErrorsAreChineseAndNameTheArgument(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"scene", "run"}, "DIRECTORY"},
		{[]string{"scene", "run-all"}, "ROOT"},
		{[]string{"config", "get"}, "KEY"},
		{[]string{"validate", "publish"}, "PATH"},
		{[]string{"init", "a", "b"}, "DIR"},
		{[]string{"archive", "extra"}, "不接受位置参数"},
	}
	for _, tc := range cases {
		var out bytes.Buffer
		if code := Execute(tc.args, &bytes.Buffer{}, &out); code != 1 {
			t.Errorf("%v 应以退出码 1 失败，实际 %d", tc.args, code)
		}
		message := out.String()
		if !strings.Contains(message, tc.want) {
			t.Errorf("%v 的错误信息应点名 %q，实际：%s", tc.args, tc.want, message)
		}
		if strings.Contains(message, "accepts") || strings.Contains(message, "arg(s)") {
			t.Errorf("%v 仍在使用 cobra 的英文默认信息：%s", tc.args, message)
		}
		// 用法行要能直接照着补参数。
		if !strings.Contains(message, "用法：") {
			t.Errorf("%v 的错误信息缺少用法行：%s", tc.args, message)
		}
	}
}

// TestHelpDocumentsBothToolsForExampleRegeneration 守着一个已经犯过的错。
//
// --regenerate-examples 实际需要 rsvg-convert（SVG 转 PNG）和 magick（拼概览图）
// 两个命令，而帮助曾经只写 ImageMagick——按它装工具的人会在第一步就失败。
// SVG 那一步尤其不能用 ImageMagick 顶替：它的内置 SVG 渲染器产出黑底无字的图
// 却返回退出码 0。
func TestHelpDocumentsBothToolsForExampleRegeneration(t *testing.T) {
	root := newRoot(&bytes.Buffer{}, &bytes.Buffer{})
	var style *cobra.Command
	walk(root, func(cmd *cobra.Command) {
		if cmd.CommandPath() == "am validate style" {
			style = cmd
		}
	})
	if style == nil {
		t.Fatal("找不到 am validate style 命令")
	}
	text := style.Long + style.Flags().Lookup("regenerate-examples").Usage
	for _, tool := range []string{"rsvg-convert", "magick"} {
		if !strings.Contains(text, tool) {
			t.Errorf("--regenerate-examples 的帮助没提到必需的 %s", tool)
		}
	}
}

// TestExitCodesAreDocumentedInRootHelp 让退出码契约与实现绑在一起。
// 调用方靠退出码分支，帮助里少写一个就会被当成通用失败处理。
func TestExitCodesAreDocumentedInRootHelp(t *testing.T) {
	root := newRoot(&bytes.Buffer{}, &bytes.Buffer{})
	for _, code := range []string{"127", "130"} {
		if !strings.Contains(root.Long, code) {
			t.Errorf("根命令帮助没有说明退出码 %s", code)
		}
	}
}
