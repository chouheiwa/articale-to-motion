package castbeats

import (
	"encoding/json"
	"fmt"
	"github.com/chouheiwa/articale-to-motion/internal/scene"
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

// castProject 建一个多角色项目根。只放一个 cast.yaml：本命令只看它存在与否
// （cast.yaml 存在与否就是叙事模式本身），不读里面的内容。
func castProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "cast.yaml"), "schema: cast/v1\npacks: []\n")
	return root
}

// TestApplyWritesBeatsAndKeepsOtherFields 是本命令的主路径：cast 块里原本
// 没有 beats，跑完之后 beats 出现且是镜头本地时间，而 pack_dir / ground_y /
// on_stage / text 一个字符都不许变。
func TestApplyWritesBeatsAndKeepsOtherFields(t *testing.T) {
	root := castProject(t)
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
	root := castProject(t)
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
	root := castProject(t)
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
	root := castProject(t)
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
	root := castProject(t)
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
	root := castProject(t)
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
	root := castProject(t)
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
	root := castProject(t)
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
	root := castProject(t)
	writeDialogue(t, root, "heiwa", 0.0, 1.2, "zhaocai", 1.0, 2.0)
	writeScene(t, root, "scene-001", 2.0, castBlockNoBeats)
	_, err := Apply(root)
	if err == nil || !strings.Contains(err.Error(), "重叠") {
		t.Fatalf("dialogue.json 台词行重叠时应报错，得到：%v", err)
	}
}

// TestApplyRejectsTinyOverlapBetweenFloatEpsAndMinBeatSeconds 钉住跨模块容差
// 错配的修复：6e-7 秒的重叠落在 (1e-9, 1e-6] 区间——大于 scene 包判定 beats
// 重叠的容差（floatEps，1e-9），却小于本包曾经拿来当重叠容差用的
// minBeatSeconds（1e-6）。旧实现会在这里放行，切出的节拍写进 scene.json
// 之后又会被 scene.Load 以"时间重叠"拒绝：命令退出码 0，产物却过不了校验，
// 重跑也不会自愈，用户只能手改 dialogue.json。
//
// 现在两边共用 scene.BeatOverlapToleranceSeconds，这种输入必须在这里就被拒绝。
func TestApplyRejectsTinyOverlapBetweenFloatEpsAndMinBeatSeconds(t *testing.T) {
	root := castProject(t)
	// heiwa 止于 1.0000006，zhaocai 起于 1.0——重叠恰好 6e-7 秒。
	writeDialogue(t, root, "heiwa", 0.0, 1.0000006, "zhaocai", 1.0, 2.0)
	writeScene(t, root, "scene-001", 2.0, castBlockNoBeats)

	_, err := Apply(root)
	if err == nil || !strings.Contains(err.Error(), "重叠") {
		t.Fatalf("6e-7 秒的重叠应当在 am dialogue beats 阶段就被拒绝，得到：%v", err)
	}

	// 幂等：重跑必须得到完全相同的拒绝，而不是在中途写出一个自己都加载
	// 不了的 scene.json 之后才报错。
	_, err2 := Apply(root)
	if err2 == nil || err2.Error() != err.Error() {
		t.Fatalf("重跑应得到与第一次完全相同的拒绝，得到：%v", err2)
	}
}

