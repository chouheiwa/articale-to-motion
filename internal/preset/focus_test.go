package preset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShippedSpecsAgreeOnFocus：frame.md 与风格说明书曾要求「每个镜头至少两个
// 视觉焦点」，PROMPT-PRODUCTION.md 却要求「一拍一个焦点」。渲染器同时读到两条
// 强制约束只能择一违反，统一为「同一时刻一个主焦点，整镜最多两个依次登场」。
func TestShippedSpecsAgreeOnFocus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "assets", "presets", "*", "frame.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到生成的 frame.md：%v", err)
	}
	docs, _ := filepath.Glob(filepath.Join("..", "..", "assets", "presets", "*", "docs", "*.md"))
	for _, path := range append(files, docs...) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, conflicting := range []string{"至少有两个视觉焦点", "至少两个焦点", "标出两个焦点"} {
			if strings.Contains(string(content), conflicting) {
				t.Errorf("%s 仍含与「一拍一个焦点」冲突的要求 %q", path, conflicting)
			}
		}
	}
}
