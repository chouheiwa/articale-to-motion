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

// noMirrorAttr 是 cast.js 用来标记"需要二次翻转"的属性名。facing: left 用
// scaleX(-1) 整体镜像角色，带 noMirrorAttr 的元素会被自动二次翻转抵消，
// 否则举牌、字幕板这类带文字的元素在朝左时会镜像成反字。
const noMirrorAttr = "data-no-mirror"

// mirrorSensitiveElements 是带方向/文字语义、必须显式标注 noMirrorAttr（自身
// 或任一祖先）的元素。这是 character-rig SKILL.md 那条规则本身的机械化：
// "牌子、字幕板、任何带文字或方向语义的元素都该在 rig.svg 里打上
// data-no-mirror"。cast.js 的二次翻转只在 rig 子树内按 [data-no-mirror]
// 选择器查找，失效域正好就是 rig.svg 本身，所以按这条规则逐元素检查相对
// cast.js 的实现没有误报余地。
var mirrorSensitiveElements = map[string]bool{"text": true, "tspan": true}

// checkForbiddenStyleTokens 检查内容中是否包含禁止的样式词汇（@keyframes/animation/transition）。
// location 用于错误消息，例如 "rig 的 <style>" 或 "rig 的 style 属性"。
func checkForbiddenStyleTokens(content, location string) []string {
	var problems []string
	lower := strings.ToLower(content)
	for _, needle := range forbiddenStyleTokens {
		if strings.Contains(lower, needle) {
			problems = append(problems, fmt.Sprintf("%s 内不得出现 %s：动画必须由 timeline 驱动", location, needle))
		}
	}
	return problems
}

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

	// mirrorStack 与当前还未闭合的元素栈一一对应：mirrorStack[i] 表示深度 i
	// 的这个元素、或它的任一祖先，是否已经出现过 noMirrorAttr。<style> 走
	// DecodeElement 整体消费、不出现在这条栈里，因为它的子树不可能含真正
	// 需要镜像语义的 SVG 文本元素，见下方 "style" 分支的说明。
	var mirrorStack []bool

	decoder := xml.NewDecoder(file)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return append(problems, fmt.Sprintf("解析 rig SVG 失败：%v", err))
		}

		if _, ok := token.(xml.EndElement); ok {
			if len(mirrorStack) > 0 {
				mirrorStack = mirrorStack[:len(mirrorStack)-1]
			}
			continue
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
			// DecodeElement 会读到这个元素自己的 EndElement 为止，因此
			// 不会有落单的 EndElement token 回到上面的分支——不需要，也
			// 不应该为它 push/pop mirrorStack。
			var body string
			if err := decoder.DecodeElement(&body, &start); err == nil {
				problems = append(problems, checkForbiddenStyleTokens(body, "rig 的 <style>")...)
			}
			continue
		}

		parentCovered := len(mirrorStack) > 0 && mirrorStack[len(mirrorStack)-1]
		covered := parentCovered || hasAttr(start, noMirrorAttr)
		mirrorStack = append(mirrorStack, covered)
		if mirrorSensitiveElements[local] && !covered {
			problems = append(problems, fmt.Sprintf(
				"rig 内 <%s> 缺少 %s：facing=left 时角色整体镜像，带文字/方向语义的元素"+
					"必须显式标注（自身或祖先均可）做二次翻转抵消，否则举牌类内容会镜像成反字且不报错",
				local, noMirrorAttr))
		}

		if local == "svg" && !rootChecked {
			rootChecked = true
			problems = append(problems, checkViewBox(start, rig)...)
		}
		problems = append(problems, checkAttributes(start)...)
		if local == "g" {
			if id := attr(start, "id"); strings.HasPrefix(id, jointPrefix) {
				seenJoints[strings.TrimPrefix(id, jointPrefix)] = true
				problems = append(problems, checkJointGroupNativeTransform(start, id)...)
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

// checkJointGroupNativeTransform 禁止 j- 分组自带原生 transform 属性。
//
// 预览（cast.PoseSVG）与运行时（cast.js 的 pose()）都用
// setAttribute('transform', 'rotate(...)') 独占这个属性：手写的 transform
// 要么被追加成第二个同名属性（无效 XML，rsvg-convert 行为不可预期），要么
// 被静默覆盖——本地看着正常，成片是错的，退出码为 0，与本文件里已有的
// 自走动画禁令是同一类危害。位移/缩放请放在该分组的父级或子级元素上。
func checkJointGroupNativeTransform(start xml.StartElement, id string) []string {
	if attr(start, "transform") == "" {
		return nil
	}
	return []string{fmt.Sprintf(
		"rig 内 <g id=%q> 不得写 transform：驱动库（预览与运行时）独占这个属性，位移/缩放请放在该分组的父级或子级元素上", id)}
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
		case name == "style":
			if strings.Contains(a.Value, "transform-origin") {
				problems = append(problems, "rig 的 style 属性内不得写 transform-origin：pivot 的唯一真相在 character.yaml")
			}
			problems = append(problems, checkForbiddenStyleTokens(a.Value, "rig 的 style 属性")...)
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

func hasAttr(start xml.StartElement, name string) bool {
	for _, a := range start.Attr {
		if a.Name.Local == name {
			return true
		}
	}
	return false
}
