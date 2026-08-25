package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/dialogue"
)

// castPackYAML 是测试用最小可自洽角色包清单模板：%s 依次是 id、name、
// minimax voiceId 所在行、bailian voiceId 所在行。缺声明某个 provider 时
// 传空字符串即可整行去掉。
const castPackYAMLTemplate = `schema: cast/v1
id: %s
name: %s
summary: 测试角色
voice:
%s
rig:
  file: rig.svg
  viewBox: [0, 0, 400, 520]
  baselineY: 512
  joints:
    head:     { pivot: [200, 168], rotate: [-18, 18] }
    frontLeg: { pivot: [176, 330], rotate: [-40, 55] }
scale:
  heightRatio: [0.22, 0.32]
poses:
  idle: {}
`

const castPackSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 520">
  <g id="j-head"><circle cx="200" cy="168" r="60" fill="#fff" stroke="#000" stroke-width="6"/></g>
  <g id="j-frontLeg"><path d="M176 330 L176 420" stroke="#000" stroke-width="6"/></g>
</svg>
`

// writeCastPack 在 dir 下写出 character.yaml + rig.svg，可选跳过 dna.md
// （withDNA=false 时不写，用于覆盖率测试）。voiceLines 是 voice 映射下的
// YAML 行（已含缩进），跳过某个 provider 就不传对应那一行。
func writeCastPack(t *testing.T, dir, id, name string, voiceLines []string, withDNA bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(castPackYAMLTemplate, id, name, strings.Join(voiceLines, "\n"))
	if err := os.WriteFile(filepath.Join(dir, "character.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rig.svg"), []byte(castPackSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	if withDNA {
		if err := os.WriteFile(filepath.Join(dir, "dna.md"), []byte("# "+name+" 角色 DNA\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

var bothVoiceLines = []string{
	`  minimax: { voiceId: "v-1", speed: 0.98 }`,
	`  bailian: { voiceId: "v-2", rate: 0.98 }`,
}

func voiceLinesWithout(provider string) []string {
	var out []string
	for _, line := range bothVoiceLines {
		if !strings.HasPrefix(strings.TrimSpace(line), provider+":") {
			out = append(out, line)
		}
	}
	return out
}

// castDialogueLine 是构造 dialogue.json/srt 用的最小行描述。
type castDialogueLine struct {
	speaker      string
	startSeconds float64
	endSeconds   float64
	text         string
}

var defaultCastLines = []castDialogueLine{
	{speaker: "heiwa", startSeconds: 0, endSeconds: 1, text: "台词一"},
	{speaker: "zhaocai", startSeconds: 1, endSeconds: 2, text: "台词二"},
	{speaker: "heiwa", startSeconds: 2, endSeconds: 3, text: "台词三"},
	{speaker: "zhaocai", startSeconds: 3, endSeconds: 4, text: "台词四"},
}

func writeDialogueJSON(t *testing.T, path string, lines []castDialogueLine) {
	t.Helper()
	type jsonLine struct {
		SRTIndex     int     `json:"srtIndex"`
		Speaker      string  `json:"speaker"`
		StartSeconds float64 `json:"startSeconds"`
		EndSeconds   float64 `json:"endSeconds"`
	}
	payload := struct {
		Schema   string     `json:"schema"`
		Segments []struct{} `json:"segments"`
		Lines    []jsonLine `json:"lines"`
	}{Schema: "cast-dialogue/v1"}
	for i, line := range lines {
		payload.Lines = append(payload.Lines, jsonLine{
			SRTIndex: i + 1, Speaker: line.speaker,
			StartSeconds: line.startSeconds, EndSeconds: line.endSeconds,
		})
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func clockTC(seconds float64) string {
	total := int64(seconds*1000 + 0.5)
	ms := total % 1000
	total /= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", total/3600, total%3600/60, total%60, ms)
}

func writeCastSRT(t *testing.T, path string, lines []castDialogueLine) {
	t.Helper()
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, clockTC(line.startSeconds), clockTC(line.endSeconds), line.text)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// castSceneBeat 是构造 scene.json 里 cast.beats 用的最小描述，时间是镜头本地时间。
type castSceneBeat struct {
	speaker    string
	start, end float64
}

type castSceneSpec struct {
	id              string
	durationSeconds float64
	beats           []castSceneBeat
	skipDNAFor      string // 该角色 id 在这个镜头的本地角色包目录不写 dna.md
}

// writeCastScene 写出一个完整可通过 scene.Load 的镜头：cast.pack_dir 指向
// 镜头目录内自带的一份角色包拷贝（heiwa/zhaocai 都在，不论这个镜头是否
// 真的用到），满足 validateCast 对 on_stage/pose/view 的全部前置校验。
func writeCastScene(t *testing.T, scenesDir string, spec castSceneSpec) {
	t.Helper()
	dir := filepath.Join(scenesDir, spec.id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "transcript.txt"), []byte("占位"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCastPack(t, filepath.Join(dir, "cast-pack", "heiwa"), "heiwa", "黑娃", bothVoiceLines, spec.skipDNAFor != "heiwa")
	writeCastPack(t, filepath.Join(dir, "cast-pack", "zhaocai"), "zhaocai", "招财", bothVoiceLines, spec.skipDNAFor != "zhaocai")

	type jsonBeat struct {
		Speaker string  `json:"speaker"`
		Start   float64 `json:"start"`
		End     float64 `json:"end"`
	}
	type jsonActor struct {
		ID     string  `json:"id"`
		X      float64 `json:"x"`
		Pose   string  `json:"pose"`
		Facing string  `json:"facing"`
	}
	sceneJSON := struct {
		ID              string  `json:"id"`
		DurationSeconds float64 `json:"duration_seconds"`
		Output          string  `json:"output"`
		Transcript      string  `json:"transcript"`
		Text            string  `json:"text"`
		Cast            struct {
			PackDir string      `json:"pack_dir"`
			GroundY float64     `json:"ground_y"`
			OnStage []jsonActor `json:"on_stage"`
			Beats   []jsonBeat  `json:"beats"`
		} `json:"cast"`
	}{
		ID: spec.id, DurationSeconds: spec.durationSeconds,
		Output: "out.mp4", Transcript: "transcript.txt", Text: "占位文案",
	}
	sceneJSON.Cast.PackDir = "cast-pack"
	sceneJSON.Cast.GroundY = 0.78
	sceneJSON.Cast.OnStage = []jsonActor{
		{ID: "heiwa", X: 0.3, Pose: "idle", Facing: "right"},
		{ID: "zhaocai", X: 0.7, Pose: "idle", Facing: "left"},
	}
	for _, beat := range spec.beats {
		sceneJSON.Cast.Beats = append(sceneJSON.Cast.Beats, jsonBeat{Speaker: beat.speaker, Start: beat.start, End: beat.end})
	}
	body, err := json.MarshalIndent(sceneJSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scene.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

var defaultCastScenes = []castSceneSpec{
	{id: "scene-001", durationSeconds: 2, beats: []castSceneBeat{
		{speaker: "heiwa", start: 0, end: 1},
		{speaker: "zhaocai", start: 1, end: 2},
	}},
	{id: "scene-002", durationSeconds: 2, beats: []castSceneBeat{
		{speaker: "heiwa", start: 0, end: 1},
		{speaker: "zhaocai", start: 1, end: 2},
	}},
}

// castFixtureOptions 是 castProject 的可选缺陷注入点，每个选项各自破坏一处，
// 便于每条测试各自基于同一份可通过的多角色项目做单点破坏。
type castFixtureOptions struct {
	dropVoiceFromHeiwa string
	dialogueLines      []castDialogueLine
	srtLines           []castDialogueLine
	scenes             []castSceneSpec
	emptyRoster        bool
	dropRosterDNAFor   string
}

func withoutVoice(provider string) func(*castFixtureOptions) {
	return func(o *castFixtureOptions) { o.dropVoiceFromHeiwa = provider }
}

func dialogueMissingLastLine() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		o.dialogueLines = defaultCastLines[:len(defaultCastLines)-1]
	}
}

func sceneMissingBeat() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		scenes := make([]castSceneSpec, len(defaultCastScenes))
		copy(scenes, defaultCastScenes)
		// 去掉 scene-002 里 zhaocai 的那一拍：全局时间线上 3-4 秒（第 4 行）
		// 就此没有任何镜头的 cast.beats 覆盖。
		scenes[1] = castSceneSpec{id: "scene-002", durationSeconds: 2, beats: []castSceneBeat{
			{speaker: "heiwa", start: 0, end: 1},
		}}
		o.scenes = scenes
	}
}

func missingDNAFor(id string) func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		scenes := make([]castSceneSpec, len(defaultCastScenes))
		copy(scenes, defaultCastScenes)
		scenes[0].skipDNAFor = id
		o.scenes = scenes
	}
}

func emptyRoster() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) { o.emptyRoster = true }
}

// dialogueSpeakerNotInRoster 把最后一行的说话人换成班底里不存在的
// "ghost"——覆盖 spec §7.3 明列、此前无测试守护的一条：dialogue.json 的
// 说话人必须都在 cast.yaml 班底内。timing 不变，只换 speaker 字段，避免
// 牵连 dialogueLineProblems 的时序/重叠校验。
func dialogueSpeakerNotInRoster() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		lines := make([]castDialogueLine, len(defaultCastLines))
		copy(lines, defaultCastLines)
		lines[len(lines)-1].speaker = "ghost"
		o.dialogueLines = lines
		o.srtLines = lines
	}
}

// dialogueLinesOverlap 让第 2 行提前 0.5 秒起，与第 1 行的 [0,1] 区间重叠
// 0.5 秒——覆盖 spec §7.3 明列、此前无测试守护的另一条：dialogue.json 逐行
// 不得重叠。只改时间，不改说话人，与"说话人不在班底"这条互相独立。
func dialogueLinesOverlap() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		lines := []castDialogueLine{
			{speaker: "heiwa", startSeconds: 0, endSeconds: 1, text: "台词一"},
			{speaker: "zhaocai", startSeconds: 0.5, endSeconds: 2, text: "台词二与上一行重叠"},
			{speaker: "heiwa", startSeconds: 2, endSeconds: 3, text: "台词三"},
			{speaker: "zhaocai", startSeconds: 3, endSeconds: 4, text: "台词四"},
		}
		o.dialogueLines = lines
		o.srtLines = lines
	}
}

// missingDNAFromRosterSource 删掉班底源目录（cast/<id>/）里的 dna.md，
// 模拟 am cast new 生成后被人手误删；镜头目录内的拷贝（若有）不受影响，
// 用来单独打这条检查——它跟"镜头引用路径缺 dna.md"是两条互相独立的检查。
func missingDNAFromRosterSource(id string) func(*castFixtureOptions) {
	return func(o *castFixtureOptions) { o.dropRosterDNAFor = id }
}

// lineSpanningSceneCut 构造一句台词横跨镜头切点的场景：zhaocai 的第 2 行
// [1,3] 秒被拆镜头切成两拍，分别落在 scene-001 与 scene-002 里，全局时间
// 换算后首尾相接（scene-001 止于 2，scene-002 起于 2）。这是"按语义拆镜头"
// 的正常产物，不是缺陷——合并覆盖判断必须认得出这两拍拼起来就是完整的一行。
func lineSpanningSceneCut() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		lines := []castDialogueLine{
			{speaker: "heiwa", startSeconds: 0, endSeconds: 1, text: "台词一"},
			{speaker: "zhaocai", startSeconds: 1, endSeconds: 3, text: "台词二横跨切点"},
			{speaker: "heiwa", startSeconds: 3, endSeconds: 4, text: "台词三"},
		}
		o.dialogueLines = lines
		o.srtLines = lines
		o.scenes = []castSceneSpec{
			{id: "scene-001", durationSeconds: 2, beats: []castSceneBeat{
				{speaker: "heiwa", start: 0, end: 1},
				{speaker: "zhaocai", start: 1, end: 2},
			}},
			{id: "scene-002", durationSeconds: 2, beats: []castSceneBeat{
				{speaker: "zhaocai", start: 0, end: 1},
				{speaker: "heiwa", start: 1, end: 2},
			}},
		}
	}
}

// lineSpanningSceneCutWithGap 在 lineSpanningSceneCut 的基础上，把
// scene-002 里 zhaocai 那一拍的本地起点从 0 错开到 0.2——换算回全局后
// 变成 [2.2,3]，与 scene-001 的 zhaocai 拍 [1,2] 之间留了 [2,2.2] 的真实
// 缺口。合并逻辑不能把这种真实缺口也糊过去，仍然必须报第 2 行未覆盖。
func lineSpanningSceneCutWithGap() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		lineSpanningSceneCut()(o)
		scenes := make([]castSceneSpec, len(o.scenes))
		copy(scenes, o.scenes)
		scenes[1] = castSceneSpec{id: "scene-002", durationSeconds: 2, beats: []castSceneBeat{
			{speaker: "zhaocai", start: 0.2, end: 1},
			{speaker: "heiwa", start: 1, end: 2},
		}}
		o.scenes = scenes
	}
}

// sceneWithoutBeats 把 scene-002 的 beats 整个清空：它覆盖的第 3、4 行
// 于是全部未覆盖，症状与"镜头时长累计漂移"完全一样（从某一行起一路延伸
// 到最后一行），但病因完全不同——被删掉的那条启发式正是在这里给出误导性
// 结论的。
func sceneWithoutBeats() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		scenes := make([]castSceneSpec, len(defaultCastScenes))
		copy(scenes, defaultCastScenes)
		scenes[1] = castSceneSpec{id: "scene-002", durationSeconds: 2}
		o.scenes = scenes
	}
}

// sceneDurationDrift 模拟"某个镜头的 duration_seconds 与实际配音时长不
// 一致"这一类病根：scene-001 的两条 beats 与声明的 2 秒时长本身自洽（能
// 通过 scene.Load），但真实台词（dialogue.json）里 scene-001 其实用了
// 2.5 秒——后续所有镜头的全局起点都基于声明的 2 秒去累加，于是从这一点
// 之后，所有台词的换算区间都统一偏移了 0.5 秒，表现为“从某一行起，后面
// 所有行全部未覆盖”，而不是零散的漏拍。
func sceneDurationDrift() func(*castFixtureOptions) {
	return func(o *castFixtureOptions) {
		lines := []castDialogueLine{
			{speaker: "heiwa", startSeconds: 0, endSeconds: 1, text: "台词一"},
			{speaker: "zhaocai", startSeconds: 1, endSeconds: 2, text: "台词二"},
			{speaker: "heiwa", startSeconds: 2.5, endSeconds: 3.5, text: "台词三"},
			{speaker: "zhaocai", startSeconds: 3.5, endSeconds: 4.5, text: "台词四"},
		}
		o.dialogueLines = lines
		o.srtLines = lines
		o.scenes = []castSceneSpec{
			{id: "scene-001", durationSeconds: 2, beats: []castSceneBeat{
				{speaker: "heiwa", start: 0, end: 1},
				{speaker: "zhaocai", start: 1, end: 2},
			}},
			{id: "scene-002", durationSeconds: 2, beats: []castSceneBeat{
				{speaker: "heiwa", start: 0, end: 1},
				{speaker: "zhaocai", start: 1, end: 2},
			}},
		}
	}
}

// castProject 搭一个可通过全部校验的多角色项目，再按 opts 逐个破坏。
func castProject(t *testing.T, opts ...func(*castFixtureOptions)) string {
	t.Helper()
	options := castFixtureOptions{
		dialogueLines: defaultCastLines,
		srtLines:      defaultCastLines,
		scenes:        defaultCastScenes,
	}
	for _, opt := range opts {
		opt(&options)
	}

	root := t.TempDir()

	rosterBody := "schema: cast/v1\npacks: []\ndefaults:\n  ground_y: 0.78\n  gap_ms: { turn: 240, interject: 100 }\n"
	if !options.emptyRoster {
		rosterBody = "schema: cast/v1\npacks: [cast/heiwa, cast/zhaocai]\ndefaults:\n  ground_y: 0.78\n  gap_ms: { turn: 240, interject: 100 }\n"
	}
	if err := os.WriteFile(filepath.Join(root, "cast.yaml"), []byte(rosterBody), 0o644); err != nil {
		t.Fatal(err)
	}

	heiwaVoices := bothVoiceLines
	if options.dropVoiceFromHeiwa != "" {
		heiwaVoices = voiceLinesWithout(options.dropVoiceFromHeiwa)
	}
	writeCastPack(t, filepath.Join(root, "cast", "heiwa"), "heiwa", "黑娃", heiwaVoices, true)
	writeCastPack(t, filepath.Join(root, "cast", "zhaocai"), "zhaocai", "招财", bothVoiceLines, true)
	if options.dropRosterDNAFor != "" {
		if err := os.Remove(filepath.Join(root, "cast", options.dropRosterDNAFor, "dna.md")); err != nil {
			t.Fatal(err)
		}
	}

	writeDialogueJSON(t, filepath.Join(root, "production", "dialogue.json"), options.dialogueLines)
	writeCastSRT(t, filepath.Join(root, "transcription-production.srt"), options.srtLines)

	scenesDir := filepath.Join(root, "scenes")
	for _, spec := range options.scenes {
		writeCastScene(t, scenesDir, spec)
	}

	return root
}

func TestValidateCastNilWithoutRoster(t *testing.T) {
	root := t.TempDir() // 没有 cast.yaml：老项目
	if problems := CastProblems(root, "minimax"); problems != nil {
		t.Fatalf("没有 cast.yaml 时应直接返回 nil，得到 %v", problems)
	}
}

func TestValidateCastValidProjectHasNoProblems(t *testing.T) {
	root := castProject(t)
	if problems := CastProblems(root, "minimax"); len(problems) != 0 {
		t.Fatalf("完全自洽的项目不应报任何问题，得到 %v", problems)
	}
}

func TestValidateCastRosterMissingVoiceForProvider(t *testing.T) {
	// 班底里的角色没有当前 TTS_PROVIDER 的音色时必须失败。
	root := castProject(t, withoutVoice("bailian"))
	problems := CastProblems(root, "bailian")
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), "bailian") {
		t.Fatalf("期望报缺 bailian 音色，得到 %v", problems)
	}
}

func TestValidateCastDialogueCoversEverySRTLine(t *testing.T) {
	root := castProject(t, dialogueMissingLastLine())
	problems := CastProblems(root, "minimax")
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), "字幕") {
		t.Fatalf("期望报 dialogue.json 未覆盖全部字幕行，得到 %v", problems)
	}
}

// TestValidateCastDialogueSpeakerMustBeInRoster 覆盖遗留缺口④之一：spec
// §7.3 明列"dialogue.json 的说话人必须都在班底内"，此前没有任何测试守护
// 这条——把 CastProblems 里对应的 if 改成 if false（永不拒绝）仍然全绿。
// want 挑"不在班底里"：这是 line.Speaker 查 packs 失败这条分支独有的措辞
// （拼错的说话人名字既进不了 on_stage，也进不了 packs，其余检查用的都是
// 不同的错误文案），不会被别的分支意外撑住。
func TestValidateCastDialogueSpeakerMustBeInRoster(t *testing.T) {
	root := castProject(t, dialogueSpeakerNotInRoster())
	problems := CastProblems(root, "minimax")
	const want = `不在班底里`
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), want) {
		t.Fatalf("期望报 %q，得到 %v", want, problems)
	}
}

// TestValidateCastDialogueLinesMustNotOverlap 覆盖遗留缺口④之二：spec
// §7.3 明列"dialogue.json 逐行重叠"必须被拦下，此前无测试守护。want 挑
// "与上一行重叠"，是 dialogueLineProblems 里这条分支独有的措辞。
func TestValidateCastDialogueLinesMustNotOverlap(t *testing.T) {
	root := castProject(t, dialogueLinesOverlap())
	problems := CastProblems(root, "minimax")
	const want = `与上一行重叠`
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), want) {
		t.Fatalf("期望报 %q，得到 %v", want, problems)
	}
}

func TestValidateCastSceneBeatsCoverDialogue(t *testing.T) {
	root := castProject(t, sceneMissingBeat())
	problems := CastProblems(root, "minimax")
	// 断言完整短语而不是"镜头"两个字：「读取镜头目录失败」这条错误信息里也
	// 含"镜头"，fixture 本身如果因为别的原因坏掉（比如目录读取失败），
	// 这条断言会在错误理由完全不对的情况下误判通过。
	const want = "没有被任何镜头的 cast.beats 覆盖"
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), want) {
		t.Fatalf("期望报 %q，得到 %v", want, problems)
	}
}

// TestValidateCastLineSpanningSceneCutNotMisreported 钉住 Important 1：
// 一句台词按语义拆镜头切成两拍、换算回全局时间后首尾相接，这是正常产物，
// 不该被误报成"未覆盖"。
func TestValidateCastLineSpanningSceneCutNotMisreported(t *testing.T) {
	root := castProject(t, lineSpanningSceneCut())
	problems := CastProblems(root, "minimax")
	if len(problems) != 0 {
		t.Fatalf("台词横跨镜头切点、两拍首尾相接时不应误报，得到 %v", problems)
	}
}

// TestValidateCastLineSpanningSceneCutGapStillReported 证明合并逻辑不会
// 把真实缺口也糊过去：两拍之间留了 0.2 秒的真实间隙时仍要报。
func TestValidateCastLineSpanningSceneCutGapStillReported(t *testing.T) {
	root := castProject(t, lineSpanningSceneCutWithGap())
	problems := CastProblems(root, "minimax")
	joined := strings.Join(problems, "\n")
	if len(problems) == 0 || !strings.Contains(joined, "第 2 行") {
		t.Fatalf("合并后仍有真实缺口时应当继续报第 2 行未覆盖，得到 %v", problems)
	}
}

// TestValidateCastUncoveredLinesPointAtBeatsCommand 取代原先那条启发式
// 漂移提示：有台词行没被盖住时，逐行报告之外要追加一条确定性的出路——
// 去跑 am dialogue beats，由它按 dialogue.json 重算全部节拍，而不是让人
// 逐行手改 beats。用镜头时长漂移这个 fixture，是因为它正是原先那条启发式
// 唯一说得准的场景，新提示在这里同样成立。
func TestValidateCastUncoveredLinesPointAtBeatsCommand(t *testing.T) {
	root := castProject(t, sceneDurationDrift())
	problems := CastProblems(root, "minimax")
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "第 3 行") || !strings.Contains(joined, "第 4 行") {
		t.Fatalf("未覆盖的行仍要逐行报出来，得到 %v", problems)
	}
	if !strings.Contains(joined, "am dialogue beats") {
		t.Fatalf("有未覆盖行时应指向重算节拍的命令，得到 %v", problems)
	}
}

// TestValidateCastSceneWithoutBeatsNotMisreportedAsDrift 钉住被删掉的那条
// 启发式的已知误导：某个镜头压根没写 beats 时，症状同样是"从某一行起后面
// 全部未覆盖"，旧提示会一口咬定是"镜头时长累计漂移"，把人支去改时长——
// 而真正要做的是把节拍补上。现在的提示对两种病因都成立。
func TestValidateCastSceneWithoutBeatsNotMisreportedAsDrift(t *testing.T) {
	root := castProject(t, sceneWithoutBeats())
	problems := CastProblems(root, "minimax")
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "第 3 行") || !strings.Contains(joined, "第 4 行") {
		t.Fatalf("没写 beats 的镜头覆盖的两行都要报出来，得到 %v", problems)
	}
	if strings.Contains(joined, "时长累计漂移") {
		t.Fatalf("镜头没写 beats 不是时长漂移，不得给出这个结论：%v", problems)
	}
	if !strings.Contains(joined, "am dialogue beats") {
		t.Fatalf("应指向重算节拍的命令，得到 %v", problems)
	}
}

// TestValidateCastCleanProjectHasNoBeatsHint 确认那条提示只在真的有未覆盖行
// 时才出现：一个完好的多角色项目不该收到任何提示。
func TestValidateCastCleanProjectHasNoBeatsHint(t *testing.T) {
	root := castProject(t)
	problems := CastProblems(root, "minimax")
	if strings.Contains(strings.Join(problems, "\n"), "am dialogue beats") {
		t.Fatalf("项目完好时不该出现重算节拍的提示，得到 %v", problems)
	}
}

func TestValidateCastEmptyRosterReported(t *testing.T) {
	root := castProject(t, emptyRoster())
	problems := CastProblems(root, "minimax")
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), "班底为空") {
		t.Fatalf("期望报班底为空，得到 %v", problems)
	}
}

func TestValidateCastSceneActorMissingDNA(t *testing.T) {
	root := castProject(t, missingDNAFor("zhaocai"))
	problems := CastProblems(root, "minimax")
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), "dna.md") {
		t.Fatalf("期望报角色缺少 dna.md，得到 %v", problems)
	}
}

// TestValidateCastRosterSourceMissingDNA 钉住 Important 3：班底源目录
// （cast/<id>/）自己的 dna.md 被误删时必须报——即便这个角色在某个镜头里
// 引用的 pack_dir 拷贝仍然完好。这条检查跟"镜头引用路径缺 dna.md"
// （TestValidateCastSceneActorMissingDNA）相互独立，缺一个都会漏掉一种
// 真实的误删场景。
func TestValidateCastRosterSourceMissingDNA(t *testing.T) {
	root := castProject(t, missingDNAFromRosterSource("zhaocai"))
	problems := CastProblems(root, "minimax")
	joined := strings.Join(problems, "\n")
	if len(problems) == 0 || !strings.Contains(joined, "dna.md") || !strings.Contains(joined, "zhaocai") {
		t.Fatalf("期望报班底源目录缺少 dna.md，得到 %v", problems)
	}
}

// TestLoadDialogueResultErrorsUseDialogueRelPathConstant 钉住"错误信息硬编码
// 路径"这条缺陷的修复：把 dialogue.DialogueRelPath 换成一个独一无二的标记值，
// 让 CastProblems 在一个本来完全自洽的项目上也去错误的位置找 dialogue.json，
// 断言报出的消息引用的是这个标记，而不是写死的 "production/dialogue.json"
// 字面量。落地路径以后一改，这条测试会先于用户发现报错文案对不上（若退回
// 硬编码字面量，本测试立即变红）。
func TestLoadDialogueResultErrorsUseDialogueRelPathConstant(t *testing.T) {
	root := castProject(t)
	const marker = "MUTATED-MARKER-DIR/dialogue.json"
	original := dialogue.DialogueRelPath
	dialogue.DialogueRelPath = marker
	defer func() { dialogue.DialogueRelPath = original }()

	problems := CastProblems(root, "minimax")
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, marker) {
		t.Fatalf("报错应引用 DialogueRelPath 常量（当前标记 %q），得到 %v", marker, problems)
	}
	if strings.Contains(joined, "production/dialogue.json") {
		t.Fatalf("报错不该再包含硬编码的旧路径字面量：%v", problems)
	}
}
