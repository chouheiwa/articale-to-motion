package song

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Lyria returns inline MP3 in model_output steps. No remote URI is fetched and
// no model-supplied lyric/timing text overwrites the project's verified lyrics.
func (h hostedClient) generateLyria(ctx context.Context, lyrics, dest string) (string, error) {
	prompt := fmt.Sprintf("Create a complete solo vocal song. Style: %s. Language: %s. Target duration: %.0f seconds. Tempo: %d BPM. Sing the provided lyrics verbatim in order, including every repeated section. Do not rewrite, translate, add or omit lyrics. Return MP3 audio.\nLyrics:\n%s", h.config.Style, h.config.Language, h.config.TargetSeconds, h.config.BPM, lyrics)
	r, e := h.request(ctx, "POST", "/v1beta/interactions", map[string]any{"model": h.config.Model, "input": prompt, "store": false, "stream": false})
	if e != nil {
		return "", e
	}
	defer r.Body.Close()
	return decodeLyria(r.Body, dest)
}
func decodeLyria(reader io.Reader, dest string) (string, error) {
	// Unlike metadata-only responses this JSON contains the base64 audio itself.
	const maxJSON = 96 << 20
	b, e := io.ReadAll(io.LimitReader(reader, maxJSON+1))
	if e != nil || len(b) > maxJSON {
		return "", fmt.Errorf("Lyria 音频响应中断或超过 96 MiB")
	}
	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Steps  []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				MIME string `json:"mime_type"`
				Data string `json:"data"`
			} `json:"content"`
		} `json:"steps"`
	}
	if json.Unmarshal(b, &result) != nil {
		return "", fmt.Errorf("Lyria 返回无效 JSON")
	}
	id := result.ID
	if !idPattern.MatchString(id) {
		id = ""
	}
	if result.Status != "" && result.Status != "completed" {
		return id, fmt.Errorf("Lyria 未返回完成状态；未重新生成")
	}
	var encoded string
	count := 0
	for _, step := range result.Steps {
		if step.Type != "model_output" {
			continue
		}
		for _, block := range step.Content {
			if block.Type != "audio" {
				continue
			}
			count++
			if block.MIME != "audio/mpeg" && block.MIME != "audio/mp3" {
				return id, fmt.Errorf("Lyria 返回非 MP3 音频，拒绝保存为 MP3")
			}
			encoded = block.Data
		}
	}
	if count != 1 || encoded == "" {
		return id, fmt.Errorf("Lyria 应返回一个完整内联音轨，实际音频块缺失或有多个；未自动选择或拼接")
	}
	return id, saveAudio(base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded)), dest)
}
