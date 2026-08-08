package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chouheiwa/articale-to-motion/internal/preset"
)

func TestResolveCanvasAcceptsKnownID(t *testing.T) {
	p, err := resolveCanvas("vertical-9x16", nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("resolveCanvas: %v", err)
	}
	if p.Canvas.HeightPx != 1920 {
		t.Errorf("高度 = %d，期望 1920", p.Canvas.HeightPx)
	}
}

func TestResolveCanvasRejectsUnknownID(t *testing.T) {
	_, err := resolveCanvas("landscape-16x9", nil, &bytes.Buffer{})
	if err == nil {
		t.Fatal("未内置的画幅应报错")
	}
	for _, id := range preset.IDs() {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("错误信息应列出 %s，实际：%v", id, err)
		}
	}
}

// /dev/null 是字符设备但不是终端。用 os.ModeCharDevice 判断会把重定向输入
// 误认成交互，绕过失败关闭掉进读不到按键的选择框。
func TestIsTerminalRejectsDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("/dev/null 不应被判定为终端")
	}
}

func TestIsTerminalRejectsRegularFileAndNil(t *testing.T) {
	if isTerminal(nil) {
		t.Error("nil 不应被判定为终端")
	}
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("普通文件不应被判定为终端")
	}
}

// 非交互且未传 --canvas 时，错误必须点名 --canvas，而不是抛出选择框相关的话。
func TestResolveCanvasFailsClosedWithoutTerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = resolveCanvas("", f, &bytes.Buffer{})
	if err == nil {
		t.Fatal("非交互环境应报错")
	}
	if !strings.Contains(err.Error(), "非交互环境") {
		t.Errorf("应走失败关闭分支，实际：%v", err)
	}
}

func TestPickerStartsOnDefaultPreset(t *testing.T) {
	m := newCanvasPicker()
	if m.cursor != 0 {
		t.Errorf("初始光标 = %d，期望 0", m.cursor)
	}
	if m.Selected().ID != preset.Default().ID {
		t.Errorf("初始选中 = %s，期望 %s", m.Selected().ID, preset.Default().ID)
	}
}

func TestPickerMovesAndSelects(t *testing.T) {
	m := newCanvasPicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(canvasPicker)
	if m.cursor != 1 {
		t.Fatalf("下移后光标 = %d，期望 1", m.cursor)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(canvasPicker)
	if !m.confirmed || m.Selected().ID != "vertical-9x16" {
		t.Errorf("确认后 = %v %s", m.confirmed, m.Selected().ID)
	}
}

func TestPickerSupportsVimKeys(t *testing.T) {
	m := newCanvasPicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if next.(canvasPicker).cursor != 1 {
		t.Error("j 应下移")
	}
	next, _ = next.(canvasPicker).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if next.(canvasPicker).cursor != 0 {
		t.Error("k 应上移")
	}
}

func TestPickerClampsAtBoundaries(t *testing.T) {
	m := newCanvasPicker()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if next.(canvasPicker).cursor != 0 {
		t.Error("首项上移应停在 0")
	}
	last := len(m.choices) - 1
	m.cursor = last
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if next.(canvasPicker).cursor != last {
		t.Error("末项下移应停在末位")
	}
}

// Ctrl+C / Esc 必须走取消路径，不能被当成选中默认值。
func TestPickerAbort(t *testing.T) {
	for name, key := range map[string]tea.KeyMsg{
		"ctrl-c": {Type: tea.KeyCtrlC},
		"esc":    {Type: tea.KeyEsc},
		"q":      {Type: tea.KeyRunes, Runes: []rune("q")},
	} {
		t.Run(name, func(t *testing.T) {
			next, _ := newCanvasPicker().Update(key)
			m := next.(canvasPicker)
			if !m.aborted || m.confirmed {
				t.Errorf("aborted=%v confirmed=%v", m.aborted, m.confirmed)
			}
		})
	}
}

func TestPickerViewMarksCursorAndListsAllPresets(t *testing.T) {
	m := newCanvasPicker()
	m.cursor = 1
	view := m.View()
	for _, p := range preset.All() {
		if !strings.Contains(view, p.Label) {
			t.Errorf("视图缺少 %s", p.Label)
		}
	}
	if !strings.Contains(view, "❯ "+m.choices[1].Label) {
		t.Errorf("光标未标在第二项：\n%s", view)
	}
}
