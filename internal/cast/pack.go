// Package cast 是角色包（character pack）规范的唯一真相源。
//
// am 不内置任何角色：这个包只定义格式、解析它、并拦住那些会在渲染阶段
// 静默坏掉的写法。角色包由 am cast add 从外部拷进项目。
package cast

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SchemaVersion 是角色包与项目班底共用的 schema 标识。
const SchemaVersion = "cast/v1"

// PackFile 是角色包的清单文件名。
const PackFile = "character.yaml"

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Voice 是一个 provider 下的音色声明。
//
// 不做跨 provider 映射：MiniMax 的 speed/pitch 与百炼的 rate/instruction
// 参数名和取值域都不同，硬映射必然错。各家的字段各自可空。
type Voice struct {
	VoiceID     string   `yaml:"voiceId"`
	Speed       *float64 `yaml:"speed"`
	Rate        *float64 `yaml:"rate"`
	Pitch       *float64 `yaml:"pitch"`
	Volume      *float64 `yaml:"volume"`
	Instruction string   `yaml:"instruction"`
}

// Joint 是一个可动关节。Pivot 用 viewBox 的 user unit 绝对坐标。
type Joint struct {
	Pivot  [2]float64 `yaml:"pivot"`
	Rotate [2]float64 `yaml:"rotate"`
}

type Rig struct {
	File      string           `yaml:"file"`
	ViewBox   [4]float64       `yaml:"viewBox"`
	BaselineY float64          `yaml:"baselineY"`
	Joints    map[string]Joint `yaml:"joints"`
}

type Scale struct {
	HeightRatio [2]float64 `yaml:"heightRatio"`
}

// Pose 是关节名到角度的映射，缺省的关节取 0。
type Pose map[string]float64

// IdlePose 是每个角色包必须提供的基准姿势。
const IdlePose = "idle"

type Pack struct {
	Dir     string           `yaml:"-"`
	Schema  string           `yaml:"schema"`
	ID      string           `yaml:"id"`
	Name    string           `yaml:"name"`
	Summary string           `yaml:"summary"`
	Voice   map[string]Voice `yaml:"voice"`
	Rig     Rig              `yaml:"rig"`
	Scale   Scale            `yaml:"scale"`
	Poses   map[string]Pose  `yaml:"poses"`
}

// ParsePack 只解析并校验 character.yaml 本身，不碰 rig.svg。
// 完整校验走 Load。
func ParsePack(dir string) (Pack, error) {
	body, err := os.ReadFile(filepath.Join(dir, PackFile))
	if err != nil {
		return Pack{}, fmt.Errorf("读取角色包清单失败：%w", err)
	}
	var pack Pack
	if err := yaml.Unmarshal(body, &pack); err != nil {
		return Pack{}, fmt.Errorf("解析 %s 失败：%w", PackFile, err)
	}
	pack.Dir = dir
	if pack.Schema != SchemaVersion {
		return Pack{}, fmt.Errorf("角色包 schema 必须是 %s，收到 %q", SchemaVersion, pack.Schema)
	}
	if !idPattern.MatchString(pack.ID) {
		return Pack{}, fmt.Errorf("角色包 id 必须匹配 %s，收到 %q", idPattern, pack.ID)
	}
	if pack.Name == "" {
		return Pack{}, fmt.Errorf("角色包 %s 缺少 name", pack.ID)
	}
	if len(pack.Voice) == 0 {
		return Pack{}, fmt.Errorf("角色包 %s 没有声明任何音色", pack.ID)
	}
	if pack.Rig.File == "" {
		return Pack{}, fmt.Errorf("角色包 %s 缺少 rig.file", pack.ID)
	}
	if len(pack.Rig.Joints) == 0 {
		return Pack{}, fmt.Errorf("角色包 %s 没有声明任何关节", pack.ID)
	}
	for name, joint := range pack.Rig.Joints {
		if joint.Rotate[0] >= joint.Rotate[1] {
			return Pack{}, fmt.Errorf("角色包 %s 关节 %s 的 rotate 区间无效：%v", pack.ID, name, joint.Rotate)
		}
	}
	if pack.Rig.BaselineY <= pack.Rig.ViewBox[1] || pack.Rig.BaselineY > pack.Rig.ViewBox[1]+pack.Rig.ViewBox[3] {
		return Pack{}, fmt.Errorf("角色包 %s 的 baselineY=%v 落在 viewBox 之外", pack.ID, pack.Rig.BaselineY)
	}
	lo, hi := pack.Scale.HeightRatio[0], pack.Scale.HeightRatio[1]
	if lo <= 0 || hi <= lo || hi >= 1 {
		return Pack{}, fmt.Errorf("角色包 %s 的 heightRatio 必须满足 0 < lo < hi < 1，收到 %v", pack.ID, pack.Scale.HeightRatio)
	}
	if _, ok := pack.Poses[IdlePose]; !ok {
		return Pack{}, fmt.Errorf("角色包 %s 必须提供 %s 姿势作为基准", pack.ID, IdlePose)
	}
	for poseName, pose := range pack.Poses {
		for jointName, angle := range pose {
			joint, ok := pack.Rig.Joints[jointName]
			if !ok {
				return Pack{}, fmt.Errorf("角色包 %s 姿势 %s 引用了未声明的关节 %s", pack.ID, poseName, jointName)
			}
			if angle < joint.Rotate[0] || angle > joint.Rotate[1] {
				return Pack{}, fmt.Errorf("角色包 %s 姿势 %s 的关节 %s 角度 %v 超出区间 %v",
					pack.ID, poseName, jointName, angle, joint.Rotate)
			}
		}
	}
	return pack, nil
}

// VoiceFor 返回指定 provider 的音色声明。
func (p Pack) VoiceFor(provider string) (Voice, error) {
	voice, ok := p.Voice[provider]
	if !ok {
		return Voice{}, fmt.Errorf("角色 %s 没有声明 %s 的音色", p.ID, provider)
	}
	if voice.VoiceID == "" {
		return Voice{}, fmt.Errorf("角色 %s 的 %s 音色缺少 voiceId", p.ID, provider)
	}
	return voice, nil
}

// Load 解析角色包并校验 rig，是外部唯一该用的入口。
func Load(dir string) (Pack, error) {
	pack, err := ParsePack(dir)
	if err != nil {
		return Pack{}, err
	}
	problems := ValidateRig(filepath.Join(dir, pack.Rig.File), pack.Rig)
	if len(problems) > 0 {
		return Pack{}, fmt.Errorf("角色包 %s 的 %s 有 %d 处问题：\n  - %s",
			pack.ID, pack.Rig.File, len(problems), strings.Join(problems, "\n  - "))
	}
	return pack, nil
}
