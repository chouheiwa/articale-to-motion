package cast

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RosterFile 是项目级班底文件名。它存在与否就是叙事模式本身：
// 不引入运行期开关，避免"配置说 cast、项目里没有角色包"这种可矛盾状态。
const RosterFile = "cast.yaml"

// GapMs 是换人停顿，单位毫秒。
// turn 是常规轮换，interject 是打断式插话——后者压短才有抢话感。
type GapMs struct {
	Turn      int `yaml:"turn"`
	Interject int `yaml:"interject"`
}

type Defaults struct {
	GroundY float64 `yaml:"ground_y"`
	GapMs   GapMs   `yaml:"gap_ms"`
}

type Roster struct {
	Schema   string   `yaml:"schema"`
	Packs    []string `yaml:"packs"`
	Defaults Defaults `yaml:"defaults"`
}

// HasRoster 判断项目是否为多角色模式。
func HasRoster(root string) bool {
	info, err := os.Stat(filepath.Join(root, RosterFile))
	return err == nil && !info.IsDir()
}

func LoadRoster(root string) (Roster, error) {
	body, err := os.ReadFile(filepath.Join(root, RosterFile))
	if err != nil {
		return Roster{}, fmt.Errorf("读取 %s 失败：%w", RosterFile, err)
	}
	var roster Roster
	if err := yaml.Unmarshal(body, &roster); err != nil {
		return Roster{}, fmt.Errorf("解析 %s 失败：%w", RosterFile, err)
	}
	if roster.Schema != SchemaVersion {
		return Roster{}, fmt.Errorf("%s 的 schema 必须是 %s，收到 %q", RosterFile, SchemaVersion, roster.Schema)
	}
	if roster.Defaults.GroundY <= 0 || roster.Defaults.GroundY >= 1 {
		return Roster{}, fmt.Errorf("%s 的 defaults.ground_y 必须在 (0,1)，收到 %v", RosterFile, roster.Defaults.GroundY)
	}
	if roster.Defaults.GapMs.Turn < 0 || roster.Defaults.GapMs.Interject < 0 {
		return Roster{}, fmt.Errorf("%s 的 defaults.gap_ms 不得为负：%+v", RosterFile, roster.Defaults.GapMs)
	}
	return roster, nil
}

// containedPack 检查 packs 路径是否逃出项目根目录。
// 处理 `../../outside` 这类相对路径逃逸，以及指向项目外的软链。
func containedPack(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%s 的 packs 必须是非空相对路径，收到 %q", RosterFile, rel)
	}
	root, _ = filepath.Abs(root)
	target, _ := filepath.Abs(filepath.Join(root, rel))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("无法解析项目目录：%w", err)
	}
	resolvedTarget, resolveErr := filepath.EvalSymlinks(target)
	if resolveErr != nil {
		// 目标不存在（可能还没有被创建），尝试解析其父目录
		resolvedParent, parentErr := filepath.EvalSymlinks(filepath.Dir(target))
		if parentErr == nil {
			resolvedTarget = filepath.Join(resolvedParent, filepath.Base(target))
		} else {
			resolvedTarget = target
		}
	}
	rel, err = filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s 的 packs 不得逃出项目目录：%s", RosterFile, rel)
	}
	return resolvedTarget, nil
}

// LoadPacks 按班底加载全部角色包，键是角色 id。
func (r Roster) LoadPacks(root string) (map[string]Pack, error) {
	out := make(map[string]Pack, len(r.Packs))
	for _, rel := range r.Packs {
		// 检查路径安全性，防止逃逸和软链指向项目外
		if _, err := containedPack(root, rel); err != nil {
			return nil, err
		}
		pack, err := Load(filepath.Join(root, rel))
		if err != nil {
			return nil, fmt.Errorf("角色包 %s：%w", rel, err)
		}
		if existing, ok := out[pack.ID]; ok {
			return nil, fmt.Errorf("角色 id %s 重复：%s 与 %s", pack.ID, existing.Dir, pack.Dir)
		}
		out[pack.ID] = pack
	}
	return out, nil
}
