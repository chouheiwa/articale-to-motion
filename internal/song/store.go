// Package song owns immutable song candidates and their derived timing contract.
package song

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
	"gopkg.in/yaml.v3"
)

const Store = "production/song"
const DefaultConfig = `schema: 1
provider: minimax
lyrics: lyrics.txt
style: "Mandarin educational solo rap, clear diction"
language: zh
target_seconds: 120
bpm: 100
# Run am song providers for platform access and API key setup.
# Run am song configure --provider bailian|elevenlabs|fal|minimax|acestep|mureka|lyria|manual.
# endpoint: https://your-acestep-server.example
python: python3
`

type Config struct {
	Schema        int     `yaml:"schema" json:"schema"`
	Provider      string  `yaml:"provider" json:"provider"`
	Model         string  `yaml:"model,omitempty" json:"model"`
	Workspace     string  `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	Endpoint      string  `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Lyrics        string  `yaml:"lyrics" json:"lyrics"`
	Style         string  `yaml:"style" json:"style"`
	Language      string  `yaml:"language" json:"language"`
	TargetSeconds float64 `yaml:"target_seconds" json:"target_seconds"`
	BPM           int     `yaml:"bpm" json:"bpm"`
	Python        string  `yaml:"python" json:"-"`
}

func Enabled(root string) bool {
	_, err := os.Stat(filepath.Join(root, "song.yaml"))
	return err == nil
}
func LoadConfig(root string) (Config, error) {
	var c Config
	if _, err := os.Stat(filepath.Join(root, "cast.yaml")); err == nil {
		return c, fmt.Errorf("首版不支持 song + cast")
	}
	b, err := os.ReadFile(filepath.Join(root, "song.yaml"))
	if err != nil {
		return c, err
	}
	return decodeConfig(root, b)
}
func decodeConfig(root string, b []byte) (Config, error) {
	var c Config
	if _, err := os.Stat(filepath.Join(root, "cast.yaml")); err == nil {
		return c, fmt.Errorf("首版不支持 song + cast")
	}
	var err error
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	if err = d.Decode(&c); err != nil {
		return c, fmt.Errorf("song.yaml 字段无效")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return c, fmt.Errorf("song.yaml 只能包含一份配置")
	}
	if c.Schema != 1 || !knownProvider(c.Provider) || strings.TrimSpace(c.Style) == "" || c.Language == "" || !finite(c.TargetSeconds) || c.TargetSeconds < 10 || c.TargetSeconds > 600 || c.BPM < 30 || c.BPM > 300 {
		return c, fmt.Errorf("song.yaml: schema=1，provider=minimax|acestep|bailian|elevenlabs|fal|mureka|lyria|manual，style/language 必填，target_seconds=10..600，bpm=30..300")
	}
	if c.Model == "" {
		c.Model = providerInfo(c.Provider).Model
	}
	if c.Python == "" {
		c.Python = "python3"
	}
	if _, err = LocalPath(root, c.Lyrics); err != nil {
		return c, err
	}
	if c.Provider == "acestep" {
		u, e := url.Parse(c.Endpoint)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("ACE-Step endpoint 必须是无凭据、无查询参数的 HTTP(S) 地址")
		}
	}
	if err = validateHostedConfig(c); err != nil {
		return c, err
	}
	return c, nil
}

// LocalPath rejects traversal and symlinks before reading or writing project state.
func LocalPath(root, rel string) (string, error) {
	if rel == "" || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("项目路径必须是安全相对路径")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	p := root
	for _, part := range strings.Split(filepath.Clean(rel), string(filepath.Separator)) {
		p = filepath.Join(p, part)
		i, e := os.Lstat(p)
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
		if e == nil && i.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("歌曲路径不得包含符号链接：%s", rel)
		}
	}
	return p, nil
}
func HashFile(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func Digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func WriteJSON(p string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if old, e := os.ReadFile(p); e == nil && bytes.Equal(old, b) {
		return nil
	}
	return fsutil.AtomicWrite(p, b, 0644)
}
func ReadJSON(p string, v any) error {
	b, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		return fmt.Errorf("JSON 文件无效：%s", p)
	}
	var x any
	if d.Decode(&x) != io.EOF {
		return fmt.Errorf("JSON 存在多余内容：%s", p)
	}
	return nil
}

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,80}$`)

type Candidate struct {
	ID               string  `json:"id"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	TaskID           string  `json:"task_id,omitempty"`
	RequestSHA       string  `json:"request_sha256,omitempty"`
	Audio            string  `json:"audio"`
	Lyrics           string  `json:"lyrics"`
	AudioSHA         string  `json:"audio_sha256"`
	LyricsSHA        string  `json:"lyrics_sha256"`
	Duration         float64 `json:"duration_seconds"`
	State            string  `json:"state"`
	StartedAt        string  `json:"started_at,omitempty"`
	ElapsedSeconds   float64 `json:"elapsed_seconds,omitempty"`
	ProviderExitCode int     `json:"provider_exit_code,omitempty"`
}

func CandidateDir(root, id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("候选 ID 无效")
	}
	return LocalPath(root, filepath.Join(Store, "candidates", id))
}
func LoadCandidate(root, id string) (Candidate, error) {
	var c Candidate
	dir, e := CandidateDir(root, id)
	if e != nil {
		return c, e
	}
	record, e := LocalPath(dir, "candidate.json")
	if e != nil {
		return c, e
	}
	if e = ReadJSON(record, &c); e != nil {
		return c, e
	}
	if c.ID != id || c.State != "ready" || !finite(c.Duration) || c.Duration <= 0 {
		return c, fmt.Errorf("歌曲候选尚未就绪")
	}
	for _, f := range []struct{ p, h string }{{c.Audio, c.AudioSHA}, {c.Lyrics, c.LyricsSHA}} {
		p, e := LocalPath(dir, f.p)
		if e != nil {
			return c, e
		}
		h, e := HashFile(p)
		if e != nil {
			return c, e
		}
		if h != f.h {
			return c, fmt.Errorf("候选文件摘要变化，请重新导入或生成")
		}
	}
	return c, nil
}
func Select(root, id string) error {
	return withLock(root, func() error { return selectCandidate(root, id) })
}
func selectCandidate(root, id string) error {
	if _, e := LoadConfig(root); e != nil {
		return e
	}
	c, e := LoadCandidate(root, id)
	if e != nil {
		return e
	}
	p, e := LocalPath(root, Store+"/selected.json")
	if e != nil {
		return e
	}
	return WriteJSON(p, c)
}
func Selected(root string) (Candidate, error) {
	var c Candidate
	p, e := LocalPath(root, Store+"/selected.json")
	if e != nil {
		return c, e
	}
	if e = ReadJSON(p, &c); e != nil {
		return c, fmt.Errorf("尚未选定歌曲，请试听后运行 am song select <id>：%w", e)
	}
	actual, e := LoadCandidate(root, c.ID)
	if e != nil {
		return c, e
	}
	if Digest(actual) != Digest(c) {
		return c, fmt.Errorf("选定记录与候选不一致，请重新选择")
	}
	return c, nil
}
func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func BytesHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
