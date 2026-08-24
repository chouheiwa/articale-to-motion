package dialogue

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func samplePlan() Plan {
	return Plan{
		Schema: SchemaVersion,
		Segments: []PlanSegment{
			{Index: 1, Speaker: "heiwa", VoiceID: "v-1", Audio: "seg-001-heiwa.wav", GapAfterMs: 100,
				Lines: []PlanLine{
					{Text: "第一句", StartSeconds: 0.00, EndSeconds: 2.14},
					{Text: "第二句", StartSeconds: 2.14, EndSeconds: 4.32},
				}},
			{Index: 2, Speaker: "zhaocai", VoiceID: "v-2", Audio: "seg-002-zhaocai.wav", GapAfterMs: 0,
				Lines: []PlanLine{
					{Text: "等一下", StartSeconds: 0.00, EndSeconds: 1.08},
				}},
		},
	}
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9 }

func TestRebuildOffsetsByMeasuredDurationAndGap(t *testing.T) {
	result, err := Rebuild(samplePlan(), []float64{4.32, 1.08})
	if err != nil {
		t.Fatal(err)
	}
	if !near(result.Segments[1].StartSeconds, 4.42) {
		t.Errorf("第二段起点 = %v，期望 4.42（4.32 + 100ms 间隔）", result.Segments[1].StartSeconds)
	}
	if len(result.Lines) != 3 {
		t.Fatalf("字幕行数 = %d，期望 3", len(result.Lines))
	}
	third := result.Lines[2]
	if third.SRTIndex != 3 || third.Speaker != "zhaocai" {
		t.Errorf("第三行 = %+v", third)
	}
	if !near(third.StartSeconds, 4.42) || !near(third.EndSeconds, 5.50) {
		t.Errorf("第三行时间 = [%v, %v]，期望 [4.42, 5.50]", third.StartSeconds, third.EndSeconds)
	}
}

// 实测时长与 agent 声明的段内时间戳不一致时，必须按实测拉伸，
// 否则误差会一段段累积到片尾。
func TestRebuildScalesLinesToMeasuredDuration(t *testing.T) {
	result, err := Rebuild(samplePlan(), []float64{8.64, 1.08})
	if err != nil {
		t.Fatal(err)
	}
	if !near(result.Lines[0].EndSeconds, 4.28) {
		t.Errorf("首行结束 = %v，期望 4.28（2.14 按 2 倍实测时长拉伸）", result.Lines[0].EndSeconds)
	}
	if !near(result.Segments[1].StartSeconds, 8.74) {
		t.Errorf("第二段起点 = %v，期望 8.74", result.Segments[1].StartSeconds)
	}
}

func TestRebuildRejects(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*Plan)
		measured []float64
		want     string
	}{
		{"实测时长数量对不上", func(p *Plan) {}, []float64{4.32}, "数量"},
		{"段内时间倒挂", func(p *Plan) { p.Segments[0].Lines[1].EndSeconds = 1.0 }, []float64{4.32, 1.08}, "倒挂"},
		{"段内有空洞", func(p *Plan) { p.Segments[0].Lines[1].StartSeconds = 3.0 }, []float64{4.32, 1.08}, "不连续"},
		{"说话人为空", func(p *Plan) { p.Segments[1].Speaker = "" }, []float64{4.32, 1.08}, "说话人"},
		{"间隔为负", func(p *Plan) { p.Segments[0].GapAfterMs = -5 }, []float64{4.32, 1.08}, "间隔"},
		{"实测时长含 NaN", func(p *Plan) {}, []float64{math.NaN(), 1.08}, "无效"},
		{"实测时长含 +Inf", func(p *Plan) {}, []float64{math.Inf(1), 1.08}, "无效"},
		{"行的结束时间为 NaN", func(p *Plan) { p.Segments[0].Lines[1].EndSeconds = math.NaN() }, []float64{4.32, 1.08}, "无效"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := samplePlan()
			tc.mutate(&plan)
			_, err := Rebuild(plan, tc.measured)
			if err == nil {
				t.Fatalf("期望报错含 %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("错误 = %v，期望含 %q", err, tc.want)
			}
		})
	}
}

func TestWriteSRTFormat(t *testing.T) {
	result, err := Rebuild(samplePlan(), []float64{4.32, 1.08})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "transcription-production.srt")
	if err := WriteSRT(path, result.Lines); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "1\n00:00:00,000 --> 00:00:02,140\n第一句\n\n" +
		"2\n00:00:02,140 --> 00:00:04,320\n第二句\n\n" +
		"3\n00:00:04,420 --> 00:00:05,500\n等一下\n"
	if string(body) != want {
		t.Errorf("SRT =\n%q\n期望\n%q", body, want)
	}
}

// SRT 里不得出现说话人：会进成片字幕。
func TestWriteSRTHasNoSpeaker(t *testing.T) {
	result, _ := Rebuild(samplePlan(), []float64{4.32, 1.08})
	path := filepath.Join(t.TempDir(), "a.srt")
	if err := WriteSRT(path, result.Lines); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	for _, speaker := range []string{"heiwa", "zhaocai"} {
		if strings.Contains(string(body), speaker) {
			t.Errorf("SRT 里出现了说话人 %s", speaker)
		}
	}
}

func TestResultWriteJSONOmitsText(t *testing.T) {
	result, _ := Rebuild(samplePlan(), []float64{4.32, 1.08})
	path := filepath.Join(t.TempDir(), "dialogue.json")
	if err := result.WriteJSON(path); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "第一句") {
		t.Error("dialogue.json 不该带字幕正文，正文的真相是 SRT")
	}
	if !strings.Contains(string(body), `"speaker": "zhaocai"`) {
		t.Errorf("dialogue.json 缺说话人：%s", body)
	}
}

func TestWriteSRTEmptyLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.srt")
	if err := WriteSRT(path, []Line{}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if len(body) != 0 {
		t.Errorf("空 lines 应该写出空文件，但得到 %q", body)
	}
}
