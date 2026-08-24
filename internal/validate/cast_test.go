package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestValidateCastSceneBeatsCoverDialogue(t *testing.T) {
	root := castProject(t, sceneMissingBeat())
	problems := CastProblems(root, "minimax")
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), "镜头") {
		t.Fatalf("期望报镜头 beats 未覆盖 dialogue.json，得到 %v", problems)
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
