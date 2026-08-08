package preset

import "testing"

// 3:4 的推导结果必须逐值等于仓库现有 frame.md 的字面值。
// 这是整个预设机制的零破坏地基，改坏了存量项目会撞 am init 的内容冲突检查。
func TestVertical3x4MatchesShippedFrameValues(t *testing.T) {
	assertSafeArea(t, "vertical-3x4", map[string]ResolvedBox{
		"structural":    {LeftPx: 40, RightPx: 40, TopPx: 96, BottomPx: 60},
		"main_content":  {LeftPx: 88, RightPx: 88, TopPx: 120, BottomPx: 100},
		"critical_text": {LeftPx: 88, RightPx: 180, TopPx: 120, BottomPx: 260},
		"cover_title":   {LeftPx: 88, RightPx: 180, TopPx: 260, BottomPx: 460},
		"subtitles":     {LeftPx: 88, RightPx: 180, TopPx: 990, BottomPx: 260},
	})
}

func TestVertical9x16DerivesFromSameAnchors(t *testing.T) {
	assertSafeArea(t, "vertical-9x16", map[string]ResolvedBox{
		"structural":    {LeftPx: 40, RightPx: 40, TopPx: 96, BottomPx: 60},
		"main_content":  {LeftPx: 88, RightPx: 88, TopPx: 120, BottomPx: 100},
		"critical_text": {LeftPx: 88, RightPx: 180, TopPx: 120, BottomPx: 260},
		"cover_title":   {LeftPx: 88, RightPx: 180, TopPx: 260, BottomPx: 940},
		"subtitles":     {LeftPx: 88, RightPx: 180, TopPx: 1470, BottomPx: 260},
	})
}

func assertSafeArea(t *testing.T, id string, want map[string]ResolvedBox) {
	t.Helper()
	p, ok := ByID(id)
	if !ok {
		t.Fatalf("找不到预设 %s", id)
	}
	boxes, err := p.ResolveSafeArea()
	if err != nil {
		t.Fatalf("推导失败：%v", err)
	}
	if len(boxes) != len(want) {
		t.Fatalf("安全区数量 = %d，期望 %d", len(boxes), len(want))
	}
	for _, item := range boxes {
		expected, ok := want[item.Name]
		if !ok {
			t.Fatalf("多出安全区 %s", item.Name)
		}
		if item.Box != expected {
			t.Errorf("%s = %+v，期望 %+v", item.Name, item.Box, expected)
		}
	}
}

// 安全区顺序必须与 frame.md 的字段顺序一致，生成器直接按此顺序写文件。
func TestSafeAreaOrderIsStable(t *testing.T) {
	p, _ := ByID("vertical-3x4")
	boxes, err := p.ResolveSafeArea()
	if err != nil {
		t.Fatalf("推导失败：%v", err)
	}
	want := []string{"structural", "main_content", "critical_text", "cover_title", "subtitles"}
	for i, name := range want {
		if boxes[i].Name != name {
			t.Errorf("第 %d 项 = %s，期望 %s", i, boxes[i].Name, name)
		}
	}
}

func TestResolveRejectsOverflow(t *testing.T) {
	b := Box{Name: "subtitles", Anchor: AnchorBottom, InsetBottomPx: 260, HeightPx: 190}
	if _, err := b.Resolve(400); err == nil {
		t.Fatal("画布高度不足时应报错")
	}
}

func TestResolveRejectsUnknownAnchor(t *testing.T) {
	b := Box{Name: "x", Anchor: Anchor("middle")}
	if _, err := b.Resolve(1440); err == nil {
		t.Fatal("未知 anchor 应报错")
	}
}

func TestByCanvasLookup(t *testing.T) {
	p, ok := ByCanvas(1080, 1920, 30, "vertical")
	if !ok || p.ID != "vertical-9x16" {
		t.Fatalf("反查 = %v %v，期望 vertical-9x16", p.ID, ok)
	}
	if _, ok := ByCanvas(1920, 1080, 30, "horizontal"); ok {
		t.Fatal("未内置的画幅不应命中")
	}
}

func TestDefaultIsVertical3x4(t *testing.T) {
	if Default().ID != "vertical-3x4" {
		t.Fatalf("Default = %s，期望 vertical-3x4", Default().ID)
	}
}

func TestIDsListsAllBuiltins(t *testing.T) {
	got := IDs()
	want := []string{"vertical-3x4", "vertical-9x16"}
	if len(got) != len(want) {
		t.Fatalf("IDs = %v，期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("IDs[%d] = %s，期望 %s", i, got[i], want[i])
		}
	}
}

// All 返回副本，调用方改动不得影响内置表。
func TestAllReturnsCopy(t *testing.T) {
	first := All()
	first[0].ID = "篡改"
	if Default().ID != "vertical-3x4" {
		t.Fatal("All 的返回值被改动后影响了内置表")
	}
}

func TestCanvasLabelUsesFullWidthMultiplicationSign(t *testing.T) {
	got := Canvas{WidthPx: 1080, HeightPx: 1440}.Label()
	if got != "1080×1440" {
		t.Fatalf("Label = %q，期望 1080×1440", got)
	}
}
