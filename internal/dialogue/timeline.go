package dialogue

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
)

// isFinite 检查浮点数是否有限（既不是 NaN 也不是 ±Inf）。
func isFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// Rebuild 用实测时长把各段拼成全局时间轴。
//
// 段内行时间按 实测时长 / 声明时长 等比拉伸：TTS 声明的段内时间戳与实际
// 音频长度常有几十毫秒出入，不拉伸就会一段段累积到片尾。
func Rebuild(plan Plan, measured []float64) (Result, error) {
	if plan.Schema != SchemaVersion {
		return Result{}, fmt.Errorf("plan schema 必须是 %s，收到 %q", SchemaVersion, plan.Schema)
	}
	if len(plan.Segments) == 0 {
		return Result{}, fmt.Errorf("plan 里没有任何分段")
	}
	if len(measured) != len(plan.Segments) {
		return Result{}, fmt.Errorf("实测时长数量 %d 与分段数量 %d 不一致", len(measured), len(plan.Segments))
	}

	result := Result{Schema: SchemaVersion}
	cursor := 0.0
	srtIndex := 0
	for i, segment := range plan.Segments {
		if segment.Speaker == "" {
			return Result{}, fmt.Errorf("第 %d 段没有说话人", i+1)
		}
		if segment.GapAfterMs < 0 {
			return Result{}, fmt.Errorf("第 %d 段的间隔不得为负：%d", i+1, segment.GapAfterMs)
		}
		if len(segment.Lines) == 0 {
			return Result{}, fmt.Errorf("第 %d 段没有字幕行", i+1)
		}
		duration := measured[i]
		if !isFinite(duration) {
			return Result{}, fmt.Errorf("第 %d 段的实测时长无效：%v", i+1, duration)
		}
		if duration <= 0 {
			return Result{}, fmt.Errorf("第 %d 段的实测时长无效：%v", i+1, duration)
		}
		declared := segment.Lines[len(segment.Lines)-1].EndSeconds
		if !isFinite(declared) {
			return Result{}, fmt.Errorf("第 %d 段声明的时长无效：%v", i+1, declared)
		}
		if declared <= 0 {
			return Result{}, fmt.Errorf("第 %d 段声明的时长无效：%v", i+1, declared)
		}
		scale := duration / declared

		previousEnd := 0.0
		for j, line := range segment.Lines {
			if !isFinite(line.StartSeconds) {
				return Result{}, fmt.Errorf("第 %d 段第 %d 行的起始时间无效：%v", i+1, j+1, line.StartSeconds)
			}
			if !isFinite(line.EndSeconds) {
				return Result{}, fmt.Errorf("第 %d 段第 %d 行的结束时间无效：%v", i+1, j+1, line.EndSeconds)
			}
			if line.EndSeconds <= line.StartSeconds {
				return Result{}, fmt.Errorf("第 %d 段第 %d 行时间倒挂：[%v, %v]", i+1, j+1, line.StartSeconds, line.EndSeconds)
			}
			if math.Abs(line.StartSeconds-previousEnd) > 1e-9 {
				return Result{}, fmt.Errorf("第 %d 段第 %d 行与上一行不连续：上行止于 %v，本行起于 %v",
					i+1, j+1, previousEnd, line.StartSeconds)
			}
			previousEnd = line.EndSeconds
			srtIndex++
			result.Lines = append(result.Lines, Line{
				SRTIndex:     srtIndex,
				Speaker:      segment.Speaker,
				StartSeconds: cursor + line.StartSeconds*scale,
				EndSeconds:   cursor + line.EndSeconds*scale,
				Text:         line.Text,
			})
		}
		result.Segments = append(result.Segments, Segment{
			Index:        segment.Index,
			Speaker:      segment.Speaker,
			VoiceID:      segment.VoiceID,
			Audio:        segment.Audio,
			StartSeconds: cursor,
			EndSeconds:   cursor + duration,
			GapAfterMs:   segment.GapAfterMs,
		})
		cursor += duration + float64(segment.GapAfterMs)/1000
	}
	return result, nil
}

// WriteSRT 写出标准 SRT。
//
// 刻意不带说话人：SRT 没有说话人字段，塞前缀会进成片字幕，塞注释各家播放器
// 行为不一。说话人映射走 dialogue.json。
func WriteSRT(path string, lines []Line) error {
	if len(lines) == 0 {
		return fmt.Errorf("没有任何字幕行")
	}

	var builder strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&builder, "%d\n%s --> %s\n%s\n",
			line.SRTIndex, clock(line.StartSeconds), clock(line.EndSeconds), line.Text)
		// 最后一行不加空行
		if i < len(lines)-1 {
			builder.WriteString("\n")
		}
	}
	return fsutil.AtomicWrite(path, []byte(builder.String()), 0o644)
}

// clock 把秒转成 SRT 的 HH:MM:SS,mmm。四舍五入到毫秒，不截断：
// 截断会让每一行都提前最多 1ms，累积起来是可见的字幕偏移。
func clock(seconds float64) string {
	total := int64(math.Round(seconds * 1000))
	if total < 0 {
		total = 0
	}
	ms := total % 1000
	total /= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", total/3600, total%3600/60, total%60, ms)
}

// WriteJSON 写出 dialogue.json。
//
// JSON 里不带字幕正文（Text 字段）：字幕正文的唯一真相是 SRT 文件。Line 的 Text
// 被标记为 json:"-"，序列化时自动忽略。JSON 里只有说话人、时间戳和 SRT 索引，
// 便于外部系统进行说话人识别和时间同步。
func (r Result) WriteJSON(path string) error {
	body, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化对话数据失败：%w", err)
	}
	return fsutil.AtomicWrite(path, append(body, '\n'), 0o644)
}
