package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/preset"
	"golang.org/x/term"
)

// canvasOptions 供错误信息与选择框共用的可读清单。
func canvasOptions() string {
	lines := make([]string, 0, len(preset.All()))
	for _, p := range preset.All() {
		lines = append(lines, fmt.Sprintf("  %-16s%s", p.ID, p.Label))
	}
	return strings.Join(lines, "\n")
}

// resolveCanvas 决定本次 init 使用哪套画幅预设。
//
// 未显式传入且不在终端里时直接报错，不静默取默认值：画幅一旦选错，整个项目的
// 排版基准、安全区和成片规格都是错的，而这在 init 当下不会有任何症状，要到成片
// 才暴露。这与项目里 AI CLI 权限隔离降级时的处理一致——失败关闭。
func resolveCanvas(flag string, stdin *os.File, stdout io.Writer) (preset.Preset, error) {
	if flag != "" {
		p, ok := preset.ByID(flag)
		if !ok {
			return preset.Preset{}, fmt.Errorf("未知画幅 %q，可选：\n%s", flag, canvasOptions())
		}
		return p, nil
	}
	if !isTerminal(stdin) {
		return preset.Preset{}, fmt.Errorf("非交互环境必须显式传入 --canvas，可选：\n%s", canvasOptions())
	}
	return pickCanvas(stdout)
}

// isTerminal 判断 f 是不是真正的交互终端。
//
// 不能用 os.ModeCharDevice 代替：/dev/null 也是字符设备，`am init < /dev/null`
// 会被误判成交互，绕过失败关闭直接掉进选择框，而选择框在没有 TTY 时读不到按键。
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// pickCanvas 的交互实现在下一步接入 bubbletea。
func pickCanvas(io.Writer) (preset.Preset, error) {
	return preset.Preset{}, fmt.Errorf("交互选择框尚未接入，请显式传入 --canvas，可选：\n%s", canvasOptions())
}
