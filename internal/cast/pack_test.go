package cast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePack(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "character.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const goodYAML = `schema: cast/v1
id: heiwa
name: 黑娃
summary: 面无表情，认真做一件荒诞但成立的事
voice:
  minimax: { voiceId: "v-1", speed: 0.98, pitch: 0 }
  bailian: { voiceId: "v-2", rate: 0.98, instruction: "平静、真诚的语气" }
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
  pointing: { frontLeg: 48, head: -6 }
`

func TestParsePackReadsAllFields(t *testing.T) {
	pack, err := ParsePack(writePack(t, goodYAML))
	if err != nil {
		t.Fatal(err)
	}
	if pack.ID != "heiwa" || pack.Name != "黑娃" {
		t.Errorf("id/name = %q/%q", pack.ID, pack.Name)
	}
	if pack.Rig.BaselineY != 512 || pack.Rig.ViewBox != [4]float64{0, 0, 400, 520} {
		t.Errorf("rig = %+v", pack.Rig)
	}
	if len(pack.Rig.Joints) != 2 || pack.Rig.Joints["head"].Pivot != [2]float64{200, 168} {
		t.Errorf("joints = %+v", pack.Rig.Joints)
	}
	if pack.Poses["pointing"]["frontLeg"] != 48 {
		t.Errorf("poses = %+v", pack.Poses)
	}
}

func TestParsePackRejects(t *testing.T) {
	cases := []struct {
		name, mutate, want string
	}{
		{"schema 版本不对", "schema: cast/v0", "schema"},
		{"id 与目录名规则不符", "id: Heiwa", "id"},
		{"缺 idle 姿势", "  idle: {}\n", "idle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := goodYAML
			switch tc.name {
			case "schema 版本不对":
				body = strings.Replace(body, "schema: cast/v1", tc.mutate, 1)
			case "id 与目录名规则不符":
				body = strings.Replace(body, "id: heiwa", tc.mutate, 1)
			case "缺 idle 姿势":
				body = strings.Replace(body, tc.mutate, "", 1)
			}
			if _, err := ParsePack(writePack(t, body)); err == nil {
				t.Fatalf("期望报错，含 %q", tc.want)
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("错误信息 = %v，期望含 %q", err, tc.want)
			}
		})
	}
}

func TestVoiceForMissingProvider(t *testing.T) {
	body := strings.Replace(goodYAML, `  bailian: { voiceId: "v-2", rate: 0.98, instruction: "平静、真诚的语气" }`+"\n", "", 1)
	pack, err := ParsePack(writePack(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pack.VoiceFor("bailian"); err == nil {
		t.Fatal("期望报缺失音色")
	}
	if _, err := pack.VoiceFor("minimax"); err != nil {
		t.Fatalf("minimax 应该有音色：%v", err)
	}
}

func TestLoadAggregatesRigProblems(t *testing.T) {
	dir := writePack(t, goodYAML)
	bad := strings.Replace(goodSVG, "</svg>", `<animate attributeName="opacity"/><script>1</script></svg>`, 1)
	if err := os.WriteFile(filepath.Join(dir, "rig.svg"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("期望报错")
	}
	for _, want := range []string{"animate", "script"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息 = %v，期望含 %q（应一次列出全部问题）", err, want)
		}
	}
}

func TestLoadAcceptsGoodPack(t *testing.T) {
	dir := writePack(t, goodYAML)
	if err := os.WriteFile(filepath.Join(dir, "rig.svg"), []byte(goodSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	pack, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pack.ID != "heiwa" {
		t.Errorf("id = %q", pack.ID)
	}
}

const viewsYAML = goodYAML + `views:
  three-quarter:
    file: rig-tq.svg
    viewBox: [0, 0, 400, 520]
    baselineY: 512
    joints:
      head:     { pivot: [186, 170], rotate: [-22, 22] }
      frontLeg: { pivot: [168, 332], rotate: [-40, 55] }
  side:
    file: rig-side.svg
    viewBox: [0, 0, 400, 520]
    baselineY: 512
    joints:
      head:     { pivot: [172, 174], rotate: [-26, 26] }
      frontLeg: { pivot: [160, 334], rotate: [-45, 60] }
turns:
  front->side: { via: [three-quarter], durationMs: 200 }
`

func TestParsePackWithoutViewsKeepsSingleView(t *testing.T) {
	pack, err := ParsePack(writePack(t, goodYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Views) != 0 {
		t.Errorf("未声明 views 时不应凭空造出视图：%+v", pack.Views)
	}
	names := pack.ViewNames()
	if len(names) != 1 || names[0] != DefaultView {
		t.Errorf("ViewNames() = %v，期望只有 %s", names, DefaultView)
	}
	view, err := pack.View(DefaultView)
	if err != nil || view.File != "rig.svg" {
		t.Errorf("默认视图应回落到 rig 字段：%+v %v", view, err)
	}
}

func TestParsePackReadsViewsAndTurns(t *testing.T) {
	pack, err := ParsePack(writePack(t, viewsYAML))
	if err != nil {
		t.Fatal(err)
	}
	if got := pack.ViewNames(); len(got) != 3 {
		t.Errorf("ViewNames() = %v，期望 3 个", got)
	}
	side, err := pack.View("side")
	if err != nil {
		t.Fatal(err)
	}
	if side.Joints["head"].Pivot != [2]float64{172, 174} {
		t.Errorf("side 视图的 head pivot = %v，期望 [172 174]", side.Joints["head"].Pivot)
	}
	turn, ok := pack.Turns["front->side"]
	if !ok || len(turn.Via) != 1 || turn.Via[0] != "three-quarter" || turn.DurationMs != 200 {
		t.Errorf("turns = %+v", pack.Turns)
	}
}

func TestParsePackRejectsViewProblems(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"视图重名默认视图",
			goodYAML + "views:\n  front:\n    file: x.svg\n    viewBox: [0,0,400,520]\n    baselineY: 512\n    joints:\n      head: { pivot: [1,1], rotate: [-1,1] }\n",
			"front"},
		{"姿势用到的关节在某视图缺失",
			strings.Replace(viewsYAML, "      head:     { pivot: [172, 174], rotate: [-26, 26] }\n", "", 1),
			"side"},
		{"姿势角度超出该视图区间",
			strings.Replace(viewsYAML, "frontLeg: { pivot: [160, 334], rotate: [-45, 60] }", "frontLeg: { pivot: [160, 334], rotate: [-10, 10] }", 1),
			"pointing"},
		{"turns 引用未声明视图",
			viewsYAML + "  front->back: { via: [three-quarter], durationMs: 200 }\n",
			"back"},
		{"turns 的 via 引用未声明视图",
			strings.Replace(viewsYAML, "via: [three-quarter]", "via: [profile]", 1),
			"profile"},
		{"turns 键格式不对",
			viewsYAML + "  frontToSide: { via: [], durationMs: 200 }\n",
			"frontToSide"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePack(writePack(t, tc.body))
			if err == nil {
				t.Fatalf("期望报错含 %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("错误 = %v，期望含 %q", err, tc.want)
			}
		})
	}
}

func TestLoadValidatesEveryViewRig(t *testing.T) {
	dir := writePack(t, viewsYAML)
	// 默认视图与 three-quarter 都合规，side 的 rig 里塞一个自走动画。
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tq := strings.Replace(goodSVG, `id="j-tail"`, `id="j-unused"`, 1)
	write("rig.svg", goodSVG)
	write("rig-tq.svg", tq)
	write("rig-side.svg", strings.Replace(goodSVG, "</svg>", `<animate attributeName="opacity"/></svg>`, 1))

	_, err := Load(dir)
	if err == nil {
		t.Fatal("期望报错")
	}
	if !strings.Contains(err.Error(), "side") || !strings.Contains(err.Error(), "animate") {
		t.Errorf("错误信息 = %v，期望同时点出视图名 side 与 animate", err)
	}
}

func TestViewNamesDedupesDefaultView(t *testing.T) {
	// 手工构造 Pack，绕过 ParsePack 的"views 不得声明 front"校验，模拟测试
	// fixture 或其它调用方直接拼 Pack 的场景。ViewNames() 自己必须保证
	// 不重复，不能指望调用方先过一遍 ParsePack。
	pack := Pack{
		ID:    "heiwa",
		Rig:   Rig{File: "rig.svg"},
		Views: map[string]Rig{DefaultView: {File: "dup.svg"}},
	}
	names := pack.ViewNames()
	if len(names) != 1 || names[0] != DefaultView {
		t.Errorf("ViewNames() = %v，期望去重后只有 [%s]", names, DefaultView)
	}
}
