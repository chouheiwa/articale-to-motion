package dialogue

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
)

// writePlanProject 生成一个可装配的最小项目：两段真实音频 + plan.json。
func writePlanProject(t *testing.T, declaredSecondForFirst float64) (string, string) {
	t.Helper()
	root := t.TempDir()
	audioDir := filepath.Join(root, "production", "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tone(t, audioDir, "seg-001-heiwa.wav", 1.0)
	tone(t, audioDir, "seg-002-zhaocai.wav", 0.5)
	plan := Plan{
		Schema: SchemaVersion,
		Segments: []PlanSegment{
			{Index: 1, Speaker: "heiwa", VoiceID: "v-1", Audio: "production/audio/seg-001-heiwa.wav",
				GapAfterMs: 200, Lines: []PlanLine{{Text: "第一句", StartSeconds: 0, EndSeconds: declaredSecondForFirst}}},
			{Index: 2, Speaker: "zhaocai", VoiceID: "v-2", Audio: "production/audio/seg-002-zhaocai.wav",
				GapAfterMs: 0, Lines: []PlanLine{{Text: "等一下", StartSeconds: 0, EndSeconds: 0.5}}},
		},
	}
	body, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(audioDir, "plan.json")
	if err := fsutil.AtomicWrite(planPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, planPath
}

func TestAssembleProducesThreeArtifacts(t *testing.T) {
	newTestRunner(t) // 只用来在缺 ffmpeg 时 skip
	root, planPath := writePlanProject(t, 1.0)
	result, err := Assemble(context.Background(), Options{Root: root, PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"production/audio/voice.wav", "transcription-production.srt", "production/dialogue.json"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("缺产物 %s：%v", rel, err)
		}
	}
	if len(result.Lines) != 2 || result.Lines[1].Speaker != "zhaocai" {
		t.Errorf("lines = %+v", result.Lines)
	}
	srt, _ := os.ReadFile(filepath.Join(root, "transcription-production.srt"))
	if !strings.HasPrefix(string(srt), "1\n00:00:00,000 --> ") {
		t.Errorf("SRT 开头不对：%q", srt)
	}
}

// 漂移断言：plan 声明的时长与实测严重不符时必须失败，而不是产出错的成片。
func TestAssembleFailsOnDrift(t *testing.T) {
	newTestRunner(t)
	root, planPath := writePlanProject(t, 1.0)
	// 把首段音频换成明显更长的一条，实测总时长会远超按 plan 推出的值。
	tone(t, filepath.Join(root, "production", "audio"), "seg-001-heiwa.wav", 3.0)
	_, err := Assemble(context.Background(), Options{Root: root, PlanPath: planPath, ExpectTotalSeconds: 1.7})
	if err == nil {
		t.Fatal("期望漂移断言失败")
	}
	if !strings.Contains(err.Error(), "漂移") {
		t.Errorf("错误 = %v，期望提到漂移", err)
	}
}

// 复现代码评审发现的场景：某段实际音频跟 plan 声明的时长差出好几倍（文件
// 拿错，或 TTS 输出被截断/重复）。Rebuild 的等比拉伸会把这类差异悄悄摊
// 平，产出的成片画面正常、只是嘴和字对不上——不能指望调用方一定会传
// ExpectTotalSeconds 兜底（当前没有任何调用方传），必须在装配阶段自己拦
// 住，而且要在还没写出 voice.wav 之前就失败。
func TestAssembleFailsWhenSegmentAudioContradictsDeclaredDuration(t *testing.T) {
	newTestRunner(t)
	root, planPath := writePlanProject(t, 1.0)
	// 首段声明 1.0 秒，实际音频做成 3.0 秒。
	tone(t, filepath.Join(root, "production", "audio"), "seg-001-heiwa.wav", 3.0)
	_, err := Assemble(context.Background(), Options{Root: root, PlanPath: planPath})
	if err == nil {
		t.Fatal("期望因分段声明与实测时长不符而失败")
	}
	if !strings.Contains(err.Error(), "第 1 段") || !strings.Contains(err.Error(), "seg-001-heiwa.wav") {
		t.Errorf("错误 = %v，期望点名第 1 段与具体文件", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "production", "audio", "voice.wav")); statErr == nil {
		t.Error("不应该产出 voice.wav：声明与实测差出数倍时必须在拼接前就失败")
	}
}

// writeSingleSegmentProject 生成一个只有一段的最小项目，用来精确控制
// 「声明时长」与「实际音频时长」之间的差值，验证逐段漂移容差的边界。
func writeSingleSegmentProject(t *testing.T, declaredSeconds, actualAudioSeconds float64) (string, string) {
	t.Helper()
	root := t.TempDir()
	audioDir := filepath.Join(root, "production", "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tone(t, audioDir, "seg-001-heiwa.wav", actualAudioSeconds)
	plan := Plan{
		Schema: SchemaVersion,
		Segments: []PlanSegment{
			{Index: 1, Speaker: "heiwa", VoiceID: "v-1", Audio: "production/audio/seg-001-heiwa.wav",
				Lines: []PlanLine{{Text: "第一句", StartSeconds: 0, EndSeconds: declaredSeconds}}},
		},
	}
	body, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(audioDir, "plan.json")
	if err := fsutil.AtomicWrite(planPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, planPath
}

// 阈值内侧：声明 3.0 秒，容差 = max(0.20, 3.0*0.10) = 0.30 秒；实际音频做成
// 3.300 秒，差异恰好等于容差，必须通过——正常的 TTS 抖动不能被新校验误杀。
func TestAssembleSegmentDriftAtToleranceBoundaryPasses(t *testing.T) {
	newTestRunner(t)
	root, planPath := writeSingleSegmentProject(t, 3.0, 3.300)
	if _, err := Assemble(context.Background(), Options{Root: root, PlanPath: planPath}); err != nil {
		t.Fatalf("差异恰好等于容差应当通过，却报错：%v", err)
	}
}

// 阈值外侧：同样声明 3.0 秒、容差 0.30 秒，实际音频做成 3.310 秒，差异刚好
// 越界 10 毫秒，必须失败——证明容差不是摆设。
func TestAssembleSegmentDriftJustOverToleranceFails(t *testing.T) {
	newTestRunner(t)
	root, planPath := writeSingleSegmentProject(t, 3.0, 3.310)
	_, err := Assemble(context.Background(), Options{Root: root, PlanPath: planPath})
	if err == nil {
		t.Fatal("差异刚好超出容差应当失败")
	}
	if !strings.Contains(err.Error(), "漂移") {
		t.Errorf("错误 = %v，期望提到漂移", err)
	}
}
