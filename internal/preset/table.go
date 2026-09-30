package preset

import "fmt"

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

// verticalPlatform 是两套竖屏画幅共用的平台措辞：面向抖音等竖屏平台，
// 右侧互动栏与底部 UI 是关键避让区。
func verticalPlatform() Platform {
	return Platform{
		Orientation: "竖屏",
		UI:          "抖音右侧互动栏和底部 UI",
		Zone:        "右侧互动栏和底部 UI",
		Avoidance: func(critical ResolvedBox) string {
			return fmt.Sprintf("右侧 %dpx 和底部 %dpx 是抖音互动栏与底部 UI", critical.RightPx, critical.BottomPx)
		},
	}
}

// landscapeSafeArea 是 16:9 横屏的安全区。
//
// 横屏平台（B 站、YouTube、西瓜）没有右侧互动栏，需要让开的是底部的播放器
// 控制栏、进度条与弹幕输入区，所以左右对称内缩、底部留得最深。字幕带贴在
// 进度条上方；封面标题块贴顶，留出下方给主视觉。
func landscapeSafeArea() []Box {
	return []Box{
		{Name: "structural", LeftPx: 48, RightPx: 48, Anchor: AnchorFill, InsetTopPx: 40, InsetBottomPx: 40},
		{Name: "main_content", LeftPx: 120, RightPx: 120, Anchor: AnchorFill, InsetTopPx: 80, InsetBottomPx: 96},
		{Name: "critical_text", LeftPx: 160, RightPx: 160, Anchor: AnchorFill, InsetTopPx: 96, InsetBottomPx: 200},
		{Name: "cover_title", LeftPx: 160, RightPx: 160, Anchor: AnchorTop, InsetTopPx: 220, HeightPx: 520},
		{Name: "subtitles", LeftPx: 240, RightPx: 240, Anchor: AnchorBottom, InsetBottomPx: 90, HeightPx: 150},
	}
}

func landscapePlatform() Platform {
	return Platform{
		Orientation: "横屏",
		UI:          "播放器底部控制栏与进度条",
		Zone:        "底部控制栏与进度条",
		Avoidance: func(critical ResolvedBox) string {
			return fmt.Sprintf("底部 %dpx 是播放器控制栏、进度条与弹幕输入区", critical.BottomPx)
		},
	}
}

// landscapeLayoutNotes 补上横屏与竖屏构图的差异。风格说明书与示例的版式骨架
// 按竖屏写成，横屏照搬「上下堆叠」会在两侧留出大片空白、中间挤成一条窄栏。
const landscapeLayoutNotes = `

## 横屏构图要点

本项目是 16:9 横屏（1920×1080），画面的主方向是左右而不是上下。风格说明书里按竖屏描述的版式骨架，落到横屏时按下面的原则改排：

- 左右分栏优先于上下堆叠：命题型用「左文右图」或「左大字、右主视觉」；对照型直接左右对开；流程型沿水平方向展开，最多 5 步，超过就分两行；系统总览用横向网格。
- 不要把竖屏版式原样居中：中间一条窄栏、两侧大面积空白，是横屏最常见的「没排完」。
- 一行文字不超过 22 个汉字：横屏宽度大，单行过长会让视线来回扫，宁可断行。
- 标题字号以画面高度 1080px 为基准，与竖屏的同级字号保持一致，不因为画面变宽而放大。
- 底部控制栏与进度条是关键避让区：标题、数字、结论和字幕都不得进入 critical_text 下沿以下的区域；字幕带贴在进度条上方。
`

// 顺序即 am init 选择框里的展示顺序，第一项是默认值。
var builtin = []Preset{
	{
		ID:       "vertical-3x4",
		Label:    "3:4  竖屏  1080×1440",
		Canvas:   Canvas{WidthPx: 1080, HeightPx: 1440, FPS: 30, Orientation: "vertical"},
		SafeArea: baseSafeArea(),
		Platform: verticalPlatform(),
	},
	{
		ID:       "vertical-9x16",
		Label:    "9:16 竖屏  1080×1920",
		Canvas:   Canvas{WidthPx: 1080, HeightPx: 1920, FPS: 30, Orientation: "vertical"},
		SafeArea: baseSafeArea(),
		Platform: verticalPlatform(),
	},
	{
		ID:          "landscape-16x9",
		Label:       "16:9 横屏  1920×1080",
		Canvas:      Canvas{WidthPx: 1920, HeightPx: 1080, FPS: 30, Orientation: "landscape"},
		SafeArea:    landscapeSafeArea(),
		Platform:    landscapePlatform(),
		LayoutNotes: landscapeLayoutNotes,
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
