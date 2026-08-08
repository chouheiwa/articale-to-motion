// Package styleimage 把风格示例 SVG 渲染成 PNG。
//
// 独立成包是因为渲染器的选择有个不显眼的坑，两处调用点（go generate 的生成器
// 与 am validate style --regenerate-examples）必须共用同一份判断，否则修一处
// 漏一处。
package styleimage

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// RenderSVG 把 svgPath 渲染成 pngPath。
//
// 必须用 rsvg-convert 而不是 ImageMagick：多数 ImageMagick 构建自带一个内置的
// XML SVG 渲染器（magick -list format 里的 MSVG / SVG "XML x.y.z"），它会抢在
// rsvg 委托前面接管 .svg，却渲染不出文字、也画不对背景填充——产出的是一张黑底
// 无字的图，而且退出码为 0，不会报错。风格示例是给人看的排版基准，静默坏掉比
// 直接失败更糟。
//
// 加 -background 参数同样会落进内置渲染器，所以这里连试都不试 ImageMagick。
func RenderSVG(svgPath, pngPath string) error {
	rsvg, err := exec.LookPath("rsvg-convert")
	if err != nil {
		return fmt.Errorf("需要 librsvg 的 `rsvg-convert` 命令（ImageMagick 的内置 SVG 渲染器会产出黑底无字的图）")
	}
	if out, err := exec.Command(rsvg, "-o", pngPath, svgPath).CombinedOutput(); err != nil {
		return fmt.Errorf("rsvg-convert 渲染 %s 失败: %w: %s", filepath.Base(svgPath), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ContactSheet 把若干张 PNG 拼成一张概览图。
// 这一步的输入已经是位图，ImageMagick 的 montage 没有上面那个问题。
func ContactSheet(pngPaths []string, outputPath, geometry, background string) error {
	magick, err := exec.LookPath("magick")
	if err != nil {
		return fmt.Errorf("需要 ImageMagick 的 `magick` 命令")
	}
	args := append([]string{"montage"}, pngPaths...)
	args = append(args, "-thumbnail", geometry, "-tile", "4x1", "-geometry", geometry+"+10+10",
		"-background", background, outputPath)
	if out, err := exec.Command(magick, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ImageMagick 生成 contact sheet 失败: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
