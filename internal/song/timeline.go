package song

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
)

type Word struct {
	Text      string  `json:"text"`
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Estimated bool    `json:"estimated,omitempty"`
}
type Line struct {
	ID            string  `json:"id"`
	SourceLine    int     `json:"source_line"`
	Text          string  `json:"text"`
	AlignmentText string  `json:"alignment_text,omitempty"`
	Start         float64 `json:"start"`
	End           float64 `json:"end"`
	Words         []Word  `json:"words,omitempty"`
}
type Interval struct {
	Kind  string  `json:"kind"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}
type Timeline struct {
	Schema        int        `json:"schema"`
	CandidateID   string     `json:"candidate_id"`
	AudioSHA      string     `json:"audio_sha256"`
	LyricsSHA     string     `json:"lyrics_sha256"`
	Duration      float64    `json:"duration_seconds"`
	Source        string     `json:"source"`
	Reviewed      bool       `json:"reviewed"`
	Lines         []Line     `json:"lines"`
	Beats         []float64  `json:"beats,omitempty"`
	Instrumentals []Interval `json:"instrumentals,omitempty"`
	Warnings      []string   `json:"warnings,omitempty"`
}

func (t Timeline) Frames(fps int) int { return int(math.Ceil(t.Duration*float64(fps) - 1e-9)) }
func (t Timeline) Problems() []string {
	var p []string
	if t.Schema != 1 || !finite(t.Duration) || t.Duration <= 0 {
		p = append(p, "无效时间轴版本或时长")
	}
	if len(t.Lines) == 0 {
		p = append(p, "没有已对齐歌词")
	}
	seen := map[string]bool{}
	end := 0.0
	for _, l := range t.Lines {
		if l.ID == "" || seen[l.ID] {
			p = append(p, "歌词 ID 缺失或重复")
		}
		seen[l.ID] = true
		if strings.TrimSpace(l.Text) == "" || !finite(l.Start) || !finite(l.End) || l.Start < end-1e-6 || l.End <= l.Start || l.End > t.Duration+1e-6 {
			p = append(p, "歌词缺失、倒序、重叠或越界："+l.ID)
		}
		end = l.End
		wEnd := l.Start
		for _, w := range l.Words {
			if w.Estimated || !finite(w.Start) || !finite(w.End) || w.Start < wEnd-1e-6 || w.End <= w.Start || w.End > l.End+1e-6 || strings.TrimSpace(w.Text) == "" {
				p = append(p, "字词时间不可靠："+l.ID)
			}
			wEnd = w.End
		}
	}
	last := -1.0
	for _, b := range t.Beats {
		if !finite(b) || b < 0 || b >= t.Duration || b <= last {
			p = append(p, "节拍倒序或越界")
		}
		last = b
	}
	return p
}
func (t Timeline) Slice(start, duration float64) Timeline {
	out := t
	out.Duration = duration
	out.Lines = nil
	out.Beats = nil
	out.Instrumentals = nil
	end := start + duration
	for _, l := range t.Lines {
		if l.End <= start || l.Start >= end {
			continue
		}
		c := l
		c.Start = math.Max(l.Start, start) - start
		c.End = math.Min(l.End, end) - start
		c.Words = nil
		for _, w := range l.Words {
			if w.End <= start || w.Start >= end {
				continue
			}
			w.Start = math.Max(w.Start, start) - start
			w.End = math.Min(w.End, end) - start
			c.Words = append(c.Words, w)
		}
		out.Lines = append(out.Lines, c)
	}
	for _, b := range t.Beats {
		if b >= start && b < end {
			out.Beats = append(out.Beats, b-start)
		}
	}
	for _, g := range t.Instrumentals {
		if g.End <= start || g.Start >= end {
			continue
		}
		g.Start = math.Max(g.Start, start) - start
		g.End = math.Min(g.End, end) - start
		out.Instrumentals = append(out.Instrumentals, g)
	}
	return out
}
func (t *Timeline) gaps() {
	t.Instrumentals = nil
	cursor := 0.0
	for i, l := range t.Lines {
		if l.Start > cursor {
			kind := "interlude"
			if i == 0 {
				kind = "intro"
			}
			t.Instrumentals = append(t.Instrumentals, Interval{kind, cursor, l.Start})
		}
		cursor = l.End
	}
	if cursor < t.Duration {
		t.Instrumentals = append(t.Instrumentals, Interval{"outro", cursor, t.Duration})
	}
}
func checkSelection(root string, t Timeline) error {
	c, e := Selected(root)
	if e != nil {
		return e
	}
	if t.CandidateID != c.ID || t.AudioSHA != c.AudioSHA || t.LyricsSHA != c.LyricsSHA || math.Abs(t.Duration-c.Duration) > 1e-6 {
		return fmt.Errorf("时间轴已过期：与选定音频或歌词不一致")
	}
	dir, _ := CandidateDir(root, c.ID)
	b, e := os.ReadFile(filepath.Join(dir, c.Lyrics))
	if e != nil {
		return e
	}
	lines := LyricLines(string(b))
	if len(lines) != len(t.Lines) {
		return fmt.Errorf("对齐歌词行数不匹配：需要 %d 行，得到 %d 行", len(lines), len(t.Lines))
	}
	for i, l := range lines {
		if strings.TrimSpace(l) != strings.TrimSpace(t.Lines[i].Text) {
			return fmt.Errorf("第 %d 行歌词与选定文本不一致", i+1)
		}
	}
	return nil
}

// LyricLines excludes structure tags, never deduplicates repeated choruses.
func LyricLines(text string) []string {
	var out []string
	for _, s := range strings.Split(text, "\n") {
		s = strings.TrimSpace(s)
		if s == "" || (strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
			continue
		}
		out = append(out, s)
	}
	return out
}
func SaveTimeline(root string, t Timeline) error {
	if e := checkSelection(root, t); e != nil {
		return e
	}
	if p := t.Problems(); len(p) > 0 {
		return fmt.Errorf("时间轴不合格：%s", strings.Join(p, "；"))
	}
	t.gaps()
	p, e := LocalPath(root, Store+"/timeline.json")
	if e != nil {
		return e
	}
	// The timeline is the commit marker; derived files are written first.
	var s strings.Builder
	for i, l := range t.Lines {
		fmt.Fprintf(&s, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(l.Start), srtTime(l.End), l.Text)
	}
	if e = fsutil.AtomicWrite(filepath.Join(filepath.Dir(p), "lyrics.srt"), []byte(s.String()), 0644); e != nil {
		return e
	}
	return WriteJSON(p, t)
}
func LoadTimeline(root string) (Timeline, error) {
	var t Timeline
	if _, e := LoadConfig(root); e != nil {
		return t, e
	}
	p, e := LocalPath(root, Store+"/timeline.json")
	if e != nil {
		return t, e
	}
	if e = ReadJSON(p, &t); e != nil {
		return t, e
	}
	if e = checkSelection(root, t); e != nil {
		return t, e
	}
	if p := t.Problems(); len(p) > 0 {
		return t, fmt.Errorf("时间轴不合格：%s", strings.Join(p, "；"))
	}
	canonical := t
	canonical.gaps()
	if Digest(canonical.Instrumentals) != Digest(t.Instrumentals) {
		return t, fmt.Errorf("无歌词区段与歌词时间轴不一致，请重新 prepare")
	}
	return t, nil
}
func srtTime(s float64) string {
	ms := int(math.Round(s * 1000))
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

// Cue binds local timing to exact global state. Paths are relative to the scene.
type Cue struct {
	FPS      int     `json:"fps"`
	Timeline string  `json:"timeline"`
	SHA      string  `json:"sha256"`
	Start    float64 `json:"start_seconds"`
	Data     string  `json:"data"`
	DataSHA  string  `json:"data_sha256"`
}

func VerifyCue(dir string, c Cue, duration float64) error {
	p := filepath.Clean(filepath.Join(dir, c.Timeline))
	root := filepath.Dir(filepath.Dir(filepath.Dir(p)))
	if filepath.Clean(p) != filepath.Join(root, Store, "timeline.json") {
		return fmt.Errorf("song.timeline 必须引用 production/song/timeline.json")
	}
	t, e := LoadTimeline(root)
	if e != nil {
		return e
	}
	if !t.Reviewed {
		return fmt.Errorf("歌曲时间轴尚未人工复核")
	}
	h, e := HashFile(p)
	if e != nil {
		return e
	}
	if h != c.SHA {
		return fmt.Errorf("歌曲时间轴变化，请重新运行 am song cues")
	}
	if c.FPS <= 0 || !finite(c.Start) || c.Start < 0 || c.Start >= t.Duration || !finite(duration) || duration <= 0 || c.Start+duration > float64(t.Frames(c.FPS))/float64(c.FPS)+1e-6 || math.Abs(c.Start*float64(c.FPS)-math.Round(c.Start*float64(c.FPS))) > 1e-5 || math.Abs(duration*float64(c.FPS)-math.Round(duration*float64(c.FPS))) > 1e-5 {
		return fmt.Errorf("歌曲镜头时间越界")
	}
	dp, e := LocalPath(dir, c.Data)
	if e != nil {
		return e
	}
	dh, e := HashFile(dp)
	if e != nil {
		return e
	}
	if dh != c.DataSHA {
		return fmt.Errorf("歌曲局部时间轴摘要变化")
	}
	var actual Timeline
	if e = ReadJSON(dp, &actual); e != nil {
		return e
	}
	expected := t.Slice(c.Start, duration)
	if Digest(expected) != Digest(actual) {
		return fmt.Errorf("镜头局部歌词或节拍与全局不一致")
	}
	return nil
}
