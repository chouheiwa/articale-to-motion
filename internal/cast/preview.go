package cast

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/styleimage"
)

// 预览缩略图宽度，与 internal/preset/gen 的风格示例保持同一量级。
const previewThumbnailWidthPx = 240

// PoseSVG 把命名姿势静态地烘进 rig SVG，供人眼验收用。
//
// 用 xml.Decoder 定位 <g id="j-…"> 的注入点，只在原始字节上做插入，而不是
// 重新序列化整棵 XML 树：与 internal/preset/gen 不做 YAML 重序列化同理——
// 手绘 SVG 里的属性顺序、数字写法和注释都会被序列化器改写，验收图与真实
// 渲染就对不上了。
//
// 之前用正则匹配 `<g\s+id="j-…"`，要求 id 必须紧跟在 <g 后面——手写 rig 里
// `<g class="limb" id="j-head">` 这种属性顺序完全不匹配，整个关节的旋转
// 被静默丢掉、不报错。xml.Decoder 按属性名查找，与顺序无关，能修好这个洞。
// ValidateRig 已经拦住了 j- 分组自带原生 transform 的写法（会和这里注入的
// transform 冲突成无效 XML），所以这里不需要再处理"合并已有 transform"。
func PoseSVG(pack Pack, poseName string) (string, error) {
	pose, ok := pack.Poses[poseName]
	if !ok {
		return "", fmt.Errorf("角色 %s 没有姿势 %s", pack.ID, poseName)
	}
	raw, err := os.ReadFile(filepath.Join(pack.Dir, pack.Rig.File))
	if err != nil {
		return "", fmt.Errorf("读取 rig 失败：%w", err)
	}
	src := string(raw)

	type injection struct {
		at   int
		text string
	}
	var injections []injection

	decoder := xml.NewDecoder(strings.NewReader(src))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("解析 rig 失败：%w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "g" {
			continue
		}
		id := attr(start, "id")
		if !strings.HasPrefix(id, jointPrefix) {
			continue
		}
		name := strings.TrimPrefix(id, jointPrefix)
		angle, ok := pose[name]
		if !ok || angle == 0 {
			continue
		}
		pivot := pack.Rig.Joints[name].Pivot

		// decoder.InputOffset() 在读完这个 start tag 后，指向它结束处
		// 的下一个字节：普通标签是 "...>" 之后，自闭合标签是 ".../>" 之后。
		// 插入点要落在那个 ">" 之前——自闭合标签还要往前多让一位，避开 "/"。
		end := int(decoder.InputOffset())
		at := end - 1
		if end >= 2 && src[end-2:end] == "/>" {
			at = end - 2
		}
		injections = append(injections, injection{
			at: at,
			text: fmt.Sprintf(` transform="rotate(%s %s %s)"`,
				trimFloat(angle), trimFloat(pivot[0]), trimFloat(pivot[1])),
		})
	}

	// 从后往前插入，避免前面的插入改变后面记录下来的字节偏移。
	sort.Slice(injections, func(i, j int) bool { return injections[i].at > injections[j].at })
	for _, inj := range injections {
		src = src[:inj.at] + inj.text + src[inj.at:]
	}
	return src, nil
}

func trimFloat(value float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.3f", value), "0"), ".")
}

// Preview 把每个命名姿势渲成 PNG 并拼成一张 contact sheet。
func Preview(pack Pack, outDir string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	names := make([]string, 0, len(pack.Poses))
	for name := range pack.Poses {
		names = append(names, name)
	}
	sort.Strings(names)

	pngs := make([]string, 0, len(names))
	for _, name := range names {
		body, err := PoseSVG(pack, name)
		if err != nil {
			return "", err
		}
		svgPath := filepath.Join(outDir, pack.ID+"-"+name+".svg")
		if err := os.WriteFile(svgPath, []byte(body), 0o644); err != nil {
			return "", err
		}
		pngPath := strings.TrimSuffix(svgPath, ".svg") + ".png"
		if err := styleimage.RenderSVG(svgPath, pngPath); err != nil {
			return "", err
		}
		pngs = append(pngs, pngPath)
	}
	sheet := filepath.Join(outDir, pack.ID+"-contact-sheet.png")
	geometry := fmt.Sprintf("%dx", previewThumbnailWidthPx)
	tile := fmt.Sprintf("4x%d", (len(names)+3)/4)
	if err := styleimage.ContactSheet(pngs, sheet, geometry, "white", tile); err != nil {
		return "", err
	}
	return sheet, nil
}
