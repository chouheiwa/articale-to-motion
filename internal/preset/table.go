package preset

// baseSafeArea 是两套竖屏预设共用的 anchor 声明。
//
// 共用是设计意图而非巧合：1080×1440 与 1080×1920 宽度相同，字幕带（190px 高）
// 与封面标题块（720px 高）保持同样的贴底/贴顶边距，多出的 480px 全部归给
// main_content 的可用垂直空间。新增竖屏画幅不应复制这张表。
func baseSafeArea() []Box {
	return []Box{
		{Name: "structural", LeftPx: 40, RightPx: 40, Anchor: AnchorFill, InsetTopPx: 96, InsetBottomPx: 60},
		{Name: "main_content", LeftPx: 88, RightPx: 88, Anchor: AnchorFill, InsetTopPx: 120, InsetBottomPx: 100},
		{Name: "critical_text", LeftPx: 88, RightPx: 180, Anchor: AnchorFill, InsetTopPx: 120, InsetBottomPx: 260},
		{Name: "cover_title", LeftPx: 88, RightPx: 180, Anchor: AnchorTop, InsetTopPx: 260, HeightPx: 720},
		{Name: "subtitles", LeftPx: 88, RightPx: 180, Anchor: AnchorBottom, InsetBottomPx: 260, HeightPx: 190},
	}
}

// 顺序即 am init 选择框里的展示顺序，第一项是默认值。
var builtin = []Preset{
	{
		ID:       "vertical-3x4",
		Label:    "3:4  竖屏  1080×1440",
		Canvas:   Canvas{WidthPx: 1080, HeightPx: 1440, FPS: 30, Orientation: "vertical"},
		SafeArea: baseSafeArea(),
	},
	{
		ID:       "vertical-9x16",
		Label:    "9:16 竖屏  1080×1920",
		Canvas:   Canvas{WidthPx: 1080, HeightPx: 1920, FPS: 30, Orientation: "vertical"},
		SafeArea: baseSafeArea(),
	},
}

func All() []Preset {
	out := make([]Preset, len(builtin))
	copy(out, builtin)
	return out
}

func Default() Preset { return builtin[0] }

func IDs() []string {
	out := make([]string, 0, len(builtin))
	for _, p := range builtin {
		out = append(out, p.ID)
	}
	return out
}

func ByID(id string) (Preset, bool) {
	for _, p := range builtin {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// ByCanvas 供 validate 从 frame.md 的 canvas 反查预设。
func ByCanvas(width, height, fps int, orientation string) (Preset, bool) {
	for _, p := range builtin {
		c := p.Canvas
		if c.WidthPx == width && c.HeightPx == height && c.FPS == fps && c.Orientation == orientation {
			return p, true
		}
	}
	return Preset{}, false
}
