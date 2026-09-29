package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/preset"
)

// styleOptions 供错误信息与帮助文字共用的可读清单。
func styleOptions() string {
	lines := make([]string, 0, len(preset.AllStyles()))
	for _, s := range preset.AllStyles() {
		lines = append(lines, fmt.Sprintf("  %-32s%s", s.ID, s.Label))
	}
	return strings.Join(lines, "\n")
}

func newStylePicker() choicePicker[preset.Style] {
	return choicePicker[preset.Style]{title: "选择风格", choices: preset.AllStyles(), label: func(s preset.Style) string { return s.Label }}
}

// resolveStyle 决定本次 init 使用哪套风格。
//
// 与画幅不同，非交互且未传 --style 时取默认风格而不报错：风格选错在第一张
// 示例图上就看得出来，不像画幅那样要到成片才暴露；而且 --style 出现之前的
// 脚本和 CI 只传 --canvas，失败关闭会让它们全部坏掉。
func resolveStyle(flag string, stdin *os.File, stdout io.Writer) (preset.Style, error) {
	if flag != "" {
		s, ok := preset.StyleByID(flag)
		if !ok {
			return preset.Style{}, fmt.Errorf("未知风格 %q，可选：\n%s", flag, styleOptions())
		}
		return s, nil
	}
	if !isTerminal(stdin) || len(preset.AllStyles()) == 1 {
		return preset.DefaultStyle(), nil
	}
	return runPicker(newStylePicker(), "风格", stdout)
}
