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
