// Package srt 只做一件事：从字幕文件读出时间跨度。
//
// 不做完整的 SRT 解析，也不关心文本内容——调用方需要的是「第一条字幕开始到
// 最后一条字幕结束」这一个数字，用来核对镜头拆分有没有漏掉或重复覆盖时间轴。
package srt

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// timecodeLine 匹配 "00:00:01,234 --> 00:00:05,678"。
//
// 分隔符同时接受逗号和点号：规范写逗号，但不少工具（尤其是从 VTT 转过来的）
// 产出点号，一律拒绝会让合法字幕被判成格式错误。
var timecodeLine = regexp.MustCompile(
	`^\s*(\d{1,2}):(\d{2}):(\d{2})[,.](\d{1,3})\s*-->\s*(\d{1,2}):(\d{2}):(\d{2})[,.](\d{1,3})`)

// Span 是一份字幕的时间跨度。
type Span struct {
	// StartSeconds 是第一条字幕的开始时间。
	StartSeconds float64
	// EndSeconds 是最后一条字幕的结束时间。
	EndSeconds float64
	// Cues 是解析到的字幕条数。
	Cues int
}

// Seconds 返回跨度长度，即镜头总时长应当匹配的目标值。
func (s Span) Seconds() float64 { return s.EndSeconds - s.StartSeconds }

// ReadSpan 从 SRT 文件读出时间跨度。
//
// 取的是所有时间码里的最小开始与最大结束，而不是第一行和最后一行：
// 字幕条目乱序或时间重叠时，按出现顺序取会得到一个偏小甚至为负的跨度。
func ReadSpan(path string) (Span, error) {
	file, err := os.Open(path)
	if err != nil {
		return Span{}, fmt.Errorf("无法读取字幕：%w", err)
	}
	defer file.Close()

	var span Span
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		match := timecodeLine.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}
		start, err := clockSeconds(match[1], match[2], match[3], match[4])
		if err != nil {
			return Span{}, err
		}
		end, err := clockSeconds(match[5], match[6], match[7], match[8])
		if err != nil {
			return Span{}, err
		}
		if span.Cues == 0 || start < span.StartSeconds {
			span.StartSeconds = start
		}
		if span.Cues == 0 || end > span.EndSeconds {
			span.EndSeconds = end
		}
		span.Cues++
	}
	if err := scanner.Err(); err != nil {
		return Span{}, fmt.Errorf("读取字幕失败：%w", err)
	}
	if span.Cues == 0 {
		return Span{}, fmt.Errorf("字幕里没有任何时间码：%s", path)
	}
	if span.Seconds() <= 0 {
		return Span{}, fmt.Errorf("字幕跨度非正：开始 %.3f 秒、结束 %.3f 秒", span.StartSeconds, span.EndSeconds)
	}
	return span, nil
}

// clockSeconds 把时分秒毫秒转成秒。
func clockSeconds(hours, minutes, seconds, millis string) (float64, error) {
	h, errH := strconv.Atoi(hours)
	m, errM := strconv.Atoi(minutes)
	s, errS := strconv.Atoi(seconds)
	// 毫秒位数不足时要右补零而不是左补：",5" 是 500 毫秒，不是 5 毫秒。
	ms, errMS := strconv.Atoi(millis + strings.Repeat("0", 3-len(millis)))
	if errH != nil || errM != nil || errS != nil || errMS != nil {
		return 0, fmt.Errorf("时间码解析失败：%s:%s:%s,%s", hours, minutes, seconds, millis)
	}
	if m > 59 || s > 59 {
		return 0, fmt.Errorf("时间码的分或秒超出范围：%s:%s:%s,%s", hours, minutes, seconds, millis)
	}
	total := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute +
		time.Duration(s)*time.Second + time.Duration(ms)*time.Millisecond
	return total.Seconds(), nil
}
