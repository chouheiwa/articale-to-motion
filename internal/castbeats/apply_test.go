package castbeats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const packYAML = `schema: cast/v1
id: %s
name: %s
summary: 测试角色
voice:
  minimax: { voiceId: "v-1", speed: 0.98 }
rig:
  file: rig.svg
  viewBox: [0, 0, 400, 520]
  baselineY: 512
  joints:
    head:     { pivot: [200, 168], rotate: [-18, 18] }
scale:
  heightRatio: [0.22, 0.32]
poses:
  idle: {}
`

const packSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 520">
  <g id="j-head"><circle cx="200" cy="168" r="60" fill="#fff" stroke="#000" stroke-width="6"/></g>
</svg>
`

func writePack(t *testing.T, dir, id, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "character.yaml"), fmt.Sprintf(packYAML, id, name))
	write(t, filepath.Join(dir, "rig.svg"), packSVG)
	write(t, filepath.Join(dir, "dna.md"), "# "+name+"\n")
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeDialogue 写出一份最小 dialogue.json，lines 依次是 speaker/start/end 三元组。
func writeDialogue(t *testing.T, root string, lines ...any) {
	t.Helper()
	var rows []map[string]any
	for i := 0; i < len(lines); i += 3 {
		rows = append(rows, map[string]any{
			"srtIndex":     i/3 + 1,
			"speaker":      lines[i],
			"startSeconds": lines[i+1],
			"endSeconds":   lines[i+2],
		})
	}
	body, err := json.MarshalIndent(map[string]any{
		"schema": "cast-dialogue/v1",
		"lines":  rows,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "production", "dialogue.json"), string(body))
}

// writeScene 写出一个镜头目录：transcript 占位文件、一份角色包拷贝、scene.json。
// castBlock 传空字符串表示这个镜头不带 cast 块（老镜头形态）。
func writeScene(t *testing.T, root, id string, duration float64, castBlock string) string {
	t.Helper()
	dir := filepath.Join(root, "scenes", id)
	write(t, filepath.Join(dir, "transcript.txt"), "占位")
	writePack(t, filepath.Join(dir, "cast-pack", "heiwa"), "heiwa", "黑娃")
	writePack(t, filepath.Join(dir, "cast-pack", "zhaocai"), "zhaocai", "招财")
	body := fmt.Sprintf(`{
  "id": %q,
  "duration_seconds": %v,
  "output": "out.mp4",
  "transcript": "transcript.txt",
  "text": "占位文案 引号\" 与反斜杠\\ 都要原样留着"%s
}
`, id, duration, castBlock)
	write(t, filepath.Join(dir, "scene.json"), body)
	return filepath.Join(dir, "scene.json")
}

const castBlockNoBeats = `,
  "cast": {
    "pack_dir": "cast-pack",
    "ground_y": 0.78,
    "on_stage": [
      {"id": "heiwa", "x": 0.3, "pose": "idle", "facing": "right"}
    ]
  }`

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// TestApplyWritesBeatsAndKeepsOtherFields 是本命令的主路径：cast 块里原本
// 没有 beats，跑完之后 beats 出现且是镜头本地时间，而 pack_dir / ground_y /
// on_stage / text 一个字符都不许变。
func TestApplyWritesBeatsAndKeepsOtherFields(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "heiwa", 0.0, 1.0, "zhaocai", 1.0, 2.5)
	path := writeScene(t, root, "scene-001", 1.0, castBlockNoBeats)
	writeScene(t, root, "scene-002", 1.5, castBlockNoBeats)

	report, err := Apply(root)
	if err != nil {
		t.Fatalf("Apply 应当成功：%v", err)
	}
	if report.Lines != 2 {
		t.Errorf("应报出读到 2 行台词，得到 %d", report.Lines)
	}

	got := read(t, path)
	if !strings.Contains(got, `"beats": [`) {
		t.Fatalf("scene-001 应写出 beats：%s", got)
	}
	if !strings.Contains(got, `"speaker": "heiwa"`) {
		t.Errorf("scene-001 的节拍说话人应是 heiwa：%s", got)
	}
	if !strings.Contains(got, `"pack_dir": "cast-pack"`) || !strings.Contains(got, `"ground_y": 0.78`) {
		t.Errorf("pack_dir / ground_y 必须原样保留：%s", got)
	}
	if !strings.Contains(got, `"pose": "idle"`) || !strings.Contains(got, `"facing": "right"`) {
		t.Errorf("on_stage 必须原样保留：%s", got)
	}
	if !strings.Contains(got, `引号\" 与反斜杠\\`) {
		t.Errorf("text 的转义必须原样保留：%s", got)
	}

	second := read(t, filepath.Join(root, "scenes", "scene-002", "scene.json"))
	if !strings.Contains(second, `"start": 0`) || !strings.Contains(second, `"end": 1.5`) {
		t.Errorf("scene-002 的节拍应减去镜头起点 1.0，成为本地 [0, 1.5]：%s", second)
	}
	if strings.Contains(second, `"start": 1`) && !strings.Contains(second, `"start": 0`) {
		t.Errorf("scene-002 写进了全局时间而不是本地时间：%s", second)
	}
}

