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

// PoseSVG 把命名姿势静态地烘进 viewName 对应视图的 rig SVG，供人眼验收用。
//
// 同一个姿势在不同视图下旋转的支点不同：这里用 pack.View(viewName) 取出
// 那个视图自己的 File 与 Joints[...].Pivot，而不是永远用默认视图的——否则
// 角色转身后，姿势预览用的还是正面的支点，肉眼验收图和实际渲染对不上。
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
func PoseSVG(pack Pack, viewName, poseName string) (string, error) {
	pose, ok := pack.Poses[poseName]
	if !ok {
		return "", fmt.Errorf("角色 %s 没有姿势 %s", pack.ID, poseName)
	}
	rig, err := pack.View(viewName)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(pack.Dir, rig.File))
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
		if !strings.HasPrefix(id, JointPrefix) {
			continue
		}
		name := strings.TrimPrefix(id, JointPrefix)
		angle, ok := pose[name]
		if !ok || angle == 0 {
			continue
		}
		pivot := rig.Joints[name].Pivot

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

// Preview 为每个视图各出一套 PNG 与一张 contact sheet，返回视图名到
// contact sheet 路径的映射。
//
// 输出文件名必须带视图名（<id>-<view>-<pose>.png、
// <id>-<view>-contact-sheet.png）：不同视图往往共用同一批姿势名，文件名不
// 带视图名的话，后写的视图会静默覆盖先写的视图的同名 PNG——不报错，只是
// 验收图少了几张，人不会第一时间发现。
func Preview(pack Pack, outDir string) (map[string]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pack.Poses))
	for name := range pack.Poses {
		names = append(names, name)
	}
	sort.Strings(names)

	viewNames := pack.ViewNames()
	sheets := make(map[string]string, len(viewNames))
	for _, viewName := range viewNames {
		pngs := make([]string, 0, len(names))
		for _, name := range names {
			body, err := PoseSVG(pack, viewName, name)
			if err != nil {
				return nil, err
			}
			svgPath := filepath.Join(outDir, fmt.Sprintf("%s-%s-%s.svg", pack.ID, viewName, name))
			if err := os.WriteFile(svgPath, []byte(body), 0o644); err != nil {
				return nil, err
			}
			pngPath := strings.TrimSuffix(svgPath, ".svg") + ".png"
			if err := styleimage.RenderSVG(svgPath, pngPath); err != nil {
				return nil, err
			}
			pngs = append(pngs, pngPath)
		}
		sheet := filepath.Join(outDir, fmt.Sprintf("%s-%s-contact-sheet.png", pack.ID, viewName))
		geometry := fmt.Sprintf("%dx", previewThumbnailWidthPx)
		tile := fmt.Sprintf("4x%d", (len(names)+3)/4)
		if err := styleimage.ContactSheet(pngs, sheet, geometry, "white", tile); err != nil {
			return nil, err
		}
		sheets[viewName] = sheet
	}
	return sheets, nil
}
