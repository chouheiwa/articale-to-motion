package cast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRosterProject(t *testing.T, roster string, packIDs ...string) string {
	t.Helper()
	root := t.TempDir()
	if roster != "" {
		if err := os.WriteFile(filepath.Join(root, RosterFile), []byte(roster), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range packIDs {
		dir := filepath.Join(root, "cast", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := strings.Replace(goodYAML, "id: heiwa", "id: "+id, 1)
		if err := os.WriteFile(filepath.Join(dir, PackFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "rig.svg"), []byte(goodSVG), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const goodRoster = `schema: cast/v1
packs: [cast/heiwa, cast/zhaocai]
defaults:
  ground_y: 0.78
  gap_ms: { turn: 240, interject: 100 }
`

func TestHasRoster(t *testing.T) {
	if HasRoster(writeRosterProject(t, "")) {
		t.Error("没有 cast.yaml 时应判为单口播模式")
	}
	if !HasRoster(writeRosterProject(t, goodRoster, "heiwa", "zhaocai")) {
		t.Error("有 cast.yaml 时应判为多角色模式")
	}
}

func TestLoadPacksKeyedByID(t *testing.T) {
	root := writeRosterProject(t, goodRoster, "heiwa", "zhaocai")
	roster, err := LoadRoster(root)
	if err != nil {
		t.Fatal(err)
	}
	if roster.Defaults.GroundY != 0.78 || roster.Defaults.GapMs.Interject != 100 {
		t.Errorf("defaults = %+v", roster.Defaults)
	}
	packs, err := roster.LoadPacks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) != 2 || packs["zhaocai"].ID != "zhaocai" {
		t.Errorf("packs = %+v", packs)
	}
}

func TestLoadPacksReportsMissingDir(t *testing.T) {
	root := writeRosterProject(t, goodRoster, "heiwa")
	roster, err := LoadRoster(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roster.LoadPacks(root); err == nil || !strings.Contains(err.Error(), "zhaocai") {
		t.Fatalf("期望报 zhaocai 缺失，得到 %v", err)
	}
}

func TestLoadPacksRejectsPathEscapeWithDotDot(t *testing.T) {
	// 测试 packs: [../outside] 这样的相对路径逃逸被拒
	root := writeRosterProject(t, "", "heiwa")
	escapeRoster := `schema: cast/v1
packs: [../outside]
defaults:
  ground_y: 0.78
  gap_ms: { turn: 240, interject: 100 }
`
	if err := os.WriteFile(filepath.Join(root, RosterFile), []byte(escapeRoster), 0o644); err != nil {
		t.Fatal(err)
	}
	roster, err := LoadRoster(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roster.LoadPacks(root); err == nil || !strings.Contains(err.Error(), "不得逃出") {
		t.Fatalf("期望报路径逃逸，得到 %v", err)
	}
}

func TestLoadPacksRejectsSymlinkOutsideProject(t *testing.T) {
	// 测试指向项目外目录的软链被拒
	root := writeRosterProject(t, "", "heiwa")

	// 创建项目外的临时目录
	outside := t.TempDir()
	dir := filepath.Join(outside, "outside-pack")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(goodYAML, "id: heiwa", "id: outside", 1)
	if err := os.WriteFile(filepath.Join(dir, PackFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rig.svg"), []byte(goodSVG), 0o644); err != nil {
		t.Fatal(err)
	}

	// 创建软链指向项目外的目录
	castDir := filepath.Join(root, "cast")
	if err := os.MkdirAll(castDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(castDir, "outside")
	if err := os.Symlink(dir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	symlinkRoster := `schema: cast/v1
packs: [cast/outside]
defaults:
  ground_y: 0.78
  gap_ms: { turn: 240, interject: 100 }
`
	if err := os.WriteFile(filepath.Join(root, RosterFile), []byte(symlinkRoster), 0o644); err != nil {
		t.Fatal(err)
	}

	roster, err := LoadRoster(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roster.LoadPacks(root); err == nil || !strings.Contains(err.Error(), "不得逃出") {
		t.Fatalf("期望报软链逃逸，得到 %v", err)
	}
}

func TestLoadPacksAcceptsNormalPath(t *testing.T) {
	// 测试正常的 cast/heiwa 仍然通过
	root := writeRosterProject(t, goodRoster, "heiwa", "zhaocai")
	roster, err := LoadRoster(root)
	if err != nil {
		t.Fatal(err)
	}
	packs, err := roster.LoadPacks(root)
	if err != nil {
		t.Fatalf("正常路径应该加载成功，得到 %v", err)
	}
	if len(packs) != 2 || packs["heiwa"].ID != "heiwa" {
		t.Errorf("期望加载成功，得到 %+v", packs)
	}
}

func TestLoadRosterAllowsEmptyPacks(t *testing.T) {
	// 测试允许空 packs 列表。
	// 理由：am init --narration cast 刚建的项目还没引入任何角色包，
	// 需要通过后续 am cast add 来添加。"班底为空"的检查由后续的
	// internal/validate 项目级校验负责，那才是发布前应该拦截的地方。
	root := writeRosterProject(t, "")
	emptyRoster := `schema: cast/v1
packs: []
defaults:
  ground_y: 0.78
  gap_ms: { turn: 240, interject: 100 }
`
	if err := os.WriteFile(filepath.Join(root, RosterFile), []byte(emptyRoster), 0o644); err != nil {
		t.Fatal(err)
	}
	roster, err := LoadRoster(root)
	if err != nil {
		t.Fatalf("空 packs 应该加载成功，得到 %v", err)
	}
	if len(roster.Packs) != 0 {
		t.Errorf("期望 packs 为空，得到 %+v", roster.Packs)
	}
	packs, err := roster.LoadPacks(root)
	if err != nil {
		t.Fatalf("空 packs 的 LoadPacks 应该返回空 map，得到 %v", err)
	}
	if len(packs) != 0 {
		t.Errorf("期望 packs map 为空，得到 %+v", packs)
	}
}

func TestHasRosterRejectsDirectory(t *testing.T) {
	// 测试 cast.yaml 是目录时被判为不存在
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, RosterFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if HasRoster(root) {
		t.Error("cast.yaml 是目录时应判为不存在多角色模式")
	}
}
