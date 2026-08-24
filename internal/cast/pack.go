// Package cast 是角色包（character pack）规范的唯一真相源。
//
// am 不内置任何角色：这个包只定义格式、解析它、并拦住那些会在渲染阶段
// 静默坏掉的写法。角色包由 am cast add 从外部拷进项目。
package cast

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	VoiceID     string   `yaml:"voiceId" json:"voiceId"`
	Speed       *float64 `yaml:"speed" json:"speed"`
	Rate        *float64 `yaml:"rate" json:"rate"`
	Pitch       *float64 `yaml:"pitch" json:"pitch"`
	Volume      *float64 `yaml:"volume" json:"volume"`
	Instruction string   `yaml:"instruction" json:"instruction"`
}

// Joint 是一个可动关节。Pivot 用 viewBox 的 user unit 绝对坐标。
type Joint struct {
	Pivot  [2]float64 `yaml:"pivot" json:"pivot"`
	Rotate [2]float64 `yaml:"rotate" json:"rotate"`
}

type Rig struct {
	File      string           `yaml:"file" json:"file"`
	ViewBox   [4]float64       `yaml:"viewBox" json:"viewBox"`
	BaselineY float64          `yaml:"baselineY" json:"baselineY"`
	Joints    map[string]Joint `yaml:"joints" json:"joints"`
}

type Scale struct {
	HeightRatio [2]float64 `yaml:"heightRatio" json:"heightRatio"`
}

// Pose 是关节名到角度的映射，缺省的关节取 0。
type Pose map[string]float64

// IdlePose 是每个角色包必须提供的基准姿势。
const IdlePose = "idle"

// DefaultView 是 rig 字段所代表的视图名。它不出现在 views 里，
// 但 ViewNames / View 都把它当作一个正常视图对待，调用方无需分两条路径。
const DefaultView = "front"

// Turn 是一次转身：经过哪些中间视图、总时长多少。
//
// 不做自动寻路：哪条路径好看是创作判断，不是图论问题。未声明的组合一律非法。
type Turn struct {
	Via        []string `yaml:"via" json:"via"`
	DurationMs int      `yaml:"durationMs" json:"durationMs"`
}

// turnKeyPattern 约束 turns 的键必须写成 <from>-><to>。
var turnKeyPattern = regexp.MustCompile(`^([a-z][a-z0-9-]*)->([a-z][a-z0-9-]*)$`)

