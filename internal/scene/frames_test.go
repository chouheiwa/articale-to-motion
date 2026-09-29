package scene

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 实跑 C 语言那期时，4 个镜头写成 14.367 秒一类的时长：14.367×30=431.01 帧，
// 渲染器向上取整多渲一帧，母版因此比时间轴长 4 帧，只能重渲染。
func TestVerifyFrameAlignmentRejectsFractionalFrames(t *testing.T) {
	dir := writeScene(t, `{"id":"scene-001","duration_seconds":14.367,"output":"out.mp4","transcript":"transcript.srt","text":"hello","style_guide":"frame.md"}`)
	os.WriteFile(filepath.Join(dir, "frame.md"), []byte(frameWithCanvas(1080, 1440)), 0o644)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = VerifyFrameAlignment(s)
	var verification *VerificationError
	if !errors.As(err, &verification) {
		t.Fatalf("非整数帧时长应返回 VerificationError（确定性失败，不重试），实际 %v", err)
	}
	message := err.Error()
	for _, want := range []string{"431.01", "14.366667"} {
		if !strings.Contains(message, want) {
			t.Errorf("错误信息应给出实际帧数与可用的精确时长 %q：%s", want, message)
		}
	}
}

// 精确到 6 位小数的 N/fps 时长是对齐的；整数秒自然也是。
func TestVerifyFrameAlignmentAcceptsExactFrames(t *testing.T) {
	for _, duration := range []string{"14.366667", "4", "0.5"} {
		dir := writeScene(t, `{"id":"scene-001","duration_seconds":`+duration+`,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`)
		s, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyFrameAlignment(s); err != nil {
			t.Errorf("%s 秒应视为整数帧：%v", duration, err)
		}
	}
}

// 提示词若把 14.366667 格式化成 14.367，渲染器照样多出一帧。必须把帧数写明。
func TestBuildPromptStatesExactFrameCount(t *testing.T) {
	dir := writeScene(t, `{"id":"scene-001","duration_seconds":14.366667,"output":"out.mp4","transcript":"transcript.srt","text":"hello"}`)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := BuildPrompt(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"431 帧", "431/30", "正好 431 帧"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("提示词缺少 %q", want)
		}
	}
}
