package cast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 520">
  <g id="j-head"><circle cx="200" cy="168" r="60" fill="#fff" stroke="#000" stroke-width="6"/></g>
  <g id="j-frontLeg"><path d="M176 330 L176 420" stroke="#000" stroke-width="6"/></g>
</svg>`

var goodRig = Rig{
	File:      "rig.svg",
	ViewBox:   [4]float64{0, 0, 400, 520},
	BaselineY: 512,
	Joints: map[string]Joint{
		"head":     {Pivot: [2]float64{200, 168}, Rotate: [2]float64{-18, 18}},
		"frontLeg": {Pivot: [2]float64{176, 330}, Rotate: [2]float64{-40, 55}},
	},
}

func writeSVG(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rig.svg")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateRigAcceptsGood(t *testing.T) {
	if problems := ValidateRig(writeSVG(t, goodSVG), goodRig); len(problems) != 0 {
		t.Fatalf("期望无问题，得到 %v", problems)
	}
}

func TestValidateRigRejects(t *testing.T) {
	cases := []struct {
		name string
		svg  string
		want string
	}{
		{"viewBox 与 yaml 不一致",
			strings.Replace(goodSVG, `viewBox="0 0 400 520"`, `viewBox="0 0 400 600"`, 1), "viewBox"},
		{"缺关节对应的 g",
			strings.Replace(goodSVG, `id="j-frontLeg"`, `id="frontLeg"`, 1), "frontLeg"},
		{"多出未声明的 j- 分组",
			strings.Replace(goodSVG, "</svg>", `<g id="j-tail"/></svg>`, 1), "tail"},
		{"内嵌 animate",
			strings.Replace(goodSVG, "</svg>", `<animate attributeName="opacity" to="0"/></svg>`, 1), "animate"},
		{"内嵌 animateTransform",
			strings.Replace(goodSVG, "</svg>", `<animateTransform attributeName="transform"/></svg>`, 1), "animateTransform"},
		{"style 里有 keyframes",
			strings.Replace(goodSVG, "</svg>", `<style>@keyframes spin{to{transform:rotate(1turn)}}</style></svg>`, 1), "@keyframes"},
		{"style 里有 transition",
			strings.Replace(goodSVG, "</svg>", `<style>#j-head{transition:all .3s}</style></svg>`, 1), "transition"},
		{"内嵌 script",
			strings.Replace(goodSVG, "</svg>", `<script>1</script></svg>`, 1), "script"},
		{"foreignObject",
			strings.Replace(goodSVG, "</svg>", `<foreignObject width="1" height="1"/></svg>`, 1), "foreignObject"},
		{"外链 href",
			strings.Replace(goodSVG, "</svg>", `<image href="https://example.com/a.png"/></svg>`, 1), "外链"},
		{"non-scaling-stroke",
			strings.Replace(goodSVG, `stroke-width="6"/></g>
  <g id="j-frontLeg">`, `stroke-width="6" vector-effect="non-scaling-stroke"/></g>
  <g id="j-frontLeg">`, 1), "non-scaling-stroke"},
		{"SVG 里写了 transform-origin",
			strings.Replace(goodSVG, `<g id="j-head">`, `<g id="j-head" transform-origin="200 168">`, 1), "transform-origin"},
		{"j- 分组带原生 transform",
			strings.Replace(goodSVG, `<g id="j-head">`, `<g id="j-head" transform="translate(10,0)">`, 1), "transform"},
		{"内联 style 里有 transition",
			strings.Replace(goodSVG, `<g id="j-head">`, `<g id="j-head" style="transition: transform 1s">`, 1), "transition"},
		{"内联 style 里有 animation",
			strings.Replace(goodSVG, `<g id="j-head">`, `<g id="j-head" style="animation: spin 2s linear infinite">`, 1), "animation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := ValidateRig(writeSVG(t, tc.svg), goodRig)
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Errorf("问题列表 = %q，期望含 %q", joined, tc.want)
			}
		})
	}
}