type Pack struct {
	Dir     string           `yaml:"-" json:"-"`
	Schema  string           `yaml:"schema" json:"schema"`
	ID      string           `yaml:"id" json:"id"`
	Name    string           `yaml:"name" json:"name"`
	Summary string           `yaml:"summary" json:"summary"`
	Voice   map[string]Voice `yaml:"voice" json:"voice"`
	Rig     Rig              `yaml:"rig" json:"rig"`
	Views   map[string]Rig   `yaml:"views" json:"views"`
	Turns   map[string]Turn  `yaml:"turns" json:"turns"`
	Scale   Scale            `yaml:"scale" json:"scale"`
	Poses   map[string]Pose  `yaml:"poses" json:"poses"`
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
	if err := validateRigFields(pack.ID, DefaultView, pack.Rig); err != nil {
		return Pack{}, err
	}
	if _, ok := pack.Views[DefaultView]; ok {
		return Pack{}, fmt.Errorf("角色包 %s 的 views 不能再声明 %s，它已经是 rig 字段代表的默认视图", pack.ID, DefaultView)
	}
	for name, view := range pack.Views {
		if err := validateRigFields(pack.ID, name, view); err != nil {
			return Pack{}, err
		}
	}
	lo, hi := pack.Scale.HeightRatio[0], pack.Scale.HeightRatio[1]
	if lo <= 0 || hi <= lo || hi >= 1 {
		return Pack{}, fmt.Errorf("角色包 %s 的 heightRatio 必须满足 0 < lo < hi < 1，收到 %v", pack.ID, pack.Scale.HeightRatio)
	}
	if _, ok := pack.Poses[IdlePose]; !ok {
		return Pack{}, fmt.Errorf("角色包 %s 必须提供 %s 姿势作为基准", pack.ID, IdlePose)
	}
	viewNames := pack.ViewNames()
	for poseName, pose := range pack.Poses {
		for jointName, angle := range pose {
			joint, ok := pack.Rig.Joints[jointName]
			if !ok {
				return Pack{}, fmt.Errorf("角色包 %s 姿势 %s 引用了未声明的关节 %s", pack.ID, poseName, jointName)
			}
			if angle < joint.Rotate[0] || angle > joint.Rotate[1] {
				return Pack{}, fmt.Errorf("角色包 %s 姿势 %s 的关节 %s 角度 %v 在视图 %s 里超出区间 %v",
					pack.ID, poseName, jointName, angle, DefaultView, joint.Rotate)
			}
			// 姿势跨视图共享，但关节是每视图独立声明的：同一个姿势换个视图，
			// 用到的关节必须在那个视图里也存在，且角度落在那个视图自己的
			// rotate 区间内。漏了这条检查，角色一转身某个姿势就会静默失效。
			for _, viewName := range viewNames {
				if viewName == DefaultView {
					continue
				}
				view := pack.Views[viewName]
				viewJoint, ok := view.Joints[jointName]
				if !ok {
					return Pack{}, fmt.Errorf("角色包 %s 姿势 %s 的关节 %s 在视图 %s 里没有声明",
						pack.ID, poseName, jointName, viewName)
				}
				if angle < viewJoint.Rotate[0] || angle > viewJoint.Rotate[1] {
					return Pack{}, fmt.Errorf("角色包 %s 姿势 %s 的关节 %s 角度 %v 在视图 %s 里超出区间 %v",
						pack.ID, poseName, jointName, angle, viewName, viewJoint.Rotate)
				}
			}
		}
	}
	if err := validateTurns(pack.ID, pack.Turns, viewNames); err != nil {
		return Pack{}, err
	}
	return pack, nil
}

// validateRigFields 校验一个 rig（不论是默认视图的 rig 字段，还是 views 里的
// 某个视图）本身的字段是否完整合法。viewName 只用来让错误信息点出问题出在
// 哪个视图，不参与校验逻辑。
func validateRigFields(id, viewName string, rig Rig) error {
	if rig.File == "" {
		return fmt.Errorf("角色包 %s 的视图 %s 缺少 file", id, viewName)
	}
	if len(rig.Joints) == 0 {
		return fmt.Errorf("角色包 %s 的视图 %s 没有声明任何关节", id, viewName)
	}
	for name, joint := range rig.Joints {
		if joint.Rotate[0] >= joint.Rotate[1] {
			return fmt.Errorf("角色包 %s 的视图 %s 关节 %s 的 rotate 区间无效：%v", id, viewName, name, joint.Rotate)
		}
	}
	if rig.BaselineY <= rig.ViewBox[1] || rig.BaselineY > rig.ViewBox[1]+rig.ViewBox[3] {
		return fmt.Errorf("角色包 %s 的视图 %s 的 baselineY=%v 落在 viewBox 之外", id, viewName, rig.BaselineY)
	}
	return nil
}

// validateTurns 校验 turns 声明：键必须匹配 <from>-><to>，from/to/via 引用的
// 视图都必须已经声明过，durationMs 必须为正。
func validateTurns(id string, turns map[string]Turn, viewNames []string) error {
	known := make(map[string]bool, len(viewNames))
	for _, name := range viewNames {
		known[name] = true
	}
	for key, turn := range turns {
		m := turnKeyPattern.FindStringSubmatch(key)
		if m == nil {
			return fmt.Errorf("角色包 %s 的 turns 键 %q 格式不对，必须是 <from>-><to>", id, key)
		}
		from, to := m[1], m[2]
		if !known[from] {
			return fmt.Errorf("角色包 %s 的 turns %q 引用了未声明的视图 %s", id, key, from)
		}
		if !known[to] {
			return fmt.Errorf("角色包 %s 的 turns %q 引用了未声明的视图 %s", id, key, to)
		}
		for _, via := range turn.Via {
			if !known[via] {
				return fmt.Errorf("角色包 %s 的 turns %q 的 via 引用了未声明的视图 %s", id, key, via)
			}
		}
		if turn.DurationMs <= 0 {
			return fmt.Errorf("角色包 %s 的 turns %q 的 durationMs 必须大于 0，收到 %d", id, key, turn.DurationMs)
		}
	}
	return nil
}

