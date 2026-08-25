package castbeats

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/chouheiwa/articale-to-motion/internal/schedule"
)

// sceneHead 是本命令需要知道的全部镜头信息：它在哪、叫什么、多长、有没有
// cast 块。既有的 cast.beats 不在其中，这是刻意的。
type sceneHead struct {
	dir      string
	id       string
	duration float64
	hasCast  bool
}

// readSceneHeads 按 schedule.SceneDirs 给出的顺序读出各镜头，只解析上面
// 那几个字段。
//
// 刻意不走 schedule.Plan（也就是不走 scene.Load）：Plan 会连带校验既有的
// cast.beats，而 beats 是本命令的输出、不是它的输入。把 duration_seconds
// 改小、旧节拍还越界时，scene.Load 会拒绝这份 scene.json——而本命令正是
// 修它的工具，跟着 Plan 走就是把工具自己锁在门外，用户只剩手改 JSON 一条
// 路。忽略既有节拍之后，重跑天然幂等，越界与否与本命令无关。
//
// 顺序仍然来自 schedule.SceneDirs，与 Plan 同源：这里放弃的只是"读哪些
// 字段"，不是"镜头谁先谁后"——后者有第二条路径才是真问题。
//
// 代价：本命令不再对损坏的 scene.json 做完整校验。这不留缺口——渲染前的
// scene.Load 与 am validate cast 仍然会拦下越界节拍与其它非法字段，本命令
// 只是不在这一步重复拦一次。
func readSceneHeads(scenesDir string) ([]sceneHead, error) {
	dirs, err := schedule.SceneDirs(scenesDir)
	if err != nil {
		return nil, fmt.Errorf("读取镜头目录失败：%w", err)
	}
	heads := make([]sceneHead, 0, len(dirs))
	for _, dir := range dirs {
		path := filepath.Join(dir, "scene.json")
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取 %s 失败：%w", path, err)
		}
		var parsed struct {
			ID              string          `json:"id"`
			DurationSeconds *float64        `json:"duration_seconds"`
			Cast            json.RawMessage `json:"cast"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("%s 不是合法 JSON：%w", path, err)
		}
		if parsed.ID == "" {
			return nil, fmt.Errorf("%s 缺少 id", path)
		}
		if parsed.DurationSeconds == nil {
			return nil, fmt.Errorf("%s 缺少 duration_seconds", path)
		}
		duration := *parsed.DurationSeconds
		if math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 {
			return nil, fmt.Errorf("%s 的 duration_seconds 必须是有限正数，收到 %v", path, duration)
		}
		heads = append(heads, sceneHead{
			dir:      dir,
			id:       parsed.ID,
			duration: duration,
			hasCast:  len(parsed.Cast) > 0 && string(parsed.Cast) != "null",
		})
	}
	return heads, nil
}
