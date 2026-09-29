package song

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chouheiwa/articale-to-motion/internal/envutil"
	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
)

const AlignerVersion = "0.6.1"
const LibrosaVersion = "0.11.0"
const versionScript = `import importlib.metadata as m
assert m.version('xingyu-lyrics-aligner') == '0.6.1', 'requires xingyu-lyrics-aligner==0.6.1'
`
const beatScript = `import importlib.metadata as m, json, sys
assert m.version('librosa') == '0.11.0'
import librosa
y, sr = librosa.load(sys.argv[1], sr=22050, mono=True)
_, frames = librosa.beat.beat_track(y=y, sr=sr)
print(json.dumps(librosa.frames_to_time(frames, sr=sr).tolist()))
`

func python(ctx context.Context, c Config, env map[string]string, args ...string) ([]byte, error) {
	binary, e := envutil.LookPath(c.Python, env["PATH"])
	if e != nil {
		return nil, e
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = envutil.EnvList(env)
	b, e := cmd.Output()
	if e != nil {
		return nil, fmt.Errorf("本地 Python 工具失败；请检查独立环境与固定版本依赖")
	}
	return b, nil
}
func ConvertAlignment(raw []byte, c Candidate, lyrics []string) (Timeline, []string, error) {
	t := Timeline{Schema: 1, CandidateID: c.ID, AudioSHA: c.AudioSHA, LyricsSHA: c.LyricsSHA, Duration: c.Duration, Source: "xingyu-lyrics-aligner/" + AlignerVersion}
	var doc struct {
		Lines []struct {
			Index    int      `json:"index"`
			Text     string   `json:"text"`
			Start    *float64 `json:"start"`
			End      *float64 `json:"end"`
			Status   string   `json:"status"`
			Warnings []string `json:"warnings"`
			Tokens   []struct {
				Text      string   `json:"text"`
				Start     *float64 `json:"start"`
				End       *float64 `json:"end"`
				Estimated bool     `json:"estimated"`
			} `json:"tokens"`
		} `json:"lines"`
		Warnings []string `json:"warnings"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return t, nil, fmt.Errorf("对齐工具 Xingyu alignment.json 格式无效")
	}
	issues := []string{}
	t.Warnings = append(t.Warnings, doc.Warnings...)
	if len(doc.Lines) != len(lyrics) {
		issues = append(issues, "对齐歌词行数不匹配")
	}
	for i, l := range doc.Lines {
		line := Line{ID: fmt.Sprintf("line-%04d", i+1), SourceLine: i + 1, Text: l.Text, AlignmentText: l.Text}
		if i < len(lyrics) {
			line.Text = lyrics[i]
			if strings.TrimSpace(l.Text) != strings.TrimSpace(lyrics[i]) {
				issues = append(issues, fmt.Sprintf("第 %d 行对齐文本不匹配", i+1))
			}
		}
		if l.Start == nil || l.End == nil {
			issues = append(issues, line.ID+" 缺失时间戳")
		} else {
			line.Start = *l.Start
			line.End = *l.End
		}
		if l.Status != "aligned" {
			issues = append(issues, line.ID+" 对齐需复核："+l.Status)
		}
		t.Warnings = append(t.Warnings, l.Warnings...)
		for _, w := range l.Tokens {
			if w.Start == nil || w.End == nil || w.Estimated {
				t.Warnings = append(t.Warnings, line.ID+" 字词缺失或为估算，已移除逐字同步依据："+w.Text)
				continue
			}
			line.Words = append(line.Words, Word{Text: w.Text, Start: *w.Start, End: *w.End})
		}
		t.Lines = append(t.Lines, line)
	}
	issues = append(issues, t.Problems()...)
	return t, issues, nil
}
func Prepare(ctx context.Context, root, manual string, reviewed bool, env map[string]string, out io.Writer) error {
	c, e := LoadConfig(root)
	if e != nil {
		return e
	}
	selected, e := Selected(root)
	if e != nil {
		return e
	}
	return withLock(root, func() error {
		if manual != "" {
			var t Timeline
			if e := ReadJSON(manual, &t); e != nil {
				return e
			}
			t.Reviewed = reviewed
			t.Source = "manual"
			return SaveTimeline(root, t)
		}
		if reviewed {
			return fmt.Errorf("--reviewed 仅用于导入已人工核对的 --timeline")
		}
		if _, e = python(ctx, c, env, "-c", versionScript); e != nil {
			return fmt.Errorf("需要 xingyu-lyrics-aligner==%s：%w", AlignerVersion, e)
		}
		dir, _ := CandidateDir(root, selected.ID)
		audio := filepath.Join(dir, selected.Audio)
		work, e := LocalPath(root, fmt.Sprintf("%s/alignment/%s-%d", Store, selected.ID, time.Now().UnixNano()))
		if e != nil {
			return e
		}
		if e = os.MkdirAll(work, 0755); e != nil {
			return e
		}
		b, e := os.ReadFile(filepath.Join(dir, selected.Lyrics))
		if e != nil {
			return e
		}
		lines := LyricLines(string(b))
		lyricsPath := filepath.Join(work, "alignment-lyrics.txt")
		if e = fsutil.AtomicWrite(lyricsPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); e != nil {
			return e
		}
		mapping := LyricMapping(string(b))
		if e = WriteJSON(filepath.Join(work, "source-lines.json"), mapping); e != nil {
			return e
		}
		media, e := mediaprobe.New(env)
		if e != nil {
			return e
		}
		wav := filepath.Join(work, "analysis.wav")
		if e = media.PrepareAlignment(ctx, audio, wav); e != nil {
			return e
		}
		fmt.Fprintln(out, "正在运行 Xingyu 0.6.1 对齐；原始报告保存在", work)
		if _, e = python(ctx, c, env, "-m", "xingyu_lyrics_aligner.cli", "align", "--audio", wav, "--lyrics", lyricsPath, "--output-dir", work, "--language", c.Language, "--device", "cpu", "--json-result"); e != nil {
			return e
		}
		raw, e := os.ReadFile(filepath.Join(work, "alignment.json"))
		if e != nil {
			return e
		}
		t, issues, e := ConvertAlignment(raw, selected, lines)
		if e != nil {
			return e
		}
		for i := range t.Lines {
			if i < len(mapping) {
				t.Lines[i].SourceLine = mapping[i].SourceLine
			}
		}
		beats, e := python(ctx, c, env, "-c", beatScript, wav)
		if e != nil || json.Unmarshal(beats, &t.Beats) != nil {
			t.Beats = nil
			t.Warnings = append(t.Warnings, "节拍分析失败，已降级为歌词驱动动画")
		}
		issues = append(issues, t.Problems()...)
		if e = WriteJSON(filepath.Join(work, "issues.json"), map[string]any{"blocking": issues, "warnings": t.Warnings}); e != nil {
			return e
		}
		if e = WriteJSON(filepath.Join(work, "timeline-draft.json"), t); e != nil {
			return e
		}
		if len(issues) > 0 {
			return fmt.Errorf("对齐需人工修正：%s；修正 timeline-draft.json 后用 prepare --timeline FILE --reviewed 导入", filepath.Join(work, "issues.json"))
		}
		if e = SaveTimeline(root, t); e != nil {
			return e
		}
		for _, w := range t.Warnings {
			fmt.Fprintln(out, "提示：", w)
		}
		fmt.Fprintln(out, "时间轴已生成。请核对术语与句首时间，再用 prepare --timeline production/song/timeline.json --reviewed 确认。")
		return nil
	})
}
func Doctor(ctx context.Context, root string, env map[string]string, out io.Writer) error {
	c, e := LoadConfig(root)
	if e != nil {
		return e
	}
	var problems []string
	check := func(name string, e error) {
		if e != nil {
			problems = append(problems, name+": "+e.Error())
			fmt.Fprintln(out, "FAIL", name)
		} else {
			fmt.Fprintln(out, "OK", name)
		}
	}
	for _, name := range []string{"ffprobe", "ffmpeg"} {
		_, e := envutil.LookPath(name, env["PATH"])
		check(name, e)
	}
	_, e = python(ctx, c, env, "-c", versionScript)
	check("xingyu-lyrics-aligner=="+AlignerVersion, e)
	_, e = python(ctx, c, env, "-c", "import importlib.metadata as m; assert m.version('librosa') == '"+LibrosaVersion+"'")
	if e != nil {
		fmt.Fprintln(out, "WARN librosa=="+LibrosaVersion+" 不可用；将降级为歌词驱动")
	}
	if c.Provider == "minimax" {
		binary, e := envutil.LookPath("mmx", env["PATH"])
		if e == nil {
			cmd := exec.CommandContext(ctx, binary, "music", "generate", "--help")
			cmd.Env = envutil.EnvList(env)
			b, err := cmd.Output()
			e = err
			if e == nil && !strings.Contains(string(b), "--lyrics-file") {
				e = fmt.Errorf("mmx 不支持 --lyrics-file")
			}
		}
		check("MiniMax mmx（登录态以实际生成为准）", e)
	} else if c.Provider == "acestep" {
		check("ACE-Step health", (ACEClient{Endpoint: c.Endpoint, Key: env["ACESTEP_API_KEY"]}).Health(ctx))
	}
	if c.Provider == "manual" {
		fmt.Fprintln(out, "OK 网页手动生成无需 API Key；handoff 不需要媒体/对齐环境，receive 和 prepare 才需要相应工具")
	}
	if c.Provider != "minimax" && c.Provider != "acestep" && c.Provider != "manual" {
		check(providerInfo(c.Provider).Key+"（仅本地存在性，不验证权限/额度）", CheckCredential(c, env))
	}
	fmt.Fprintf(out, "平台接入说明：am song providers %s\n", c.Provider)
	if len(problems) > 0 {
		return fmt.Errorf("歌曲环境检查未通过：%s", strings.Join(problems, "；"))
	}
	return nil
}

type SourceLine struct {
	SourceLine int    `json:"source_line"`
	Original   string `json:"original"`
	Display    string `json:"display"`
	Alignment  string `json:"alignment_input"`
}

func LyricMapping(text string) []SourceLine {
	var out []SourceLine
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || (strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
			continue
		}
		out = append(out, SourceLine{i + 1, raw, line, line})
	}
	return out
}
