package dialogue

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/chouheiwa/articale-to-motion/internal/envutil"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
)

// DriftToleranceSeconds 是实测总时长与推算总时长的最大容许差。
//
// 50ms 是一帧半（30fps）：再大就是肉眼可见的音画错位，再小则会被 ffmpeg
// 各版本在容器时长写入上的正常抖动误伤。
const DriftToleranceSeconds = 0.050

// Options 是 Assemble 的输入参数。
type Options struct {
	// Root 是项目根目录。
	Root string
	// PlanPath 是 plan.json 路径。
	PlanPath string
	// ExpectTotalSeconds 是可选的外部期望总时长；为 0 时只做内部一致性断言。
	ExpectTotalSeconds float64
}

// Assemble 读 plan.json，把分段音频装配成 voice.wav、SRT 和 dialogue.json。
//
// 漂移断言是整条链路唯一能发现「时间轴算错了」的地方：算错的后果是成片画面
// 正常、只是嘴和字对不上，不会报错。所以这里不做任何容差之外的补偿，超差
// 直接失败。
func Assemble(ctx context.Context, opts Options) (Result, error) {
	body, err := os.ReadFile(opts.PlanPath)
	if err != nil {
		return Result{}, fmt.Errorf("读取 plan 失败：%w", err)
	}
	var plan Plan
	if err := json.Unmarshal(body, &plan); err != nil {
		return Result{}, fmt.Errorf("解析 plan 失败：%w", err)
	}

	env := envutil.EnvMap()
	toolchain, err := mediaprobe.New(env)
	if err != nil {
		return Result{}, err
	}
	runner, err := NewRunner(env, toolchain)
	if err != nil {
		return Result{}, err
	}

	workDir := filepath.Join(opts.Root, "production", "audio", "assembled")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return Result{}, err
	}

	measured := make([]float64, len(plan.Segments))
	var pieces []string
	for i, segment := range plan.Segments {
		source := filepath.Join(opts.Root, segment.Audio)
		normalized := filepath.Join(workDir, fmt.Sprintf("seg-%03d.wav", i+1))
		if err := runner.Normalize(ctx, source, normalized); err != nil {
			return Result{}, err
		}
		media, err := toolchain.Probe(normalized)
		if err != nil {
			return Result{}, err
		}
		measured[i] = media.DurationSeconds
		pieces = append(pieces, normalized)

		if segment.GapAfterMs > 0 && i < len(plan.Segments)-1 {
			gap := filepath.Join(workDir, fmt.Sprintf("gap-%03d.wav", i+1))
			if err := runner.Silence(ctx, gap, segment.GapAfterMs); err != nil {
				return Result{}, err
			}
			pieces = append(pieces, gap)
		}
	}

	result, err := Rebuild(plan, measured)
	if err != nil {
		return Result{}, err
	}

	voicePath := filepath.Join(opts.Root, "production", "audio", "voice.wav")
	if err := runner.Concat(ctx, pieces, voicePath); err != nil {
		return Result{}, err
	}
	final, err := toolchain.Probe(voicePath)
	if err != nil {
		return Result{}, err
	}

	// Rebuild 的 cursor 对每段累加 duration + gap，TotalSeconds 取的是末段的
	// EndSeconds，因此它含全部段间间隔、不含尾部间隔——而拼接时也只在
	// i < len-1 时插入间隔，两边口径一致，直接比较即可。
	expected := result.TotalSeconds()
	if drift := math.Abs(final.DurationSeconds - expected); drift > DriftToleranceSeconds {
		return Result{}, fmt.Errorf("时间轴漂移 %.3f 秒超出容差 %.3f：实测 voice.wav %.3f 秒，按分段推算 %.3f 秒",
			drift, DriftToleranceSeconds, final.DurationSeconds, expected)
	}
	if opts.ExpectTotalSeconds > 0 {
		if drift := math.Abs(final.DurationSeconds - opts.ExpectTotalSeconds); drift > DriftToleranceSeconds {
			return Result{}, fmt.Errorf("时间轴漂移 %.3f 秒超出容差 %.3f：实测 %.3f 秒，期望 %.3f 秒",
				drift, DriftToleranceSeconds, final.DurationSeconds, opts.ExpectTotalSeconds)
		}
	}

	if err := WriteSRT(filepath.Join(opts.Root, "transcription-production.srt"), result.Lines); err != nil {
		return Result{}, err
	}
	if err := result.WriteJSON(filepath.Join(opts.Root, "production", "dialogue.json")); err != nil {
		return Result{}, err
	}
	return result, nil
}
