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
func (r Runner) Concat(ctx context.Context, paths []string, dest string) error {
	if len(paths) == 0 {
		return fmt.Errorf("没有可拼接的音频片段")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	listPath := filepath.Join(filepath.Dir(dest), ".am-dialogue-list.txt")
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
	defer os.Remove(listPath)
	return r.run(ctx, "拼接音频", "-v", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", dest)
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}
