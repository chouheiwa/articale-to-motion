package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

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
