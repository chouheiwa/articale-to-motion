package song

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ACEClient deliberately does not retry submissions or expose service response
// bodies: a disconnected POST may have succeeded and errors can contain secrets.
type ACEClient struct {
	Endpoint, Key string
	PollInterval  time.Duration
}

func (a ACEClient) client() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("redirect refused") }}
}
func (a ACEClient) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	b, e := json.Marshal(body)
	if e != nil {
		return nil, e
	}
	u := strings.TrimRight(a.Endpoint, "/") + path
	req, e := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(b))
	if e != nil {
		return nil, fmt.Errorf("ACE-Step 请求地址无效")
	}
	req.Header.Set("Content-Type", "application/json")
	if a.Key != "" {
		req.Header.Set("Authorization", "Bearer "+a.Key)
	}
	r, e := a.client().Do(req)
	if e != nil {
		return nil, fmt.Errorf("ACE-Step 请求未完成（状态可能不明）；未自动重发")
	}
	if r.StatusCode != http.StatusOK {
		r.Body.Close()
		return nil, fmt.Errorf("ACE-Step HTTP %d", r.StatusCode)
	}
	return r, nil
}
func (a ACEClient) call(ctx context.Context, path string, body, out any) error {
	r, e := a.request(ctx, "POST", path, body)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if e = json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&env); e != nil {
		return fmt.Errorf("ACE-Step 返回无效 JSON")
	}
	if env.Code != 200 {
		return fmt.Errorf("ACE-Step 服务拒绝请求（code=%d）", env.Code)
	}
	if e = json.Unmarshal(env.Data, out); e != nil {
		return fmt.Errorf("ACE-Step data 格式无效")
	}
	return nil
}
func (a ACEClient) Submit(ctx context.Context, c Config, lyrics string) (string, error) {
	req := map[string]any{"prompt": c.Style, "lyrics": lyrics, "model": c.Model, "vocal_language": c.Language, "audio_duration": c.TargetSeconds, "bpm": c.BPM, "batch_size": 1, "audio_format": "mp3", "task_type": "text2music", "sample_mode": false, "use_format": false, "use_cot_caption": false, "use_cot_language": false, "thinking": false}
	var res struct {
		TaskID string `json:"task_id"`
	}
	if e := a.call(ctx, "/release_task", req, &res); e != nil {
		return "", e
	}
	if res.TaskID == "" {
		return "", fmt.Errorf("ACE-Step 未返回任务 ID；请人工检查服务任务，勿自动重发")
	}
	return res.TaskID, nil
}
func (a ACEClient) Resume(ctx context.Context, id, dest string) error {
	interval := a.PollInterval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	for {
		var rows []struct {
			TaskID string `json:"task_id"`
			Status int    `json:"status"`
			Result string `json:"result"`
		}
		if e := a.call(ctx, "/query_result", map[string]any{"task_id_list": []string{id}}, &rows); e != nil {
			return e
		}
		if len(rows) != 1 || rows[0].TaskID != id {
			return fmt.Errorf("ACE-Step 查询结果与任务不一致")
		}
		switch rows[0].Status {
		case 1:
			var results []struct {
				File string `json:"file"`
			}
			if json.Unmarshal([]byte(rows[0].Result), &results) != nil || len(results) != 1 || results[0].File == "" {
				return fmt.Errorf("ACE-Step 下载结果无效")
			}
			return a.download(ctx, results[0].File, dest)
		case 2:
			return fmt.Errorf("ACE-Step 生成失败；未自动重发")
		case 0:
		default:
			return fmt.Errorf("ACE-Step 未知任务状态")
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (a ACEClient) download(ctx context.Context, ref, dest string) error {
	base, e := url.Parse(a.Endpoint)
	if e != nil {
		return fmt.Errorf("ACE-Step 地址无效")
	}
	u, e := url.Parse(ref)
	if e != nil {
		return fmt.Errorf("ACE-Step 下载地址无效")
	}
	u = base.ResolveReference(u)
	if u.Scheme != base.Scheme || u.Host != base.Host || u.User != nil {
		return fmt.Errorf("ACE-Step 下载必须与服务同源")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return fmt.Errorf("ACE-Step 下载请求无效")
	}
	if a.Key != "" {
		req.Header.Set("Authorization", "Bearer "+a.Key)
	}
	res, e := a.client().Do(req)
	if e != nil {
		return fmt.Errorf("ACE-Step 下载中断；可用原任务 ID 继续")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("ACE-Step 下载 HTTP %d", res.StatusCode)
	}
	if e = os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	const limit = 512 << 20
	n, e := io.Copy(f, io.LimitReader(res.Body, limit+1))
	ce := f.Close()
	if e != nil || ce != nil || n == 0 || n > limit {
		return fmt.Errorf("ACE-Step 下载不完整或超过 512 MiB")
	}
	return os.Rename(f.Name(), dest)
}
func (a ACEClient) Health(ctx context.Context) error {
	r, e := a.request(ctx, "GET", "/health", nil)
	if e != nil {
		return e
	}
	r.Body.Close()
	return nil
}
