// Package concat 把一组镜头产物按顺序拼成静音母版。
//
// 对应 PROMPT-PRODUCTION.md 第八阶段。放进二进制而不是留给渲染 agent 现场拼
// ffmpeg 命令，图的是这一句：
//
//	「规范化只用于编码、像素格式、色彩和封装统一，不得承担语义重定时。
//	 禁止通过 setpts 或改变帧率让镜头追赶配音；源时间轴有误时必须回到镜头工程重渲染。」
//
// 提示词里的禁令可以被忽略，不存在的能力不能。所以这里根本不提供重定时：
// 分辨率或帧率对不上时直接失败并要求重渲染，而不是悄悄缩放或改帧率。
package concat

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/envutil"
	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
)

// Target 是母版的目标规格。
type Target struct {
	WidthPx     int
	HeightPx    int
	FPS         int
	Codec       string
	PixelFormat string
}

// DefaultCodec 与 DefaultPixelFormat 来自 PROMPT 第八阶段的检查表。
const (
	DefaultCodec       = "h264"
	DefaultPixelFormat = "yuv420p"
)

// Input 是参与拼接的一个片段。
type Input struct {
	// ID 用于错误信息，通常是镜头编号。
	ID string
	// Path 是片段文件路径。
	Path string
}

// Plan 是拼接前的分析结果。
type Plan struct {
	// Blocking 是必须先修复才能拼接的问题，通常意味着要回去重渲染。
	Blocking []string
	// Normalize 列出需要规范化副本的片段。
	Normalize []Input
	// TotalSeconds 是各片段实测时长之和。
	TotalSeconds float64
}

// Analyze 逐个探测片段，判断能否直接 -c copy 拼接，以及哪些需要规范化。
//
// 分辨率与帧率不符归入 Blocking 而不是 Normalize：把 720p 拉伸到 1080 是画质
// 损失而不是格式统一，改帧率则会动到时间轴。这两类只能回镜头工程重渲染。
func Analyze(toolchain mediaprobe.Toolchain, inputs []Input, target Target) (Plan, error) {
	var plan Plan
	if len(inputs) == 0 {
		plan.Blocking = append(plan.Blocking, "没有可拼接的片段")
		return plan, nil
	}
	for _, input := range inputs {
		media, err := toolchain.Probe(input.Path)
		if err != nil {
			return Plan{}, err
		}
		plan.TotalSeconds += media.DurationSeconds
		video := media.Video

		// mediaprobe.Probe 现在也放行纯音频文件（Video 可以是 nil）：观察阶段
		// 不再替判定阶段拦掉这种输入。母版拼接的输入本该都是有画面的镜头产物，
		// 混进一个没有视频流的文件说明上游渲染出了错，这里按 Blocking 处理，
		// 而不是对 nil 的 video 取字段导致 panic。
		if video == nil {
			plan.Blocking = append(plan.Blocking, fmt.Sprintf(
				"%s 没有视频流，无法作为母版片段——请回镜头工程重渲染", input.ID))
			continue
		}
		if video.WidthPx != target.WidthPx || video.HeightPx != target.HeightPx {
			plan.Blocking = append(plan.Blocking, fmt.Sprintf(
				"%s 分辨率是 %s，母版要求 %dx%d——缩放会损失画质，请回镜头工程按正确画幅重渲染",
				input.ID, video.Resolution(), target.WidthPx, target.HeightPx))
			continue
		}
		if target.FPS > 0 && !sameFPS(video.FPS(), float64(target.FPS)) {
			plan.Blocking = append(plan.Blocking, fmt.Sprintf(
				"%s 帧率是 %s，母版要求 %d——改帧率会动到时间轴，请回镜头工程重渲染",
				input.ID, formatFPS(video.FPS()), target.FPS))
			continue
		}
		if media.Audio != nil {
			plan.Blocking = append(plan.Blocking, fmt.Sprintf(
				"%s 带有 %s 音轨，静音母版不允许有音轨", input.ID, media.Audio.Codec))
			continue
		}
		// 只有编码和像素格式的差异才靠规范化统一。
		if !strings.EqualFold(video.Codec, target.Codec) || !strings.EqualFold(video.PixelFormat, target.PixelFormat) {
			plan.Normalize = append(plan.Normalize, input)
		}
	}
	return plan, nil
}

