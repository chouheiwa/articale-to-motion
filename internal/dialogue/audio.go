package dialogue

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

// 全部片段统一到这一种格式后再 -c copy 拼接，样本级精确。
// 格式不一致时 concat demuxer 会产出时长不可预期的结果，那正是漂移断言想拦的东西。
const (
	sampleRate = 48000
	channels   = 1
	codec      = "pcm_s16le"
)

// Runner 是音频装配用的 ffmpeg 执行器。
//
// 不复用 internal/concat：那个包是视频母版专用的，Normalize 写死 -an 会丢音轨，
// Concat 带 -movflags +faststart 是 MP4 私有选项，wav muxer 上 ffmpeg 直接报错。
// 这里沿用它的两条做法——先统一格式再 -c copy、用 concat demuxer 而不是 filter。
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

// trimSeconds 把秒数格式化成 ffmpeg 参数用的字符串，保留三位小数（毫秒精度）。
func trimSeconds(seconds float64) string {
	return strconv.FormatFloat(seconds, 'f', 3, 64)
}

func (r Runner) run(ctx context.Context, what string, args ...string) error {
	cmd := exec.CommandContext(ctx, r.ffmpeg, args...)
	cmd.Env = r.env
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s失败：%w（%s）", what, err, firstLine(string(output)))
	}
	return nil
}

// Normalize 把一段 TTS 音频转成统一格式，保留原文件。
func (r Runner) Normalize(ctx context.Context, source, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return r.run(ctx, "规范化 "+filepath.Base(source), "-v", "error", "-y", "-i", source,
		"-vn", "-ar", strconv.Itoa(sampleRate), "-ac", strconv.Itoa(channels), "-c:a", codec, dest)
}

// Silence 生成一段精确毫秒数的静音，格式与 Normalize 一致。
func (r Runner) Silence(ctx context.Context, dest string, ms int) error {
	if ms <= 0 {
		return fmt.Errorf("静音时长必须为正，收到 %d ms", ms)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	source := fmt.Sprintf("anullsrc=r=%d:cl=mono", sampleRate)
	return r.run(ctx, "生成静音", "-v", "error", "-y", "-f", "lavfi", "-i", source,
		"-t", trimSeconds(float64(ms)/1000), "-c:a", codec, dest)
}

// Concat 按顺序拼接已规范化的片段，全程 -c copy。
//
// 契约：全部输入必须已经是 Normalize/Silence 产出的统一格式——48000Hz、单
// 声道、pcm_s16le。Concat 自己不转码，格式不一致时 concat demuxer 不会报
// 错，只会静默产出时长不可预期的结果（用哪个片段的采样率/声道数取决于
// ffmpeg 内部实现细节，调用方看不出来），那正是漂移断言要拦的东西。所以
// Concat 在拼接前会对每个输入探测一次格式，不满足直接返回中文错误并点名
// 具体文件，而不是把这条前提留给调用方自己遵守的约定。
func (r Runner) Concat(ctx context.Context, paths []string, dest string) error {
	if len(paths) == 0 {
		return fmt.Errorf("没有可拼接的音频片段")
	}
	for _, path := range paths {
		if err := r.checkFormat(path); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	// 用 CreateTemp 拿一个进程内唯一的清单文件名：固定文件名在同一目录并发跑
	// 多个 Concat 时会互相覆盖对方还没读完的清单。
	listFile, err := os.CreateTemp(filepath.Dir(dest), ".am-dialogue-list-*.txt")
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
		// concat demuxer 的清单里单引号要转义成 '\''，否则路径含单引号会截断。
		list.WriteString("file '" + strings.ReplaceAll(absolute, "'", `'\''`) + "'\n")
	}
	if err := fsutil.AtomicWrite(listPath, []byte(list.String()), 0o644); err != nil {
		return err
	}
	return r.run(ctx, "拼接音频", "-v", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", dest)
}

// checkFormat 校验 path 是否符合 Normalize/Silence 产出的统一格式。
// 保证「参与拼接的片段格式一致」这条前提本身不能只靠约定，得有代码守着。
func (r Runner) checkFormat(path string) error {
	media, err := r.toolchain.Probe(path)
	if err != nil {
		return err
	}
	if media.Audio == nil {
		return fmt.Errorf("%s 没有音轨，无法参与拼接", path)
	}
	if media.Audio.SampleRate != sampleRate {
		return fmt.Errorf("%s 采样率是 %dHz，拼接要求 %dHz：请先用 Normalize 统一格式",
			path, media.Audio.SampleRate, sampleRate)
	}
	if media.Audio.Channels != channels {
		return fmt.Errorf("%s 声道数是 %d，拼接要求 %d：请先用 Normalize 统一格式",
			path, media.Audio.Channels, channels)
	}
	if !strings.EqualFold(media.Audio.Codec, codec) {
		return fmt.Errorf("%s 音频编码是 %s，拼接要求 %s：请先用 Normalize 统一格式",
			path, media.Audio.Codec, codec)
	}
	return nil
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}
