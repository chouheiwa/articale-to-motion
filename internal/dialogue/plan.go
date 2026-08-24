// Package dialogue 把分说话人合成的音频段装配成一条时间线。
//
// 为什么这一步在 Go 而不在编排 agent：分段 TTS 只给得到段内相对时间戳，
// 拼成全局时间轴要逐段累加偏移。算错的后果是成片画面正常、只是嘴和字对不上，
// 不会报错——这类会静默坏掉的机械计算一律收进二进制。
package dialogue

// SchemaVersion 是 plan.json 与 dialogue.json 共用的 schema 标识。
const SchemaVersion = "cast-dialogue/v1"

// PlanLine 是一条字幕行，时间是相对本段起点的。
type PlanLine struct {
	Text         string  `json:"text"`
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
}

// PlanSegment 是一次 TTS 调用的产物：同一说话人的连续一段。
type PlanSegment struct {
	Index      int        `json:"index"`
	Speaker    string     `json:"speaker"`
	VoiceID    string     `json:"voiceId"`
	Audio      string     `json:"audio"`
	GapAfterMs int        `json:"gapAfterMs"`
	Lines      []PlanLine `json:"lines"`
}

type Plan struct {
	Schema   string        `json:"schema"`
	Segments []PlanSegment `json:"segments"`
}

// Segment 是装配后的段，时间已是全局的。
type Segment struct {
	Index        int     `json:"index"`
	Speaker      string  `json:"speaker"`
	VoiceID      string  `json:"voiceId"`
	Audio        string  `json:"audio"`
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
	GapAfterMs   int     `json:"gapAfterMs"`
}

// Line 是装配后的字幕行。Text 不进 dialogue.json：字幕正文的唯一真相是 SRT。
type Line struct {
	SRTIndex     int     `json:"srtIndex"`
	Speaker      string  `json:"speaker"`
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
	Text         string  `json:"-"`
}

type Result struct {
	Schema   string    `json:"schema"`
	Segments []Segment `json:"segments"`
	Lines    []Line    `json:"lines"`
}

// TotalSeconds 是最后一段的结束时间，不含尾部间隔。
func (r Result) TotalSeconds() float64 {
	if len(r.Segments) == 0 {
		return 0
	}
	return r.Segments[len(r.Segments)-1].EndSeconds
}