// TestApplyOverwritesExistingBeats 幂等的前提：已有 beats（哪怕是错的）要
// 被整体重写，而不是追加或跳过。
func TestApplyOverwritesExistingBeats(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "heiwa", 0.0, 1.0)
	stale := `,
  "cast": {
    "pack_dir": "cast-pack",
    "ground_y": 0.78,
    "on_stage": [],
    "beats": [
      {"speaker": "zhaocai", "start": 0.1, "end": 0.9}
    ]
  }`
	path := writeScene(t, root, "scene-001", 1.0, stale)
	if _, err := Apply(root); err != nil {
		t.Fatalf("Apply 应当成功：%v", err)
	}
	got := read(t, path)
	if strings.Contains(got, "zhaocai") {
		t.Errorf("旧节拍必须被整体重写掉：%s", got)
	}
	if strings.Count(got, `"speaker"`) != 1 {
		t.Errorf("重写而不是追加，应只剩一拍：%s", got)
	}
}

func TestApplyIdempotent(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "heiwa", 0.0, 1.0, "heiwa", 1.0, 2.0)
	path := writeScene(t, root, "scene-001", 1.2, castBlockNoBeats)
	writeScene(t, root, "scene-002", 0.8, castBlockNoBeats)
	if _, err := Apply(root); err != nil {
		t.Fatalf("第一次 Apply 失败：%v", err)
	}
	first := read(t, path)
	report, err := Apply(root)
	if err != nil {
		t.Fatalf("第二次 Apply 失败：%v", err)
	}
	if read(t, path) != first {
		t.Fatalf("重复执行必须得到逐字节相同的结果：\n第一次:\n%s\n第二次:\n%s", first, read(t, path))
	}
	if report.Updated != 0 {
		t.Errorf("第二次执行没有任何内容变化，Updated 应为 0，得到 %d", report.Updated)
	}
}

// TestApplySilentSceneGetsEmptyBeats 带 cast 块但整段没人说话的镜头（角色
// 在台上做反应）：写空数组，而不是把 beats 键留空缺——空数组是"算过了，
// 这一镜确实没人说话"，缺键读起来是"还没算"。
func TestApplySilentSceneGetsEmptyBeats(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "heiwa", 0.0, 1.0)
	writeScene(t, root, "scene-001", 1.0, castBlockNoBeats)
	path := writeScene(t, root, "scene-002", 2.0, castBlockNoBeats)
	if _, err := Apply(root); err != nil {
		t.Fatalf("Apply 应当成功：%v", err)
	}
	got := read(t, path)
	if !strings.Contains(got, `"beats": []`) {
		t.Fatalf("无台词覆盖的镜头应写出空数组 beats: []，得到：%s", got)
	}
}

