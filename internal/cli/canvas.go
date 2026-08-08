package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

type canvasPicker struct {
	choices   []preset.Preset
	cursor    int
	confirmed bool
	aborted   bool
}

func newCanvasPicker() canvasPicker {
	return canvasPicker{choices: preset.All()}
}

func (m canvasPicker) Selected() preset.Preset { return m.choices[m.cursor] }

func (m canvasPicker) Init() tea.Cmd { return nil }

func (m canvasPicker) up() canvasPicker {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

func (m canvasPicker) down() canvasPicker {
	if m.cursor < len(m.choices)-1 {
		m.cursor++
	}
	return m
}

func (m canvasPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyUp:
		return m.up(), nil
	case tea.KeyDown:
		return m.down(), nil
	case tea.KeyEnter:
		m.confirmed = true
		return m, tea.Quit
	case tea.KeyCtrlC, tea.KeyEsc:
		m.aborted = true
		return m, tea.Quit
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "k":
			return m.up(), nil
		case "j":
			return m.down(), nil
		case "q":
			m.aborted = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m canvasPicker) View() string {
	var b strings.Builder
	b.WriteString("选择画幅\n\n")
	for i, p := range m.choices {
		marker := "  "
		if i == m.cursor {
			marker = "❯ "
		}
		b.WriteString(marker + p.Label + "\n")
	}
	b.WriteString("\n↑/↓ 移动　Enter 确认　Esc 取消\n")
	return b.String()
}

func pickCanvas(stdout io.Writer) (preset.Preset, error) {
	final, err := tea.NewProgram(newCanvasPicker(), tea.WithOutput(stdout)).Run()
	if err != nil {
		return preset.Preset{}, fmt.Errorf("画幅选择框启动失败：%w", err)
	}
	model, ok := final.(canvasPicker)
	if !ok {
		return preset.Preset{}, fmt.Errorf("画幅选择框返回了意外的状态")
	}
	if model.aborted || !model.confirmed {
		return preset.Preset{}, fmt.Errorf("已取消初始化")
	}
	return model.Selected(), nil
}
