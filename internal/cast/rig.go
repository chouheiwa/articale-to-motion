package cast

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
)

// 这些元素在 rig 里一律禁止。
//
// 前四个是 SMIL 自走动画，最后两个能引入不受 timeline 管的执行与外部内容。
// HyperFrames 是单条 paused timeline + 逐帧 seek 出帧：任何不挂在那条
// timeline 上的动画，本地播放正常、成片是错的，而且不报错。
var forbiddenElements = []string{"animate", "animateTransform", "animateMotion", "set", "script", "foreignObject"}

// style 里出现这些片段同样意味着自走动画。
var forbiddenStyleTokens = []string{"@keyframes", "animation", "transition"}

// jointPrefix 是可动件分组 id 的前缀。
const jointPrefix = "j-"

// ValidateRig 校验 rig.svg 的全部硬约束，返回中文问题列表。
// 返回空切片表示通过。一次返回所有问题，避免修一条报一条。
func ValidateRig(svgPath string, rig Rig) []string {
	file, err := os.Open(svgPath)
	if err != nil {
		return []string{fmt.Sprintf("读取 rig 失败：%v", err)}
	}
	defer file.Close()

	var problems []string
	seenJoints := make(map[string]bool)
	rootChecked := false

	decoder := xml.NewDecoder(file)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return append(problems, fmt.Sprintf("解析 rig SVG 失败：%v", err))
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		local := start.Name.Local

		for _, name := range forbiddenElements {
			if local == name {
				problems = append(problems, fmt.Sprintf("rig 内不得出现 <%s>：它不受 timeline 控制，seek 出帧会错且不报错", name))
			}
		}
		if local == "style" {
			var body string
			if err := decoder.DecodeElement(&body, &start); err == nil {
				lower := strings.ToLower(body)
				for _, token := range forbiddenStyleTokens {
					if strings.Contains(lower, token) {
						problems = append(problems, fmt.Sprintf("rig 的 <style> 内不得出现 %s：动画必须由 timeline 驱动", token))
					}
				}
			}
			continue
		}
		if local == "svg" && !rootChecked {
			rootChecked = true
			problems = append(problems, checkViewBox(start, rig)...)
		}
		problems = append(problems, checkAttributes(start)...)
		if local == "g" {
			if id := attr(start, "id"); strings.HasPrefix(id, jointPrefix) {
				seenJoints[strings.TrimPrefix(id, jointPrefix)] = true
			}
		}
	}
	if !rootChecked {
		problems = append(problems, "rig 文件里找不到根 <svg>")
	}
	problems = append(problems, checkJointPairing(seenJoints, rig)...)
	return problems
}

func checkViewBox(start xml.StartElement, rig Rig) []string {
	raw := attr(start, "viewBox")
	var box [4]float64
	if n, err := fmt.Sscanf(strings.Join(strings.Fields(strings.ReplaceAll(raw, ",", " ")), " "),
		"%g %g %g %g", &box[0], &box[1], &box[2], &box[3]); n != 4 || err != nil {
		return []string{fmt.Sprintf("根 <svg> 的 viewBox 无法解析：%q", raw)}
	}
	for i := range box {
		if math.Abs(box[i]-rig.ViewBox[i]) > 1e-9 {
			return []string{fmt.Sprintf("rig.svg 的 viewBox %v 与 character.yaml 声明的 %v 不一致", box, rig.ViewBox)}
		}
	}
	return nil
}

func checkAttributes(start xml.StartElement) []string {
	var problems []string
	for _, a := range start.Attr {
		name := a.Name.Local
		switch {
		case name == "href":
			if value := strings.TrimSpace(a.Value); value != "" && !strings.HasPrefix(value, "#") {
				problems = append(problems, fmt.Sprintf("rig 内不得引用外链资源：%s=%q（渲染机是干净的无头 Chrome，外链静默失败）", name, a.Value))
			}
		case name == "vector-effect" && strings.Contains(a.Value, "non-scaling-stroke"):
			problems = append(problems, "rig 内不得使用 vector-effect=\"non-scaling-stroke\"：角色只占画面高度 22–32%，描边不等比缩放会细成发丝")
		case name == "transform-origin":
			problems = append(problems, "rig 内不得写 transform-origin：pivot 的唯一真相在 character.yaml")
		case name == "style" && strings.Contains(a.Value, "transform-origin"):
			problems = append(problems, "rig 的 style 属性内不得写 transform-origin：pivot 的唯一真相在 character.yaml")
		}
	}
	return problems
}

func checkJointPairing(seen map[string]bool, rig Rig) []string {
	var problems []string
	var missing, extra []string
	for name := range rig.Joints {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	for name := range seen {
		if _, ok := rig.Joints[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		problems = append(problems, fmt.Sprintf("character.yaml 声明的关节在 rig.svg 里找不到对应的 <g id=\"j-…\">：%s", strings.Join(missing, "、")))
	}
	if len(extra) > 0 {
		problems = append(problems, fmt.Sprintf("rig.svg 里的 j- 分组未在 character.yaml 声明：%s", strings.Join(extra, "、")))
	}
	return problems
}

func attr(start xml.StartElement, name string) string {
	for _, a := range start.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
