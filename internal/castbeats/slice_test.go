package castbeats

import (
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/dialogue"
	"github.com/chouheiwa/articale-to-motion/internal/scene"
)

// line 是构造 dialogue.Line 的简写。
func line(index int, speaker string, start, end float64) dialogue.Line {
	return dialogue.Line{SRTIndex: index, Speaker: speaker, StartSeconds: start, EndSeconds: end}
}

// beatsEqual 逐拍比较，浮点用 1e-9 容差。
func beatsEqual(got, want []scene.Beat) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Speaker != want[i].Speaker {
			return false
		}
		if diff := got[i].Start - want[i].Start; diff > 1e-9 || diff < -1e-9 {
			return false
		}
		if diff := got[i].End - want[i].End; diff > 1e-9 || diff < -1e-9 {
			return false
		}
	}
	return true
}

func TestSliceSingleSceneSingleLine(t *testing.T) {
	got, uncovered := slice([]dialogue.Line{line(1, "heiwa", 0, 1.5)}, []float64{0}, []float64{2})
	if len(uncovered) != 0 {
		t.Fatalf("整行都落在镜头内时不应报未覆盖，得到 %v", uncovered)
	}
	want := []scene.Beat{{Speaker: "heiwa", Start: 0, End: 1.5}}
	if !beatsEqual(got[0], want) {
		t.Fatalf("单镜头单行应原样落进本地时间：want %v got %v", want, got[0])
	}
}

// TestSliceLineOffsetSubtracted 钉住"减去镜头起点"这件事本身：第二个镜头
// 的台词落在全局 [2.5, 4.0]，写进 scene.json 的必须是本地 [0, 1.5]。
func TestSliceLineOffsetSubtracted(t *testing.T) {
	got, _ := slice([]dialogue.Line{line(1, "heiwa", 2.5, 4.0)}, []float64{0, 2.5}, []float64{2.5, 2})
	if len(got[0]) != 0 {
		t.Fatalf("第一个镜头不该有节拍，得到 %v", got[0])
	}
	want := []scene.Beat{{Speaker: "heiwa", Start: 0, End: 1.5}}
	if !beatsEqual(got[1], want) {
		t.Fatalf("第二个镜头的节拍应减去镜头起点 2.5：want %v got %v", want, got[1])
	}
}

// TestSliceLineSpanningTwoScenes 一行台词横跨一个切点：必须拆成两拍，
// 前一拍的 end 恰是前镜头时长、后一拍的 start 恰是 0，换算回全局才首尾相接。
func TestSliceLineSpanningTwoScenes(t *testing.T) {
	got, uncovered := slice([]dialogue.Line{line(1, "heiwa", 1.0, 4.0)}, []float64{0, 3}, []float64{3, 3})
	if len(uncovered) != 0 {
		t.Fatalf("跨切点不是未覆盖，得到 %v", uncovered)
	}
	if !beatsEqual(got[0], []scene.Beat{{Speaker: "heiwa", Start: 1.0, End: 3.0}}) {
		t.Errorf("首拍应止于镜头时长 3.0，得到 %v", got[0])
	}
	if !beatsEqual(got[1], []scene.Beat{{Speaker: "heiwa", Start: 0, End: 1.0}}) {
		t.Errorf("尾拍应从 0 起，得到 %v", got[1])
	}
}

// TestSliceLineSpanningThreeScenes 中间那个镜头被整段盖满：本地 [0, 时长]。
func TestSliceLineSpanningThreeScenes(t *testing.T) {
	got, uncovered := slice([]dialogue.Line{line(1, "zhaocai", 0.5, 5.5)},
		[]float64{0, 2, 4}, []float64{2, 2, 2})
	if len(uncovered) != 0 {
		t.Fatalf("跨两个切点也不是未覆盖，得到 %v", uncovered)
	}
	if !beatsEqual(got[0], []scene.Beat{{Speaker: "zhaocai", Start: 0.5, End: 2}}) {
		t.Errorf("首镜头节拍错：%v", got[0])
	}
	if !beatsEqual(got[1], []scene.Beat{{Speaker: "zhaocai", Start: 0, End: 2}}) {
		t.Errorf("中间镜头应被整段盖满：%v", got[1])
	}
	if !beatsEqual(got[2], []scene.Beat{{Speaker: "zhaocai", Start: 0, End: 1.5}}) {
		t.Errorf("尾镜头节拍错：%v", got[2])
	}
}

// TestSliceSceneWithoutLines 中间镜头整段没人说话：这一镜的节拍是空的，
// 不是漏算——空列表与"根本没写"在写盘那一层才区分。
func TestSliceSceneWithoutLines(t *testing.T) {
	got, uncovered := slice([]dialogue.Line{line(1, "heiwa", 0, 1), line(2, "heiwa", 4, 5)},
		[]float64{0, 2, 4}, []float64{2, 2, 2})
	if len(uncovered) != 0 {
		t.Fatalf("不应报未覆盖，得到 %v", uncovered)
	}
	if len(got[1]) != 0 {
		t.Fatalf("中间镜头无人说话时节拍应为空，得到 %v", got[1])
	}
}

// TestSliceBoundaryTouchDoesNotCreateZeroBeat 一行恰好止于切点：下一个镜头
// 不能凭空多出一拍长度为 0 的节拍——scene.validateCast 硬性要求 start < end。
func TestSliceBoundaryTouchDoesNotCreateZeroBeat(t *testing.T) {
	got, _ := slice([]dialogue.Line{line(1, "heiwa", 0, 2)}, []float64{0, 2}, []float64{2, 2})
	if len(got[1]) != 0 {
		t.Fatalf("恰好止于切点时后一镜头不应有节拍，得到 %v", got[1])
	}
}

// TestSliceLineBeyondLastScene 镜头时长之和短于对白：末行超出的部分没有任何
// 镜头能装，必须点名报出来，而不是悄悄截断。
func TestSliceLineBeyondLastScene(t *testing.T) {
	_, uncovered := slice([]dialogue.Line{line(1, "heiwa", 0, 1), line(2, "heiwa", 1, 3)},
		[]float64{0}, []float64{2})
	if len(uncovered) != 1 || uncovered[0] != 1 {
		t.Fatalf("第 2 行超出镜头总时长时应被点名，得到 %v", uncovered)
	}
}
