package song

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// hostedClient never retries a POST. Only fal/Mureka jobs with persisted IDs are resumable.
// API origins are fixed; credentials are never attached to media downloads.
type hostedClient struct {
	config    Config
	key, base string
	http      *http.Client
	interval  time.Duration
}

func newHosted(c Config, env map[string]string) hostedClient {
	base := "https://api.elevenlabs.io"
	if c.Provider == "mureka" {
		base = "https://api.mureka.ai"
	}
	if c.Provider == "lyria" {
		base = "https://generativelanguage.googleapis.com"
	}
	if c.Provider == "fal" {
		base = "https://queue.fal.run"
	}
	if c.Provider == "bailian" {
		base = "https://" + c.Workspace + ".cn-beijing.maas.aliyuncs.com"
	}
	return hostedClient{config: c, key: env[providerInfo(c.Provider).Key], base: base, http: &http.Client{Timeout: 20 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirect refused") }}, interval: 3 * time.Second}
}
func (h hostedClient) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, h.base+path, reader)
	if e != nil {
		return nil, fmt.Errorf("歌曲 API 地址无效")
	}
	req.Header.Set("Content-Type", "application/json")
	switch h.config.Provider {
	case "lyria":
		req.Header.Set("x-goog-api-key", h.key)
	case "elevenlabs":
		req.Header.Set("xi-api-key", h.key)
	case "fal":
		req.Header.Set("Authorization", "Key "+h.key)
	default:
		req.Header.Set("Authorization", "Bearer "+h.key)
	}
	r, e := h.http.Do(req)
	if e != nil {
		return nil, fmt.Errorf("%s 请求未完成；提交结果可能不明，未自动重发", h.config.Provider)
	}
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		r.Body.Close()
		hint := ""
		switch r.StatusCode {
		case 401:
			hint = "；检查 API Key 与地域"
		case 403:
			hint = "；检查模型权限、邀测及账户额度"
		case 429:
			hint = "；检查额度或限流"
		}
		return nil, fmt.Errorf("%s HTTP %d%s（am song providers %s 查看配置）", h.config.Provider, r.StatusCode, hint, h.config.Provider)
	}
	return r, nil
}
func (h hostedClient) json(ctx context.Context, method, path string, body, out any) error {
	r, e := h.request(ctx, method, path, body)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(out) != nil {
		return fmt.Errorf("%s 返回无效 JSON", h.config.Provider)
	}
	return nil
}
func validateLyrics(c Config, lyrics string) error {
	if c.Provider == "mureka" && (utf8.RuneCountInString(lyrics) > 5000 || utf8.RuneCountInString(murekaPrompt(c)) > 1024) {
		return fmt.Errorf("Mureka 歌词最多 5000 字符，曲风及生成要求最多 1024 字符")
	}
	if c.Provider == "bailian" {
		max := 2000
		for _, r := range lyrics {
			if unicode.Is(unicode.Han, r) {
				max = 350
				break
			}
		}
		n := utf8.RuneCountInString(lyrics)
		if n < 5 || n > max {
			return fmt.Errorf("百炼非流式歌词需 5..%d 字符（含结构标签），当前 %d", max, n)
		}
	}
	if c.Provider == "elevenlabs" {
		_, e := elevenPlan(c, lyrics)
		return e
	}
	return nil
}
func elevenPlan(c Config, lyrics string) (map[string]any, error) {
	lines := strings.Split(strings.TrimSpace(lyrics), "\n")
	for _, line := range lines {
		if utf8.RuneCountInString(line) > 200 {
			return nil, fmt.Errorf("ElevenLabs 每行歌词最多 200 字符")
		}
	}
	n := int(math.Ceil(c.TargetSeconds / 120))
	if m := (len(lines) + 29) / 30; m > n {
		n = m
	}
	total := int(math.Round(c.TargetSeconds * 1000))
	if total/n < 3000 {
		return nil, fmt.Errorf("歌词过多，请增加目标时长或缩短歌词")
	}
	chunks := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		start, end := i*len(lines)/n, (i+1)*len(lines)/n
		duration := total / n
		if i == n-1 {
			duration += total % n
		}
		chunks = append(chunks, map[string]any{"text": strings.Join(lines[start:end], "\n"), "duration_ms": duration, "positive_styles": []string{c.Style, fmt.Sprintf("%d BPM", c.BPM), "clear solo vocals", "precise diction", "educational song", "balanced accompaniment"}, "negative_styles": []string{}})
	}
	return map[string]any{"model_id": c.Model, "composition_plan": map[string]any{"chunks": chunks}}, nil
}
func (h hostedClient) Generate(ctx context.Context, lyrics, dest string) (string, error) {
	if e := validateLyrics(h.config, lyrics); e != nil {
		return "", e
	}
	if h.config.Provider == "lyria" {
		return h.generateLyria(ctx, lyrics, dest)
	}
	if h.config.Provider == "elevenlabs" {
		body, e := elevenPlan(h.config, lyrics)
		if e != nil {
			return "", e
		}
		r, e := h.request(ctx, "POST", "/v1/music?output_format=mp3_44100_128", body)
		if e != nil {
			return "", e
		}
		defer r.Body.Close()
		id := r.Header.Get("song-id")
		if !idPattern.MatchString(id) {
			id = ""
		}
		return id, saveAudio(r.Body, dest)
	}
	var result struct {
		RequestID string `json:"request_id"`
		Output    struct {
			Audio struct {
				URL string `json:"url"`
			} `json:"audio"`
		} `json:"output"`
	}
	input := map[string]any{"lyrics": lyrics, "is_instrumental": false, "format": "mp3"}
	if h.config.Model == "fun-music-preview" {
		input["prompt"] = h.config.Style
	} // required by preview; ignored with lyrics
	e := h.json(ctx, "POST", "/api/v1/services/audio/music/generation", map[string]any{"model": h.config.Model, "input": input}, &result)
	if e != nil {
		return "", e
	}
	id := result.RequestID
	if !idPattern.MatchString(id) {
		id = ""
	}
	return id, h.download(ctx, result.Output.Audio.URL, dest)
}
func (h hostedClient) Submit(ctx context.Context, c Config, lyrics string) (string, error) {
	if c.Provider == "mureka" {
		return h.submitMureka(ctx, c, lyrics)
	}
	var result struct {
		ID string `json:"request_id"`
	}
	e := h.json(ctx, "POST", "/"+c.Model, map[string]any{"tags": c.Style, "lyrics": lyrics, "duration": c.TargetSeconds}, &result)
	if e != nil {
		return "", e
	}
	if !idPattern.MatchString(result.ID) {
		return "", fmt.Errorf("fal 未返回有效任务 ID；勿自动重发")
	}
	return result.ID, nil
}
func (h hostedClient) Resume(ctx context.Context, id, dest string) error {
	if h.config.Provider == "mureka" {
		return h.resumeMureka(ctx, id, dest)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("fal 任务 ID 无效")
	}
	path := "/" + h.config.Model + "/requests/" + id
	for {
		var status struct {
			Status string          `json:"status"`
			Error  json.RawMessage `json:"error"`
		}
		if e := h.json(ctx, "GET", path+"/status", nil, &status); e != nil {
			return e
		}
		switch status.Status {
		case "COMPLETED":
			if len(status.Error) > 0 && string(status.Error) != "null" {
				return fmt.Errorf("fal 任务失败；未重新提交")
			}
			var result struct {
				Audio struct {
					URL string `json:"url"`
				} `json:"audio"`
			}
			if e := h.json(ctx, "GET", path, nil, &result); e != nil {
				return e
			}
			return h.download(ctx, result.Audio.URL, dest)
		case "IN_QUEUE", "IN_PROGRESS":
		default:
			return fmt.Errorf("fal 返回未知或失败任务状态")
		}
		timer := time.NewTimer(h.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (h hostedClient) download(ctx context.Context, ref, dest string) error {
	u, e := url.Parse(ref)
	if e != nil || u.User != nil || u.Fragment != "" || u.Port() != "" {
		return fmt.Errorf("歌曲下载地址无效")
	}
	host := strings.ToLower(u.Hostname())
	allowed := false
	if h.config.Provider == "bailian" {
		allowed = strings.HasSuffix(host, ".oss-cn-beijing.aliyuncs.com")
	} else if h.config.Provider == "mureka" {
		allowed = host == "cdn.mureka.ai" || strings.HasSuffix(host, ".mureka.ai")
	} else {
		allowed = host == "fal.media" || strings.HasSuffix(host, ".fal.media") || host == "storage.googleapis.com"
	}
	if !allowed || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("歌曲下载地址不在平台存储域名清单内")
	}
	// Official Bailian examples return HTTP OSS links; use TLS for the same signed resource.
	u.Scheme = "https"
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return fmt.Errorf("歌曲下载请求无效")
	}
	r, e := h.http.Do(req)
	if e != nil {
		return fmt.Errorf("歌曲下载中断；未重新生成")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("歌曲下载 HTTP %d；未重新生成", r.StatusCode)
	}
	return saveAudio(r.Body, dest)
}
func saveAudio(r io.Reader, dest string) error {
	f, e := os.CreateTemp(filepath.Dir(dest), ".audio-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	const limit = 512 << 20
	n, e := io.Copy(f, io.LimitReader(r, limit+1))
	ce := f.Close()
	if e != nil || ce != nil || n == 0 || n > limit {
		return fmt.Errorf("歌曲音频下载不完整或超过 512 MiB")
	}
	return os.Rename(f.Name(), dest)
}
