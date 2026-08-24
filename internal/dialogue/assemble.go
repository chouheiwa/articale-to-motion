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

// perSegmentRelativeTolerance 与 perSegmentAbsoluteFloorSeconds 定义单段声明
// 时长（plan 里该段最后一行的 EndSeconds）与实测时长（Normalize 后探测出的
// 音频时长）之间的容差：max(绝对下限, 相对比例 * 声明时长)。
//
// 用相对比例是因为长段和短段的合理抖动不是一个量级；单用绝对值，长段会漏
// 判，短段又太敏感。10% 的选择依据是 Rebuild 注释里说的「TTS 声明的段内
// 时间戳与实际音频长度常有几十毫秒出入」——对一段几秒钟的音频，几十毫秒
// 换算下来大约是百分之一到百分之几，10% 留出了数倍安全边际，不会把这类
// 正常抖动当成异常；但 10% 又远小于「文件拿错」「TTS 输出被截断或重复」
// 这类真实故障的量级（通常是数倍甚至整段丢失/重复），足够拦下来。
// 200ms 的绝对下限是防止极短分段（几百毫秒以内）被相对比例算出一个小于
// 正常抖动本身的容差，反而把合法输入误杀。
const (
	perSegmentRelativeTolerance    = 0.10
	perSegmentAbsoluteFloorSeconds = 0.20
)

// segmentDriftTolerance 返回给定声明时长下，逐段声明 vs 实测校验的容差。
func segmentDriftTolerance(declaredSeconds float64) float64 {
	return math.Max(perSegmentAbsoluteFloorSeconds, declaredSeconds*perSegmentRelativeTolerance)
}

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
// 装配过程有两道漂移防线，各自拦不同的错，互补而非替代：
//
//  1. 逐段声明 vs 实测校验（segmentDriftTolerance）：每段都拿 plan 里该段
//     最后一行的 EndSeconds（TTS agent 自己声明的段内时长）跟 Normalize
//     后实测的音频时长比对，差异超出容差就直接失败并点名具体段号和文件。
//     这道防线拦的是「输入数据本身就是错的」——音频文件拿错、TTS 输出被
//     截断或重复。Rebuild 的等比拉伸会把声明和实测的差异悄悄摊平，如果
//     不在这里拦，一段声明 1 秒、实际 3 秒的音频会被拉伸消化掉，产出画面
//     正常、只是嘴和字对不上的成片，而不会报错。
//
//  2. 总时长漂移断言（DriftToleranceSeconds）：实测拼接产物 voice.wav 的
//     总时长，跟 Rebuild 算出的 TotalSeconds 比对，再跟调用方给的
//     ExpectTotalSeconds（如果非零）比对。这道防线拦的是拼接管道自身的
//     逻辑 bug——比如某段间隔该不该计入 TotalSeconds 算错了。它对「plan
//     声明的时长本身不可信」这类问题是同义反复：expected 和 final 都源自
//     同一批 measured[]，输入数据的错误会被两边一起继承而互相抵消，光靠
//     它拦不住第 1 类问题，所以缺一不可。
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
	// 先清空再建：上一次运行留下的分段/间隔文件不会自然消失（-y 只保证不会
	// 读到旧内容，不会清理多余文件），换成更少段数的 plan 后会一直堆积。
	if err := os.RemoveAll(workDir); err != nil {
		return Result{}, fmt.Errorf("清理装配临时目录失败：%w", err)
	}
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

		// 逐段声明 vs 实测校验：拦「输入数据本身就是错的」，Rebuild 的等比
		// 拉伸不会替我们拦这个，见 Assemble 顶部的文档注释。
		if len(segment.Lines) > 0 {
			declared := segment.Lines[len(segment.Lines)-1].EndSeconds
			if isFinite(declared) && declared > 0 && isFinite(measured[i]) {
				tolerance := segmentDriftTolerance(declared)
				if diff := math.Abs(measured[i] - declared); diff > tolerance {
					return Result{}, fmt.Errorf(
						"第 %d 段时间轴漂移：%s 声明 %.3f 秒，实测 %.3f 秒，差 %.3f 秒超出容差 %.3f 秒"+
							"（可能是音频文件拿错，或 TTS 输出被截断/重复）",
						i+1, segment.Audio, declared, measured[i], diff, tolerance)
				}
			}
		}

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
