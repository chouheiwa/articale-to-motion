package cast

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/styleimage"
)

// 预览缩略图宽度，与 internal/preset/gen 的风格示例保持同一量级。
const previewThumbnailWidthPx = 240

// PoseSVG 把命名姿势静态地烘进 rig SVG，供人眼验收用。
//
// 用正则在 <g id="j-…"> 上追加 transform 而不是重新序列化整棵 XML 树：
// 与 internal/preset/gen 不做 YAML 重序列化同理——手绘 SVG 里的属性顺序、
// 数字写法和注释都会被序列化器改写，验收图与真实渲染就对不上了。
var jointGroupPattern = regexp.MustCompile(`<g\s+id="j-([A-Za-z0-9_-]+)"`)

func PoseSVG(pack Pack, poseName string) (string, error) {
	pose, ok := pack.Poses[poseName]
	if !ok {
		return "", fmt.Errorf("角色 %s 没有姿势 %s", pack.ID, poseName)
	}
	body, err := os.ReadFile(filepath.Join(pack.Dir, pack.Rig.File))
	if err != nil {
		return "", fmt.Errorf("读取 rig 失败：%w", err)
	}
	out := jointGroupPattern.ReplaceAllStringFunc(string(body), func(match string) string {
		name := jointGroupPattern.FindStringSubmatch(match)[1]
		angle, ok := pose[name]
		if !ok || angle == 0 {
			return match
		}
		pivot := pack.Rig.Joints[name].Pivot
		return fmt.Sprintf(`%s transform="rotate(%s %s %s)"`, match,
			trimFloat(angle), trimFloat(pivot[0]), trimFloat(pivot[1]))
	})
	return out, nil
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
