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
