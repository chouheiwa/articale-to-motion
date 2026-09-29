package preset

import "fmt"

// Style 是与画幅正交的视觉风格。
//
// 画幅决定画布尺寸与安全区，风格决定配色、字体、版式骨架、动效语法和禁用项。
// 两者在 am init 时各选一次，由 go generate 组合成 frame.md：canvas 与
// safe_area 两块来自画幅，其余 token 来自风格模板。
type Style struct {
	// ID 同时是 frame.md 的 style_id，也是 assets/styles/ 与模板目录的目录名。
	ID string
	// Name 同时是 frame.md 的 style_name 与风格说明书的文件名前缀。
	Name string
	// Label 是 am init 选择框里的一行说明。
	Label string
	// SheetBackground 是示例 contact sheet 的底色，取该风格的结构线或底色。
	SheetBackground string
	// Fonts 是该风格从字体池额外带进项目的字体；共用的 Noto Sans SC 不在此列。
	Fonts []Font
}

// ArchetypeIDs 是每套风格都必须按此顺序声明的四种镜头骨架。
//
// 四者是叙事职责而不是视觉样式：钩子与强结论、对照、流程、系统总览。各风格
// 用自己的界面隐喻给它们起名、画示例（例如游戏 HUD 风把 capability_deck 画成
// 角色属性面板），但 id 固定，编排提示词与校验才不必随风格改动。
var ArchetypeIDs = []string{"proposition", "comparison", "process", "capability_deck"}

// GuideDoc 返回风格说明书在项目根下的相对路径。
func (s Style) GuideDoc() string {
	return "docs/" + s.Name + "-视频风格说明书.md"
}

// 顺序即 am init 选择框里的展示顺序，第一项是默认值。
var builtinStyles = []Style{
	{
		ID:              "clear-system-blueprint-v1",
		Name:            "清晰系统蓝图",
		Label:           "清晰系统蓝图：知识、技术与产品机制讲解（默认）",
		SheetBackground: "#D5DEEB",
	},
}

func AllStyles() []Style {
	out := make([]Style, len(builtinStyles))
	copy(out, builtinStyles)
	return out
}

func DefaultStyle() Style { return builtinStyles[0] }

func StyleIDs() []string {
	out := make([]string, 0, len(builtinStyles))
	for _, s := range builtinStyles {
		out = append(out, s.ID)
	}
	return out
}

func StyleByID(id string) (Style, bool) {
	for _, s := range builtinStyles {
		if s.ID == id {
			return s, true
		}
	}
	return Style{}, false
}

// StyleByIDOrError 供 validate 从 frame.md 的 style_id 反查风格。
func StyleByIDOrError(id string) (Style, error) {
	if s, ok := StyleByID(id); ok {
		return s, nil
	}
	return Style{}, fmt.Errorf("未知风格 %q，可选：%v", id, StyleIDs())
}
