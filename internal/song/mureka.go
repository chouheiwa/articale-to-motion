package song

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"
)

func murekaPrompt(c Config) string {
	return fmt.Sprintf("%s; language %s; target duration %.0f seconds; %d BPM; clear solo vocals", c.Style, c.Language, c.TargetSeconds, c.BPM)
}
func (h hostedClient) submitMureka(ctx context.Context, c Config, lyrics string) (string, error) {
	if utf8.RuneCountInString(lyrics) > 5000 || utf8.RuneCountInString(murekaPrompt(c)) > 1024 {
		return "", fmt.Errorf("歌曲平台 Mureka 歌词最多 5000 字符，曲风及生成要求最多 1024 字符")
	}
	var task struct {
		ID string `json:"id"`
	}
	if e := h.json(ctx, "POST", "/v1/song/generate", map[string]any{"model": c.Model, "lyrics": lyrics, "prompt": murekaPrompt(c), "n": 1, "stream": false}, &task); e != nil {
		return "", e
	}
	if !idPattern.MatchString(task.ID) {
		return "", fmt.Errorf("歌曲平台 Mureka 未返回有效任务 ID；勿自动重发")
	}
	return task.ID, nil
}
func (h hostedClient) resumeMureka(ctx context.Context, id, dest string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("歌曲平台 Mureka 任务 ID 无效")
	}
	for {
		var task struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Choices []struct {
				URL string `json:"url"`
			} `json:"choices"`
		}
		if e := h.json(ctx, "GET", "/v1/song/query/"+id, nil, &task); e != nil {
			return e
		}
		if task.ID != id {
			return fmt.Errorf("歌曲平台 Mureka 查询结果与任务不一致")
		}
		switch task.Status {
		case "succeeded":
			if len(task.Choices) != 1 || task.Choices[0].URL == "" {
				return fmt.Errorf("歌曲平台 Mureka 请求一首但结果数量或下载地址异常；未自动挑选")
			}
			return h.download(ctx, task.Choices[0].URL, dest)
		case "failed", "timeouted", "cancelled":
			return fmt.Errorf("歌曲平台 Mureka 任务失败或已终止；未自动重发")
		case "preparing", "queued", "running", "streaming":
		default:
			return fmt.Errorf("歌曲平台 Mureka 未知任务状态")
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