// TestLoadDialogueLinesErrorsUseDialogueRelPathConstant 钉住"错误信息硬编码
// 路径"这条缺陷的修复：把包变量 DialogueRelPath 改成一个独一无二的标记值，
// 逐一触发 loadDialogueLines 的每条报错分支，断言消息里出现的是这个标记，
// 而不是写死的 "production/dialogue.json" 字面量。落地路径以后一改，
// 这条测试会先于用户发现报错文案对不上（若退回硬编码字面量，本测试立即变红）。
func TestLoadDialogueLinesErrorsUseDialogueRelPathConstant(t *testing.T) {
	const marker = "MUTATED-MARKER-DIR/dialogue.json"
	original := DialogueRelPath
	DialogueRelPath = marker
	defer func() { DialogueRelPath = original }()

	assertUsesMarker := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("期待报错，得到 nil")
		}
		if !strings.Contains(err.Error(), marker) {
			t.Fatalf("报错应引用 DialogueRelPath 常量（当前标记 %q），得到：%v", marker, err)
		}
		if strings.Contains(err.Error(), "production/dialogue.json") {
			t.Fatalf("报错不该再包含硬编码的旧路径字面量：%v", err)
		}
	}

	t.Run("找不到文件", func(t *testing.T) {
		root := t.TempDir()
		_, err := loadDialogueLines(filepath.Join(root, marker))
		assertUsesMarker(t, err)
	})

	t.Run("不是合法JSON", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, marker)
		write(t, p, "不是 JSON")
		_, err := loadDialogueLines(p)
		assertUsesMarker(t, err)
	})

	t.Run("schema不对", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, marker)
		write(t, p, `{"schema":"wrong","lines":[]}`)
		_, err := loadDialogueLines(p)
		assertUsesMarker(t, err)
	})

	t.Run("没有任何行", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, marker)
		write(t, p, `{"schema":"cast-dialogue/v1","lines":[]}`)
		_, err := loadDialogueLines(p)
		assertUsesMarker(t, err)
	})

	t.Run("时间倒挂", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, marker)
		write(t, p, `{"schema":"cast-dialogue/v1","lines":[
			{"srtIndex":1,"speaker":"heiwa","startSeconds":1.0,"endSeconds":0.5}
		]}`)
		_, err := loadDialogueLines(p)
		assertUsesMarker(t, err)
	})

	t.Run("与上一行重叠", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, marker)
		write(t, p, `{"schema":"cast-dialogue/v1","lines":[
			{"srtIndex":1,"speaker":"heiwa","startSeconds":0.0,"endSeconds":1.2},
			{"srtIndex":2,"speaker":"zhaocai","startSeconds":1.0,"endSeconds":2.0}
		]}`)
		_, err := loadDialogueLines(p)
		assertUsesMarker(t, err)
	})

	t.Run("有台词行没有被任何镜头盖住", func(t *testing.T) {
		root := castProject(t)
		p := filepath.Join(root, marker)
		write(t, p, `{"schema":"cast-dialogue/v1","lines":[
			{"srtIndex":1,"speaker":"heiwa","startSeconds":0.0,"endSeconds":3.0}
		]}`)
		writeScene(t, root, "scene-001", 1.0, castBlockNoBeats)
		_, err := Apply(root)
		assertUsesMarker(t, err)
	})
}

