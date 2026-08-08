// Package preset 是画幅预设的唯一真相源。
//
// 安全区在这里用 anchor 语义声明，由 go generate 在构建期推导成绝对像素写进
// frame.md。anchor 不进项目文件：渲染 agent 读到的始终是现成像素值，无需自行运算。
package preset

import "fmt"

type Anchor string

const (
	// AnchorTop 贴顶：距顶固定，高度固定，底部随画布高度浮动。
	AnchorTop Anchor = "top"
	// AnchorBottom 贴底：距底固定，高度固定，顶部随画布高度浮动。
	AnchorBottom Anchor = "bottom"
	// AnchorFill 上下都固定边距，高度随画布吸收剩余空间。
	AnchorFill Anchor = "fill"
)

type Canvas struct {
	WidthPx     int
	HeightPx    int
	FPS         int
	Orientation string
}

// Label 返回人类可读画幅，使用全角乘号，与文档正文里的写法一致。
func (c Canvas) Label() string {
	return fmt.Sprintf("%d×%d", c.WidthPx, c.HeightPx)
}

type Box struct {
	Name          string
	LeftPx        int
	RightPx       int
	Anchor        Anchor
	InsetTopPx    int // AnchorTop / AnchorFill
	InsetBottomPx int // AnchorBottom / AnchorFill
	HeightPx      int // AnchorTop / AnchorBottom
}

// ResolvedBox 用与 frame.md 一致的四边内缩表示，不是坐标。
type ResolvedBox struct {
	LeftPx   int
	RightPx  int
	TopPx    int
	BottomPx int
}

func (b Box) Resolve(canvasHeight int) (ResolvedBox, error) {
	out := ResolvedBox{LeftPx: b.LeftPx, RightPx: b.RightPx}
	switch b.Anchor {
	case AnchorTop:
		out.TopPx = b.InsetTopPx
		out.BottomPx = canvasHeight - b.InsetTopPx - b.HeightPx
	case AnchorBottom:
		out.BottomPx = b.InsetBottomPx
		out.TopPx = canvasHeight - b.InsetBottomPx - b.HeightPx
	case AnchorFill:
		out.TopPx = b.InsetTopPx
		out.BottomPx = b.InsetBottomPx
	default:
		return ResolvedBox{}, fmt.Errorf("安全区 %s 的 anchor 无效：%s", b.Name, b.Anchor)
	}
	if out.TopPx < 0 || out.BottomPx < 0 || out.TopPx+out.BottomPx >= canvasHeight {
		return ResolvedBox{}, fmt.Errorf("安全区 %s 在 %dpx 画布高度下无效：top=%d bottom=%d", b.Name, canvasHeight, out.TopPx, out.BottomPx)
	}
	return out, nil
}

type NamedBox struct {
	Name string
	Box  ResolvedBox
}

type Preset struct {
	ID       string
	Label    string
	Canvas   Canvas
	SafeArea []Box
}

// ResolveSafeArea 按声明顺序推导全部安全区，顺序与 frame.md 字段顺序一致。
func (p Preset) ResolveSafeArea() ([]NamedBox, error) {
	out := make([]NamedBox, 0, len(p.SafeArea))
	for _, box := range p.SafeArea {
		resolved, err := box.Resolve(p.Canvas.HeightPx)
		if err != nil {
			return nil, fmt.Errorf("预设 %s：%w", p.ID, err)
		}
		out = append(out, NamedBox{Name: box.Name, Box: resolved})
	}
	return out, nil
}