// TestApplyKeepsVoiceOverSpeaker 画外音：说话人不在 on_stage 里同样要出节拍，
// 否则 am validate cast 会报这一行没被任何镜头覆盖。
func TestApplyKeepsVoiceOverSpeaker(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "zhaocai", 0.0, 1.0)
	emptyStage := `,
  "cast": {
    "pack_dir": "cast-pack",
    "ground_y": 0.78,
    "on_stage": []
  }`
	path := writeScene(t, root, "scene-001", 1.0, emptyStage)
	if _, err := Apply(root); err != nil {
		t.Fatalf("Apply 应当成功：%v", err)
	}
	if !strings.Contains(read(t, path), `"speaker": "zhaocai"`) {
		t.Fatalf("画外音说话人的台词也必须出节拍：%s", read(t, path))
	}
}

// TestApplyRejectsCoveredSceneWithoutCastBlock 有台词盖过、却没有 cast 块的
// 镜头：命令不能替 agent 编 pack_dir / ground_y，只能点名报错。
func TestApplyRejectsCoveredSceneWithoutCastBlock(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "heiwa", 0.0, 1.0)
	writeScene(t, root, "scene-001", 1.0, "")
	_, err := Apply(root)
	if err == nil {
		t.Fatal("有台词盖过却没有 cast 块时必须报错")
	}
	if !strings.Contains(err.Error(), "scene-001") || !strings.Contains(err.Error(), "cast 块") {
		t.Fatalf("错误信息应点名镜头与缺失的 cast 块，得到：%v", err)
	}
}

// TestApplyRejectsLineBeyondScenes 镜头时长之和短于对白：不能截断，要报错，
// 且一个 scene.json 都不许改（要么全对要么不动）。
func TestApplyRejectsLineBeyondScenes(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "heiwa", 0.0, 1.0, "heiwa", 1.0, 3.0)
	path := writeScene(t, root, "scene-001", 1.5, castBlockNoBeats)
	before := read(t, path)
	_, err := Apply(root)
	if err == nil {
		t.Fatal("有台词落在所有镜头之外时必须报错")
	}
	if !strings.Contains(err.Error(), "第 2 行") || !strings.Contains(err.Error(), "duration_seconds") {
		t.Fatalf("错误信息应点名第几行并指向镜头时长，得到：%v", err)
	}
	if read(t, path) != before {
		t.Error("报错时不得留下改了一半的 scene.json")
	}
}

func TestApplyRejectsMissingDialogue(t *testing.T) {
	root := t.TempDir()
	writeScene(t, root, "scene-001", 1.0, castBlockNoBeats)
	_, err := Apply(root)
	if err == nil || !strings.Contains(err.Error(), "dialogue.json") {
		t.Fatalf("缺 dialogue.json 时应报错并点名文件，得到：%v", err)
	}
}

// TestApplyRejectsOverlappingLines 输入本身就有重叠时，切出来的节拍必然重叠，
// 写进 scene.json 会被 scene.Load 拒绝。与其写出一个自己都加载不了的产物，
// 不如在源头报错。
func TestApplyRejectsOverlappingLines(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "heiwa", 0.0, 1.2, "zhaocai", 1.0, 2.0)
	writeScene(t, root, "scene-001", 2.0, castBlockNoBeats)
	_, err := Apply(root)
	if err == nil || !strings.Contains(err.Error(), "重叠") {
		t.Fatalf("dialogue.json 台词行重叠时应报错，得到：%v", err)
	}
}

// TestApplyLeavesPlainScenesAlone 老模式零破坏：没有 cast 块、也没有台词
// 盖过的镜头，scene.json 一个字节都不许动。
func TestApplyLeavesPlainScenesAlone(t *testing.T) {
	root := t.TempDir()
	writeDialogue(t, root, "heiwa", 0.0, 1.0)
	writeScene(t, root, "scene-001", 1.0, castBlockNoBeats)
	path := writeScene(t, root, "scene-002", 2.0, "")
	before := read(t, path)
	if _, err := Apply(root); err != nil {
		t.Fatalf("Apply 应当成功：%v", err)
	}
	if read(t, path) != before {
		t.Fatalf("无 cast 块又无台词的老镜头不得被改写：\n改前:\n%s\n改后:\n%s", before, read(t, path))
	}
}