// View 按名字返回视图，DefaultView 回落到 rig 字段。
func (p Pack) View(name string) (Rig, error) {
	if name == "" || name == DefaultView {
		return p.Rig, nil
	}
	view, ok := p.Views[name]
	if !ok {
		return Rig{}, fmt.Errorf("角色 %s 没有视图 %s", p.ID, name)
	}
	return view, nil
}

// ViewNames 返回全部视图名（含默认视图），去重后排序，便于确定性遍历。
//
// 去重是这个方法契约的一部分，不外包给调用方保证：ParsePack 会挡掉 views
// 里出现 DefaultView 的写法，但调用方也可能绕过 ParsePack 直接手工构造
// Pack（测试 fixture 就很常见），这时 Views 仍可能意外带上 DefaultView。
// 不去重的话，下游按 ViewNames() 逐视图渲染或校验时会悄悄把同一个视图
// 处理两遍，且不报错。
func (p Pack) ViewNames() []string {
	seen := make(map[string]bool, len(p.Views)+1)
	names := make([]string, 0, len(p.Views)+1)
	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	add(DefaultView)
	for name := range p.Views {
		add(name)
	}
	sort.Strings(names)
	return names
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

// Load 解析角色包并校验每一个视图的 rig，是外部唯一该用的入口。
//
// 逐视图校验、逐视图报错：三个视图的 rig 各自独立成图，一个视图混进自走
// 动画不代表另外两个也有问题。把问题按视图分组聚合进同一个错误里一次报出
// 全部，而不是每个视图各自的错误信息里不点名是哪个视图——不然三个视图都
// 报"rig 有问题"，人不知道该改哪个文件。
func Load(dir string) (Pack, error) {
	pack, err := ParsePack(dir)
	if err != nil {
		return Pack{}, err
	}
	var groups []string
	for _, viewName := range pack.ViewNames() {
		rig, err := pack.View(viewName)
		if err != nil {
			return Pack{}, err
		}
		problems := ValidateRig(filepath.Join(dir, rig.File), rig)
		if len(problems) == 0 {
			continue
		}
		groups = append(groups, fmt.Sprintf("视图 %s（%s）有 %d 处问题：\n      - %s",
			viewName, rig.File, len(problems), strings.Join(problems, "\n      - ")))
	}
	if len(groups) > 0 {
		return Pack{}, fmt.Errorf("角色包 %s 的 rig 校验未通过：\n  - %s",
			pack.ID, strings.Join(groups, "\n  - "))
	}
	return pack, nil
}

// WriteJSON 把角色包序列化成 character.json，写进 dir。
//
// 驱动库 cast.js 要在浏览器里读角色包：渲染机是干净的无头 Chrome，CSP 下引不进
// YAML 解析库，手写 YAML 子集解析器纯属埋雷。character.yaml 仍是人工编辑的
// 唯一真相源，character.json 是每次校验（am cast new/add/validate）都重新
// 生成的产物，用 JSON.parse 读。
//
// 直接 json.MarshalIndent 序列化 Pack 本身，不手工拼 JSON：以后 Pack 加字段，
// JSON 自动跟上，不会有人忘记同步两处。Dir 字段带 json:"-"，它是本机绝对
// 路径，不该进产物。
func (p Pack) WriteJSON(dir string) error {
	body, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化角色包 %s 失败：%w", p.ID, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "character.json"), body, 0o644); err != nil {
		return fmt.Errorf("写出 character.json 失败：%w", err)
	}
	return nil
}
