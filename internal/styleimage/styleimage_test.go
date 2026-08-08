package styleimage

import (
	"os"
	"path/filepath"
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
	if err := ContactSheet([]string{"a.png"}, "out.png", "405x540", "#D5DEEB"); err == nil {
		t.Fatal("缺少 magick 时应报错")
	}
}