// 非 j- 分组上的 transform 是正常美术手段（比如整体缩放/平移一个装饰性
// 分组），不应被误伤——禁令只针对驱动库要独占的 j-* 关节分组。
func TestValidateRigAllowsTransformOutsideJointGroups(t *testing.T) {
	svg := strings.Replace(goodSVG, "</svg>",
		`<g transform="translate(5,5)"><rect width="1" height="1"/></g></svg>`, 1)
	if problems := ValidateRig(writeSVG(t, svg), goodRig); len(problems) != 0 {
		t.Fatalf("非 j- 分组上的 transform 不应报错，得到 %v", problems)
	}
}

// TestValidateRigRequiresDataNoMirrorOnTextElements 机械化 SKILL.md 里的
// 规则原文（"牌子、字幕板、任何带文字或方向语义的元素都该在 rig.svg 里
// 打上 data-no-mirror"）：rig 里任何 <text>/<tspan>，若自身及全部祖先都
// 没有 data-no-mirror，就是一条问题。cast.js 的镜像变换（scaleX(-1) 的
// 二次翻转）只在 rig 子树内查 [data-no-mirror]，失效域正好就是 rig.svg
// 本身，这条规则相对该实现没有误报余地。
func TestValidateRigRequiresDataNoMirrorOnTextElements(t *testing.T) {
	svg := strings.Replace(goodSVG, "</svg>",
		`<text x="0" y="0">举牌文字</text></svg>`, 1)
	problems := ValidateRig(writeSVG(t, svg), goodRig)
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "data-no-mirror") {
		t.Fatalf("裸 <text> 缺 data-no-mirror 时应当报问题，得到 %v", problems)
	}
}

// TestValidateRigAcceptsTextWithOwnDataNoMirror 元素自身标注时不应误报。
func TestValidateRigAcceptsTextWithOwnDataNoMirror(t *testing.T) {
	svg := strings.Replace(goodSVG, "</svg>",
		`<text x="0" y="0" data-no-mirror="true">举牌文字</text></svg>`, 1)
	if problems := ValidateRig(writeSVG(t, svg), goodRig); len(problems) != 0 {
		t.Fatalf("<text> 自身已标 data-no-mirror 时不应报问题，得到 %v", problems)
	}
}

// TestValidateRigAcceptsTextInheritingDataNoMirrorFromAncestor 标注在祖先
// 分组上时同样应当豁免：cast.js 的 querySelectorAll('[data-no-mirror]')
// 只按选择器命中，而实际二次翻转对整棵子树生效，标在父级分组上是常见写法。
func TestValidateRigAcceptsTextInheritingDataNoMirrorFromAncestor(t *testing.T) {
	svg := strings.Replace(goodSVG, "</svg>",
		`<g data-no-mirror="true"><text x="0" y="0">举牌文字</text></g></svg>`, 1)
	if problems := ValidateRig(writeSVG(t, svg), goodRig); len(problems) != 0 {
		t.Fatalf("<text> 的祖先分组已标 data-no-mirror 时不应报问题，得到 %v", problems)
	}
}

// TestValidateRigRequiresDataNoMirrorOnTspan tspan 是文字排版里常见的行内
// 拆分元素，同样带方向语义，规则同等适用。
func TestValidateRigRequiresDataNoMirrorOnTspan(t *testing.T) {
	svg := strings.Replace(goodSVG, "</svg>",
		`<text x="0" y="0" data-no-mirror="true"><tspan>没有单独标注的 tspan</tspan></text></svg>`, 1)
	// 注意：这里 <text> 自身标了 data-no-mirror，tspan 作为子孙元素应当继承，
	// 不应报错——用来确认继承逻辑同样覆盖 tspan，而不是只覆盖 text 自身。
	if problems := ValidateRig(writeSVG(t, svg), goodRig); len(problems) != 0 {
		t.Fatalf("tspan 从祖先 text 继承 data-no-mirror 时不应报问题，得到 %v", problems)
	}

	bareSVG := strings.Replace(goodSVG, "</svg>",
		`<text x="0" y="0"><tspan>裸 tspan</tspan></text></svg>`, 1)
	problems := ValidateRig(writeSVG(t, bareSVG), goodRig)
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "tspan") || !strings.Contains(joined, "data-no-mirror") {
		t.Fatalf("裸 tspan（且 text 本身也没标）应当报问题，得到 %v", problems)
	}
}
