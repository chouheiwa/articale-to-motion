package dialogue

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/envutil"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
)

func newTestRunner(t *testing.T) (Runner, mediaprobe.Toolchain) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("需要 ffmpeg")
	}
	env := envutil.EnvMap()
	toolchain, err := mediaprobe.New(env)
	if err != nil {
		t.Skip("需要 ffprobe")
	}
	runner, err := NewRunner(env, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	return runner, toolchain
}

// tone 生成一段指定秒数的测试音频，格式故意与目标格式不同，
// 用来验证 Normalize 真的在统一格式。
func tone(t *testing.T, dir string, name string, seconds float64) string {
	t.Helper()
	path := filepath.Join(dir, name)
	cmd := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration="+trimSeconds(seconds),
		"-ar", "22050", "-ac", "2", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成测试音频失败：%v（%s）", err, out)
	}
	return path
}

func TestSilenceHasExactDuration(t *testing.T) {
	runner, toolchain := newTestRunner(t)
	dest := filepath.Join(t.TempDir(), "gap.wav")
	if err := runner.Silence(context.Background(), dest, 250); err != nil {
		t.Fatal(err)
	}
	media, err := toolchain.Probe(dest)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(media.DurationSeconds-0.25) > 0.002 {
		t.Errorf("静音时长 = %v，期望 0.25", media.DurationSeconds)
	}
	if media.Audio == nil || media.Audio.SampleRate != 48000 || media.Audio.Channels != 1 {
		t.Errorf("静音格式 = %+v，期望 48000Hz 单声道", media.Audio)
	}
}

func TestNormalizeUnifiesFormat(t *testing.T) {
	runner, toolchain := newTestRunner(t)
	dir := t.TempDir()
	source := tone(t, dir, "src.wav", 1.0)
	dest := filepath.Join(dir, "norm.wav")
	if err := runner.Normalize(context.Background(), source, dest); err != nil {
		t.Fatal(err)
	}
	media, err := toolchain.Probe(dest)
	if err != nil {
		t.Fatal(err)
	}
	if media.Audio == nil || media.Audio.SampleRate != 48000 || media.Audio.Channels != 1 {
		t.Errorf("规范化后 = %+v，期望 48000Hz 单声道", media.Audio)
	}
}

func TestConcatDurationIsSumOfParts(t *testing.T) {
	runner, toolchain := newTestRunner(t)
	dir := t.TempDir()
	var parts []string
	for i, seconds := range []float64{1.0, 0.5} {
		source := tone(t, dir, "src"+string(rune('a'+i))+".wav", seconds)
		dest := filepath.Join(dir, "norm"+string(rune('a'+i))+".wav")
		if err := runner.Normalize(context.Background(), source, dest); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, dest)
	}
	gap := filepath.Join(dir, "gap.wav")
	if err := runner.Silence(context.Background(), gap, 200); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "voice.wav")
	if err := runner.Concat(context.Background(), []string{parts[0], gap, parts[1]}, out); err != nil {
		t.Fatal(err)
	}
	media, err := toolchain.Probe(out)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(media.DurationSeconds-1.7) > 0.005 {
		t.Errorf("拼接总时长 = %v，期望 1.7", media.DurationSeconds)
	}
}

// Concat 假定输入已经统一格式，自己不转码；格式不一致时 concat demuxer 不会
// 报错，只会静默拼出错误的时长。这里故意跳过 Normalize，直接把两段规格不符
// 目标格式（48000Hz/单声道）的源文件交给 Concat，断言拿到的是错误而不是一个
// 时长不对的产物，并且错误信息能定位到具体文件。
func TestConcatRejectsMismatchedFormat(t *testing.T) {
	runner, _ := newTestRunner(t)
	dir := t.TempDir()
	a := tone(t, dir, "a.wav", 1.0)
	b := tone(t, dir, "b.wav", 1.0)
	out := filepath.Join(dir, "voice.wav")
	err := runner.Concat(context.Background(), []string{a, b}, out)
	if err == nil {
		t.Fatal("格式不一致应当报错，而不是静默拼接出错误时长")
	}
	if !strings.Contains(err.Error(), a) {
		t.Errorf("错误信息应当点名具体文件 %s：%v", a, err)
	}
}
