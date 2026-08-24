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
