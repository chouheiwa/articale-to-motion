package cast

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func loadedPack(t *testing.T) Pack {
	t.Helper()
	dir := writePack(t, goodYAML)
	if err := os.WriteFile(filepath.Join(dir, "rig.svg"), []byte(goodSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	pack, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return pack
}

func TestPoseSVGAppliesRotationAtPivot(t *testing.T) {
	body, err := PoseSVG(loadedPack(t), "pointing")
	if err != nil {
		t.Fatal(err)
	}
	// pointing: frontLeg 48 度、head -6 度，pivot 来自 yaml。
	if !strings.Contains(body, `rotate(48 176 330)`) {
		t.Errorf("缺 frontLeg 的旋转，实际：%s", body)
	}
	if !strings.Contains(body, `rotate(-6 200 168)`) {
		t.Errorf("缺 head 的旋转，实际：%s", body)
	}
}

func TestPoseSVGIdleHasNoRotation(t *testing.T) {
	body, err := PoseSVG(loadedPack(t), "idle")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "rotate(") {
		t.Errorf("idle 应无旋转，实际：%s", body)
	}
}

func TestPoseSVGUnknownPose(t *testing.T) {
	if _, err := PoseSVG(loadedPack(t), "flying"); err == nil || !strings.Contains(err.Error(), "flying") {
		t.Fatalf("期望报未知姿势，得到 %v", err)
	}
}

// 姿势数超过一行 4 张时，如果 ContactSheet 的 tile 还写死 "4x1"，
// ImageMagick 的 montage 会静默产出 out-0.png、out-1.png，Preview 返回的
// 那个路径根本不存在。这里用 6 个姿势覆盖这条回归线。
func TestPreviewProducesContactSheetForManyPoses(t *testing.T) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Skip("需要 rsvg-convert，本机未安装")
	}
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skip("需要 ImageMagick 的 magick 命令，本机未安装")
	}

	manyPosesYAML := strings.Replace(goodYAML,
		"poses:\n  idle: {}\n  pointing: { frontLeg: 48, head: -6 }\n",
		`poses:
  idle: {}
  pose1: { head: 5 }
  pose2: { head: -5 }
  pose3: { frontLeg: 10 }
  pose4: { frontLeg: -10 }
  pose5: { head: 3, frontLeg: 3 }
`, 1)
	dir := writePack(t, manyPosesYAML)
	if err := os.WriteFile(filepath.Join(dir, "rig.svg"), []byte(goodSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	pack, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pack.Poses) <= 4 {
		t.Fatalf("测试夹具姿势数应超过一行 4 张，实际 %d", len(pack.Poses))
	}

	sheet, err := Preview(pack, t.TempDir())
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if _, err := os.Stat(sheet); err != nil {
		t.Fatalf("contact sheet 应存在于 %s：%v", sheet, err)
	}
}
