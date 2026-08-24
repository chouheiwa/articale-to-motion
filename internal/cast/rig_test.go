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