func sameFPS(actual, want float64) bool {
	diff := actual - want
	return diff < 0.02 && diff > -0.02
}

func formatFPS(value float64) string {
	if value == float64(int(value)) {
		return strconv.Itoa(int(value))
	}
	return strconv.FormatFloat(value, 'f', 3, 64)
}

// Runner 执行拼接。
type Runner struct {
	toolchain mediaprobe.Toolchain
	ffmpeg    string
	env       []string
}

// NewRunner 解析 ffmpeg 并返回可用的执行器。
func NewRunner(env map[string]string, toolchain mediaprobe.Toolchain) (Runner, error) {
	ffmpeg, err := envutil.LookPath("ffmpeg", env["PATH"])
	if err != nil {
		return Runner{}, err
	}
	return Runner{toolchain: toolchain, ffmpeg: ffmpeg, env: envutil.EnvList(env)}, nil
}

// Normalize 生成一个只统一编码、像素格式与封装的副本，保留原始文件。
//
// 显式带上 -fps_mode passthrough 与不设 -r：任何隐式的帧率协商都可能悄悄增删帧，
// 那正是这个函数不该做的事。不用旧写法 -vsync：它在 ffmpeg 5.1 起被标为弃用、
// 8.x 已删除，新版 ffmpeg 会直接报 Unrecognized option 'vsync'，拼接前的规范化
// 整个失败。-fps_mode 自 5.1 起可用。
func (r Runner) Normalize(ctx context.Context, source, dest string, target Target) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	codec := "libx264"
	if !strings.EqualFold(target.Codec, "h264") {
		return fmt.Errorf("规范化目前只支持 h264，收到 %s", target.Codec)
	}
	cmd := exec.CommandContext(ctx, r.ffmpeg, "-v", "error", "-y", "-i", source,
		"-c:v", codec, "-pix_fmt", target.PixelFormat,
		"-fps_mode", "passthrough", "-an",
		"-movflags", "+faststart", dest)
	cmd.Env = r.env
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("规范化失败：%s（%s）", source, firstLine(string(output)))
	}
	return nil
}

// Concat 按给定顺序把片段拼成 dest，全程 -c copy，不重新编码。
//
// 用 concat demuxer 而不是 concat filter：后者会重新编码，把已经规范化好的
// 片段再解一遍又编一遍，既慢又掉画质。
func (r Runner) Concat(ctx context.Context, paths []string, dest string) error {
	if len(paths) == 0 {
		return fmt.Errorf("没有可拼接的片段")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	// 用 CreateTemp 拿一个进程内唯一的清单文件名：固定文件名在同一目录并发跑
	// 多个 Concat 时会互相覆盖对方还没读完的清单（与 internal/dialogue/audio.go
	// 的 Concat 同样的问题、同样的修法）。
	listFile, err := os.CreateTemp(filepath.Dir(dest), ".am-concat-list-*.txt")
	if err != nil {
		return err
	}
	listPath := listFile.Name()
	if err := listFile.Close(); err != nil {
		os.Remove(listPath)
		return err
	}
	defer os.Remove(listPath)

	var list strings.Builder
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		// concat demuxer 的清单里单引号要转义成 '\'' ，否则路径含单引号会截断。
		list.WriteString("file '" + strings.ReplaceAll(absolute, "'", `'\''`) + "'\n")
	}
	if err := fsutil.AtomicWrite(listPath, []byte(list.String()), 0o644); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, r.ffmpeg, "-v", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-c", "copy", "-movflags", "+faststart", dest)
	cmd.Env = r.env
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("拼接失败：%s（%s）", dest, firstLine(string(output)))
	}
	return nil
}

func firstLine(value string) string {
	trimmed := strings.TrimSpace(value)
	if index := strings.IndexByte(trimmed, '\n'); index >= 0 {
		trimmed = trimmed[:index]
	}
	if len(trimmed) > 200 {
		trimmed = trimmed[:200]
	}
	if trimmed == "" {
		return "无输出"
	}
	return trimmed
}
