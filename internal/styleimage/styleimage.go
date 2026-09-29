// Package styleimage 把风格示例 SVG 渲染成 PNG。
//
// 独立成包是因为渲染器的选择有个不显眼的坑，两处调用点（go generate 的生成器
// 与 am validate style --regenerate-examples）必须共用同一份判断，否则修一处
// 漏一处。
package styleimage

import (
	"fmt"
	"os"
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
// CheckRenderer 确认 rsvg-convert 可用。批量渲染前先调用它，缺工具时第一时间
// 报出最根本的依赖，而不是先报一个下游的辅助工具。
func CheckRenderer() error {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		return fmt.Errorf("需要 librsvg 的 `rsvg-convert` 命令（ImageMagick 的内置 SVG 渲染器会产出黑底无字的图）")
	}
	return nil
}

func RenderSVG(svgPath, pngPath string, fontDirs ...string) error {
	rsvg, err := exec.LookPath("rsvg-convert")
	if err != nil {
		return fmt.Errorf("需要 librsvg 的 `rsvg-convert` 命令（ImageMagick 的内置 SVG 渲染器会产出黑底无字的图）")
	}
	cmd := exec.Command(rsvg, "-o", pngPath, svgPath)
	if len(fontDirs) > 0 {
		config, cleanup, err := fontconfigWith(fontDirs)
		if err != nil {
			return err
		}
		defer cleanup()
		// macOS 上 Homebrew 的 pango 默认走 CoreText，完全不读 fontconfig，
		// FONTCONFIG_FILE 会被静默忽略；强制 fc 后端，额外字体目录才生效。
		cmd.Env = append(os.Environ(), "FONTCONFIG_FILE="+config, "PANGOCAIRO_BACKEND=fc")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rsvg-convert 渲染 %s 失败: %w: %s", filepath.Base(svgPath), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// fontconfigWith 写一份临时 fontconfig 配置：先引入系统默认配置，再追加
// fontDirs。风格自带的像素、衬线、手写字体不装进系统也能被 rsvg-convert
// 找到——示例图用的正是随项目下发的那几个字体文件，而不是本机碰巧装了的字体。
func fontconfigWith(fontDirs []string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "am-fontconfig-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\"?>\n<!DOCTYPE fontconfig SYSTEM \"fonts.dtd\">\n<fontconfig>\n")
	// FONTCONFIG_FILE 会整体替换默认配置，所以把各平台常见的默认配置都引进来。
	for _, system := range []string{"/opt/homebrew/etc/fonts/fonts.conf", "/usr/local/etc/fonts/fonts.conf", "/etc/fonts/fonts.conf"} {
		fmt.Fprintf(&b, "  <include ignore_missing=\"yes\">%s</include>\n", system)
	}
	for _, fontDir := range fontDirs {
		absolute, err := filepath.Abs(fontDir)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		fmt.Fprintf(&b, "  <dir>%s</dir>\n", xmlEscape(absolute))
	}
	fmt.Fprintf(&b, "  <cachedir>%s</cachedir>\n</fontconfig>\n", xmlEscape(filepath.Join(dir, "cache")))
	path := filepath.Join(dir, "fonts.conf")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// ContactSheet 把若干张 PNG 拼成一张概览图。
// 这一步的输入已经是位图，ImageMagick 的 montage 没有上面那个问题。
//
// tile 必须由调用方按图片数量算好传入（如 "4x1"、"4x3"）：montage 在图片数
// 超过 tile 容量时不报错，而是静默产出 out-0.png、out-1.png 多个文件，
// outputPath 那个路径根本不存在。这里不设默认值，逼调用方自己算。
func ContactSheet(pngPaths []string, outputPath, geometry, background, tile string) error {
	magick, err := exec.LookPath("magick")
	if err != nil {
		return fmt.Errorf("需要 ImageMagick 的 `magick` 命令")
	}
	args := append([]string{"montage"}, pngPaths...)
	// -depth 8：montage 默认输出 16 位 PNG，体积翻倍却没有可见差别，
	// 而这些图会被嵌进 am 二进制。
	args = append(args, "-thumbnail", geometry, "-tile", tile, "-geometry", geometry+"+10+10",
		"-background", background, "-depth", "8", outputPath)
	if out, err := exec.Command(magick, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ImageMagick 生成 contact sheet 失败: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DecompressFonts 把 WOFF2 字体解成 TTF 放进一个临时目录，返回目录与清理函数。
//
// 项目字体一律是 WOFF2（给无头 Chrome 用），但常见的 fontconfig / FreeType
// 构建不带 brotli，读不了 WOFF2：直接把字体目录交给 rsvg-convert，示例图会
// 静默回退到系统字体，像素风、衬线、手写风的示例图就全画错了。
func DecompressFonts(woff2Paths []string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "am-fonts-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if len(woff2Paths) == 0 {
		return dir, cleanup, nil
	}
	fonttools, err := exec.LookPath("fonttools")
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("需要 `fonttools` 命令把 WOFF2 字体解给 rsvg-convert（pip install fonttools brotli）")
	}
	for _, path := range woff2Paths {
		target := filepath.Join(dir, strings.TrimSuffix(filepath.Base(path), ".woff2")+".ttf")
		if out, err := exec.Command(fonttools, "ttLib.woff2", "decompress", "-o", target, path).CombinedOutput(); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("解开字体 %s 失败: %w: %s", filepath.Base(path), err, strings.TrimSpace(string(out)))
		}
	}
	return dir, cleanup, nil
}

// quantizeThreshold 以上的示例图量化到 256 色。示例图会被嵌进 am 二进制，
// 纸纹、噪点这类风格的原始 PNG 单张就上 1MB；带抖动的 256 色在示例图的
// 观感上看不出区别，体积约为原来的三分之一。
const quantizeThreshold = 400 * 1024

// Compact 把过大的 PNG 就地量化为 256 色；小图原样保留。
func Compact(pngPath string) error {
	info, err := os.Stat(pngPath)
	if err != nil {
		return err
	}
	if info.Size() <= quantizeThreshold {
		return nil
	}
	magick, err := exec.LookPath("magick")
	if err != nil {
		return fmt.Errorf("需要 ImageMagick 的 `magick` 命令")
	}
	if out, err := exec.Command(magick, pngPath, "-dither", "FloydSteinberg", "-colors", "256", "PNG8:"+pngPath).CombinedOutput(); err != nil {
		return fmt.Errorf("压缩 %s 失败: %w: %s", filepath.Base(pngPath), err, strings.TrimSpace(string(out)))
	}
	return nil
}
