package song

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
)

const handoffState = Store + "/web-handoff.json"

type WebHandoff struct {
	Schema      int    `json:"schema"`
	ID          string `json:"id"`
	State       string `json:"state"`
	LyricsSHA   string `json:"lyrics_sha256"`
	StyleSHA    string `json:"style_sha256"`
	CandidateID string `json:"candidate_id,omitempty"`
}

func handoffDir(root, id string) (string, error) {
	if len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		return "", fmt.Errorf("网页交接 ID 无效")
	}
	return LocalPath(root, filepath.Join(Store, "handoffs", id))
}
func handoffStyle(c Config) string {
	return fmt.Sprintf("%s\n演唱语言：%s\n目标时长：%.0f 秒（以实际生成音频为准）\n目标 BPM：%d\n单人演唱，咬字清楚，伴奏不要盖过人声。按提供的歌词及顺序演唱，保留重复副歌，不自动改词或新增知识内容。\n", c.Style, c.Language, c.TargetSeconds, c.BPM)
}
func LoadHandoff(root string) (WebHandoff, string, error) {
	var h WebHandoff
	p, e := LocalPath(root, handoffState)
	if e != nil {
		return h, "", e
	}
	if e = ReadJSON(p, &h); e != nil {
		return h, "", fmt.Errorf("尚无网页交接，请先运行 am song handoff：%w", e)
	}
	if h.Schema != 1 || (h.State != "waiting_for_audio" && h.State != "imported") {
		return h, "", fmt.Errorf("网页交接状态无效")
	}
	if (h.State == "imported" && !idPattern.MatchString(h.CandidateID)) || (h.State == "waiting_for_audio" && h.CandidateID != "") {
		return h, "", fmt.Errorf("网页交接候选记录无效")
	}
	dir, e := handoffDir(root, h.ID)
	if e != nil {
		return h, "", e
	}
	for _, entry := range []struct{ name, sha string }{{"lyrics.txt", h.LyricsSHA}, {"style.txt", h.StyleSHA}} {
		path, e := LocalPath(dir, entry.name)
		if e != nil {
			return h, "", e
		}
		sha, e := HashFile(path)
		if e != nil {
			return h, "", e
		}
		if sha != entry.sha {
			return h, "", fmt.Errorf("网页交接内容已变化，请重新核对歌词和曲风")
		}
	}
	if h.ID != Digest([]string{h.LyricsSHA, h.StyleSHA}) {
		return h, "", fmt.Errorf("网页交接摘要不一致")
	}
	return h, dir, nil
}

// Handoff writes reproducible snapshots and a durable wait marker. It performs
// no network, model or media calls and never holds a process open awaiting a user.
func Handoff(root string, out io.Writer) error { return HandoffWithRefresh(root, false, out) }
func HandoffWithRefresh(root string, refresh bool, out io.Writer) error {
	c, e := LoadConfig(root)
	if e != nil {
		return e
	}
	return withLock(root, func() error {
		p, e := LocalPath(root, c.Lyrics)
		if e != nil {
			return e
		}
		lyrics, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		if len(LyricLines(string(lyrics))) == 0 {
			return fmt.Errorf("请先根据文章写好歌词文件 %s，再运行 am song handoff", c.Lyrics)
		}
		style := []byte(handoffStyle(c))
		h := WebHandoff{Schema: 1, State: "waiting_for_audio", LyricsSHA: BytesHash(lyrics), StyleSHA: BytesHash(style)}
		h.ID = Digest([]string{h.LyricsSHA, h.StyleSHA})
		dir, e := handoffDir(root, h.ID)
		if e != nil {
			return e
		}
		state, e := LocalPath(root, handoffState)
		if e != nil {
			return e
		}
		if _, err := os.Stat(state); err == nil {
			old, _, err := LoadHandoff(root)
			if err != nil {
				return err
			}
			if old.ID == h.ID {
				h = old
			} else if old.State == "waiting_for_audio" && !refresh {
				return fmt.Errorf("仍在等待上一版歌词的音频；不能替换已交给用户的材料。确需重做时运行 am song handoff --refresh，并告知用户使用新版歌词重新生成")
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		for _, entry := range []struct {
			name string
			data []byte
		}{{"lyrics.txt", lyrics}, {"style.txt", style}} {
			dest, e := LocalPath(dir, entry.name)
			if e != nil {
				return e
			}
			b, e := os.ReadFile(dest)
			if e == nil {
				if !bytes.Equal(b, entry.data) {
					return fmt.Errorf("网页交接快照已被修改，拒绝覆盖")
				}
				continue
			}
			if !os.IsNotExist(e) {
				return e
			}
			if e = fsutil.AtomicWrite(dest, entry.data, 0644); e != nil {
				return e
			}
		}
		if e = WriteJSON(state, h); e != nil {
			return e
		}
		fmt.Fprintf(out, "网页生成材料：%s\n\n【复制到歌词栏】\n%s\n\n【复制到曲风/风格栏】\n%s\n", dir, lyrics, style)
		if h.State == "imported" {
			candidate, e := LoadCandidate(root, h.CandidateID)
			if e != nil {
				return e
			}
			fmt.Fprintf(out, "已收到候选 %s；请先核对用户试听选择，不重复索要或生成音频。\n", candidate.ID)
		} else {
			fmt.Fprintln(out, "等待用户提供音频：请在所选网页的自定义歌词模式粘贴以上内容，生成并试听后下载 MP3/WAV。将文件上传到当前会话，或提供本机文件路径。若网页改了歌词，请同时提供最终歌词。\n此处暂停，不生成时间轴、不分镜、不渲染；收到文件后运行 am song receive AUDIO。CLI 已正常退出，不代表歌曲已生成。")
		}
		return nil
	})
}
func Receive(root, audio string, media mediaprobe.Toolchain) (Candidate, error) {
	var candidate Candidate
	if _, e := LoadConfig(root); e != nil {
		return candidate, e
	}
	err := withLock(root, func() error {
		h, dir, e := LoadHandoff(root)
		if e != nil {
			return e
		}
		if h.CandidateID != "" {
			old, e := LoadCandidate(root, h.CandidateID)
			if e != nil {
				return e
			}
			if old.LyricsSHA != h.LyricsSHA {
				return fmt.Errorf("接收记录与歌词快照不一致")
			}
			sha, e := HashFile(audio)
			if e != nil {
				return e
			}
			if sha == old.AudioSHA {
				candidate = old
				return nil
			}
		}
		// Always import the exact lyric snapshot handed to the user, even if the
		// editable project lyrics have since changed. Corrections use explicit import.
		candidate, e = importCandidate(root, audio, filepath.Join(dir, "lyrics.txt"), media)
		if e != nil {
			return e
		}
		h.State = "imported"
		h.CandidateID = candidate.ID
		p, e := LocalPath(root, handoffState)
		if e != nil {
			return e
		}
		return WriteJSON(p, h)
	})
	return candidate, err
}