// TestApplyLeavesPlainScenesAlone 老模式零破坏：没有 cast 块、也没有台词
// 盖过的镜头，scene.json 一个字节都不许动。
func TestApplyLeavesPlainScenesAlone(t *testing.T) {
	root := castProject(t)
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

const castBlockEmptyOnStage = `,
  "cast": {
    "pack_dir": "cast-pack",
    "ground_y": 0.78,
    "on_stage": []
  }`

// TestApplyProducesEmptyOnStageWithEmptyBeatsForCoverageFreeScene 钉住一个
// 以实测确认过的事实（不是猜的）：给一个完全没有台词覆盖、却预先写了
// on_stage:[] 空 cast 块的纯 B-roll 镜头跑 Apply，会正常写出 beats:[]——
// on_stage 和 beats 都是空数组的组合是本命令的正常产物，不是坏输入。
//
// internal/scene 的 castSection 必须能处理这个组合而不自相矛盾（见
// TestBuildPromptCastSectionEmptyStageAndBeats），validateCast 也不能
// 拒绝它——拒绝会让本命令自己的产物过不了校验。
func TestApplyProducesEmptyOnStageWithEmptyBeatsForCoverageFreeScene(t *testing.T) {
	root := castProject(t)
	writeDialogue(t, root, "heiwa", 0.0, 1.0)
	writeScene(t, root, "scene-001", 1.0, castBlockNoBeats)
	path2 := writeScene(t, root, "scene-002", 1.0, castBlockEmptyOnStage)

	if _, err := Apply(root); err != nil {
		t.Fatalf("Apply 不应报错：%v", err)
	}
	body := read(t, path2)
	if !strings.Contains(body, `"on_stage": []`) {
		t.Fatalf("scene-002 的 on_stage 应保持为 []，得到：%s", body)
	}
	if !strings.Contains(body, `"beats": []`) {
		t.Fatalf("scene-002 没有台词覆盖时应写出 beats: []，得到：%s", body)
	}
}

// TestApplyRejectsNonCastProject 老项目误跑到这条命令时，要说清"本项目不是
// 多角色项目"，而不是甩一句"找不到 dialogue.json"——后者会让人以为少跑了
// am dialogue assemble，接着去装配一个根本不存在的对白。
func TestApplyRejectsNonCastProject(t *testing.T) {
	root := t.TempDir() // 故意不建 cast.yaml
	writeDialogue(t, root, "heiwa", 0.0, 1.0)
	writeScene(t, root, "scene-001", 1.0, castBlockNoBeats)
	_, err := Apply(root)
	if err == nil {
		t.Fatal("没有 cast.yaml 的项目不该被这条命令处理")
	}
	if !strings.Contains(err.Error(), "cast.yaml") || !strings.Contains(err.Error(), "不是多角色项目") {
		t.Fatalf("错误信息应点名 cast.yaml 并说清本项目不是多角色项目，得到：%v", err)
	}
	if strings.Contains(err.Error(), "dialogue.json") {
		t.Fatalf("不该把人往 dialogue.json 上引：%v", err)
	}
}

// TestApplyIgnoresStaleOutOfRangeBeats 钉住一个死锁：把某镜头的
// duration_seconds 改小、旧 beats 还越界时，scene.Load 拒绝这份 scene.json，
// 而本命令正是修它的工具——读取时要是也走 scene.Load，工具就把自己锁在
// 门外，用户只能手改 JSON 脱困。
//
// beats 是本命令的输出，不该是它的输入：读镜头时只取 duration_seconds /
// pack_dir / on_stage 这些它真正需要的字段，既有 beats 一概不看。
func TestApplyIgnoresStaleOutOfRangeBeats(t *testing.T) {
	root := castProject(t)
	writeDialogue(t, root, "heiwa", 0.0, 1.0, "heiwa", 1.0, 2.0)
	stale := `,
  "cast": {
    "pack_dir": "cast-pack",
    "ground_y": 0.78,
    "on_stage": [
      {"id": "heiwa", "x": 0.3, "pose": "idle", "facing": "right"}
    ],
    "beats": [
      {"speaker": "heiwa", "start": 0, "end": 1.9}
    ]
  }`
	// 这一镜原本 2.0 秒（旧节拍止于 1.9 合法），现在被改小到 1.0 秒。
	path := writeScene(t, root, "scene-001", 1.0, stale)
	writeScene(t, root, "scene-002", 1.0, castBlockNoBeats)

	// 前提核实：这份 scene.json 现在确实加载不了，死锁的门是关着的。
	if _, err := scene.Load(filepath.Dir(path)); err == nil {
		t.Fatal("前提不成立：越界的旧 beats 本应被 scene.Load 拒绝")
	}

	if _, err := Apply(root); err != nil {
		t.Fatalf("越界的旧节拍不该挡住重算：%v", err)
	}

	// 重算之后这份 scene.json 必须重新可加载，且节拍是新算出来的。
	loaded, err := scene.Load(filepath.Dir(path))
	if err != nil {
		t.Fatalf("重算之后 scene.Load 应当通过：%v", err)
	}
	if len(loaded.Cast.Beats) != 1 || loaded.Cast.Beats[0].End != 1.0 {
		t.Fatalf("scene-001 的节拍应重算成 [0, 1.0]，得到 %+v", loaded.Cast.Beats)
	}
	second, err := scene.Load(filepath.Join(root, "scenes", "scene-002"))
	if err != nil {
		t.Fatalf("scene-002 应当能加载：%v", err)
	}
	if len(second.Cast.Beats) != 1 || second.Cast.Beats[0].End != 1.0 {
		t.Fatalf("scene-002 的节拍应是本地 [0, 1.0]，得到 %+v", second.Cast.Beats)
	}
}

// TestApplyRejectsSceneWithoutDuration 轻量解析放弃了 scene.Load 的完整校验，
// 但它自己要用的字段仍然要查：没有 duration_seconds 就算不出镜头起点，
// 静默当成 0 会把后面全部镜头的节拍一起带偏。
func TestApplyRejectsSceneWithoutDuration(t *testing.T) {
	root := castProject(t)
	writeDialogue(t, root, "heiwa", 0.0, 1.0)
	write(t, filepath.Join(root, "scenes", "scene-001", "scene.json"),
		`{"id":"scene-001","output":"out.mp4","transcript":"transcript.txt","text":"占位"}`)
	_, err := Apply(root)
	if err == nil || !strings.Contains(err.Error(), "duration_seconds") {
		t.Fatalf("缺 duration_seconds 时应点名报错，得到：%v", err)
	}
}
