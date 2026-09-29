package styleimage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 缺 rsvg-convert 时必须报错并说清原因。
// 退回 ImageMagick 是不可接受的降级：它的内置 SVG 渲染器返回 0 却产出黑底无字的图。
func TestRenderSVGRequiresRsvgConvert(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := RenderSVG("in.svg", "out.png")
	if err == nil {
		t.Fatal("缺少 rsvg-convert 时应报错")
	}
	if !strings.Contains(err.Error(), "rsvg-convert") {
		t.Errorf("错误信息应点名 rsvg-convert，实际：%v", err)
	}
}

func TestRenderSVGInvokesRsvgConvertWithOutputFlag(t *testing.T) {
	bin := t.TempDir()
	work := t.TempDir()
	stub := filepath.Join(bin, "rsvg-convert")
	// 桩把实参记进 args.log，并按 -o 指定的路径产出文件。
	script := "#!/bin/sh\necho \"$@\" > \"" + filepath.Join(work, "args.log") + "\"\n" +
		"while [ $# -gt 0 ]; do\n  if [ \"$1\" = \"-o\" ]; then shift; : > \"$1\"; exit 0; fi\n  shift\ndone\nexit 1\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	png := filepath.Join(work, "out.png")
	if err := RenderSVG(filepath.Join(work, "in.svg"), png); err != nil {
		t.Fatalf("RenderSVG: %v", err)
	}
	if _, err := os.Stat(png); err != nil {
		t.Fatalf("未产出 PNG: %v", err)
	}
	args, err := os.ReadFile(filepath.Join(work, "args.log"))
	if err != nil {
		t.Fatal(err)
	}
	// -background 会让 ImageMagick 落进内置渲染器；这里确认没有沿用那套参数。
	if strings.Contains(string(args), "-background") {
		t.Errorf("不应传 -background，实际参数：%s", args)
	}
	if !strings.Contains(string(args), "-o") {
		t.Errorf("缺少 -o 参数：%s", args)
	}
}

func TestRenderSVGReportsRendererFailure(t *testing.T) {
	bin := t.TempDir()
	stub := filepath.Join(bin, "rsvg-convert")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho 'boom' >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	err := RenderSVG("in.svg", "out.png")
	if err == nil {
		t.Fatal("渲染器退出码非零时应报错")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("错误应带上渲染器输出，实际：%v", err)
	}
}

func TestContactSheetRequiresImageMagick(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := ContactSheet([]string{"a.png"}, "out.png", "405x540", "#D5DEEB", "4x1"); err == nil {
		t.Fatal("缺少 magick 时应报错")
	}
}

func TestCheckRendererRequiresRsvgConvert(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := CheckRenderer(); err == nil || !strings.Contains(err.Error(), "rsvg-convert") {
		t.Fatalf("缺少 rsvg-convert 时应报错并点名，实际 %v", err)
	}
}

// 临时 fontconfig 必须引入系统默认配置、追加字体目录，并对路径做 XML 转义。
func TestFontconfigWithIncludesSystemConfigAndEscapesDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a&b<c>")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := fontconfigWith([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{`<include ignore_missing="yes">/etc/fonts/fonts.conf</include>`, "a&amp;b&lt;c&gt;", "<cachedir>"} {
		if !strings.Contains(text, want) {
			t.Errorf("fonts.conf 缺少 %q：\n%s", want, text)
		}
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cleanup 应删除临时配置")
	}
}

func TestDecompressFontsWithoutFontsNeedsNoTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir, cleanup, err := DecompressFonts(nil)
	if err != nil {
		t.Fatalf("没有字体时不应依赖 fonttools：%v", err)
	}
	defer cleanup()
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatal("应返回一个可用的空目录")
	}
}

func TestDecompressFontsRequiresFonttools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, _, err := DecompressFonts([]string{"x.woff2"}); err == nil || !strings.Contains(err.Error(), "fonttools") {
		t.Fatalf("缺少 fonttools 时应报错并点名，实际 %v", err)
	}
}

// 用桩代替 fonttools：检查调用参数与产物命名（x.woff2 → x.ttf）。
func TestDecompressFontsNamesOutputAfterSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub is Unix-only")
	}
	bin := t.TempDir()
	stub := "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = \"-o\" ]; then shift; : > \"$1\"; exit 0; fi\n  shift\ndone\nexit 1\n"
	os.WriteFile(filepath.Join(bin, "fonttools"), []byte(stub), 0o755)
	t.Setenv("PATH", bin)
	dir, cleanup, err := DecompressFonts([]string{"/fonts/pixel-400.woff2"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(filepath.Join(dir, "pixel-400.ttf")); err != nil {
		t.Fatalf("应产出 pixel-400.ttf：%v", err)
	}
}

func TestDecompressFontsReportsToolFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub is Unix-only")
	}
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "fonttools"), []byte("#!/bin/sh\necho boom >&2\nexit 2\n"), 0o755)
	t.Setenv("PATH", bin)
	if _, _, err := DecompressFonts([]string{"/fonts/a.woff2"}); err == nil || !strings.Contains(err.Error(), "a.woff2") {
		t.Fatalf("解字体失败应点名文件，实际 %v", err)
	}
}

func TestCompactLeavesSmallFilesUntouched(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "small.png")
	os.WriteFile(path, []byte("tiny"), 0o644)
	if err := Compact(path); err != nil {
		t.Fatalf("小图不应调用 magick：%v", err)
	}
	if body, _ := os.ReadFile(path); string(body) != "tiny" {
		t.Fatal("小图内容不应改变")
	}
	if err := Compact(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Fatal("文件不存在应报错")
	}
}

func TestCompactQuantizesLargeFilesInPlace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub is Unix-only")
	}
	bin := t.TempDir()
	// magick <in> ... PNG8:<out>：桩把最后一个参数去掉 PNG8: 前缀后写入。
	stub := "#!/bin/sh\nfor last; do :; done\nout=${last#PNG8:}\necho quantized > \"$out\"\n"
	os.WriteFile(filepath.Join(bin, "magick"), []byte(stub), 0o755)
	t.Setenv("PATH", bin)
	path := filepath.Join(t.TempDir(), "big.png")
	os.WriteFile(path, make([]byte, quantizeThreshold+1), 0o644)
	if err := Compact(path); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(path); strings.TrimSpace(string(body)) != "quantized" {
		t.Fatal("大图应被就地量化")
	}
	t.Setenv("PATH", t.TempDir())
	os.WriteFile(path, make([]byte, quantizeThreshold+1), 0o644)
	if err := Compact(path); err == nil || !strings.Contains(err.Error(), "magick") {
		t.Fatalf("缺少 magick 时应报错，实际 %v", err)
	}
}
