package castbeats

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/cast"
	"github.com/chouheiwa/articale-to-motion/internal/dialogue"
	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
	"github.com/chouheiwa/articale-to-motion/internal/scene"
)

// DialogueRelPath 是对白时间线在项目里的固定位置，与 am dialogue assemble
// 的产出位置一致。是 dialogue.DialogueRelPath 的别名，不要另造第三处：
// 写方（dialogue.Assemble）、打印方（internal/cli 的 runDialogueAssemble）、
// 读方（internal/validate/cast.go、本包）都只应引用这一份真相源。
var DialogueRelPath = dialogue.DialogueRelPath

// SceneBeats 是单个镜头的重算结果。
type SceneBeats struct {
	ID      string
	Count   int
	Changed bool
}

// Report 是一次重算的汇总，供 CLI 打印。
type Report struct {
	Lines   int
	Scenes  []SceneBeats
	Updated int
}

// Apply 读 root 下的 production/dialogue.json 与 scenes/*/scene.json，
// 把每一行台词按镜头切片、减去镜头起点，写回各镜头 cast.beats。
//
// 只动 cast.beats：pack_dir / ground_y / on_stage 以及 cast 块之外的一切
// 字段都原样保留，键的顺序也不变。没有 cast 块的镜头一律不碰——本命令不会
// 替作者决定角色站位，有台词盖过却没写 cast 块时报错而不是现编一个。
//
// 全部检查通过之前一个字节都不写：报错时不留下改了一半的镜头树。
func Apply(root string) (Report, error) {
	// cast.yaml 存在与否就是叙事模式本身。单口播项目误跑到这条命令时，
	// 先说清"本项目不是多角色项目"——否则报出来的是"找不到 dialogue.json"，
	// 会把人往"少跑了 am dialogue assemble"上引，接着去装配一个根本不存在
	// 的对白。
	if !cast.HasRoster(root) {
		return Report{}, fmt.Errorf(
			"本项目不是多角色项目：项目根没有 %s，cast.beats 只在多角色模式下存在。"+
				"单口播项目不需要执行本命令；如果这本该是多角色项目，"+
				"请确认当前目录就是项目根，并用 am init --narration cast 初始化",
			cast.RosterFile)
	}

	lines, err := loadDialogueLines(filepath.Join(root, DialogueRelPath))
	if err != nil {
		return Report{}, err
	}

	heads, err := readSceneHeads(filepath.Join(root, "scenes"))
	if err != nil {
		return Report{}, err
	}

	offsets := make([]float64, len(heads))
	durations := make([]float64, len(heads))
	total := 0.0
	for i, h := range heads {
		offsets[i] = total
		durations[i] = h.duration
		total += h.duration
	}

	perScene, uncovered := slice(lines, offsets, durations)

	var problems []string
	for i, h := range heads {
		if !h.hasCast && len(perScene[i]) > 0 {
			problems = append(problems, fmt.Sprintf(
				"镜头 %s（全局 %.3f–%.3f 秒）有台词盖过，却没有 cast 块：本命令只负责重算 beats，"+
					"不会替你决定 pack_dir / ground_y / on_stage——先把 cast 块补上（beats 可以不写）再重跑",
				h.id, offsets[i], offsets[i]+durations[i]))
		}
	}
	for _, index := range uncovered {
		l := lines[index]
		problems = append(problems, fmt.Sprintf(
			"production/dialogue.json 第 %d 行（说话人 %s，%.3f–%.3f 秒）没有被任何镜头盖住，"+
				"全部镜头合计只有 %.3f 秒：这是镜头 duration_seconds 与配音对不上，"+
				"先对齐镜头时长再重算节拍",
			l.SRTIndex, l.Speaker, l.StartSeconds, l.EndSeconds, total))
	}
	if len(problems) > 0 {
		return Report{}, fmt.Errorf("无法重算镜头节拍：\n  - %s", strings.Join(problems, "\n  - "))
	}

	report := Report{Lines: len(lines)}
	for i, h := range heads {
		if !h.hasCast {
			continue
		}
		changed, err := writeSceneBeats(filepath.Join(h.dir, "scene.json"), perScene[i])
		if err != nil {
			return Report{}, fmt.Errorf("写入镜头 %s 的节拍失败：%w", h.id, err)
		}
		report.Scenes = append(report.Scenes, SceneBeats{ID: h.id, Count: len(perScene[i]), Changed: changed})
		if changed {
			report.Updated++
		}
	}
	return report, nil
}

// loadDialogueLines 读出 dialogue.json 的台词行并校验它自身可用：schema
// 对得上、至少一行、逐行按时间递增不重叠。
//
// 重叠必须在这里拦：重叠的输入切出来的就是重叠的节拍，写进 scene.json 之后
// scene.Load 会直接拒绝加载——那时错误指向的是本命令刚写坏的镜头，而不是
// 真正有问题的 dialogue.json。
func loadDialogueLines(path string) ([]dialogue.Line, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("找不到或无法读取 production/dialogue.json：%w"+
			"（多角色项目的对白时间线由 am dialogue assemble 产出，请先执行它）", err)
	}
	var result dialogue.Result
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("production/dialogue.json 不是合法 JSON：%w", err)
	}
	if result.Schema != dialogue.SchemaVersion {
		return nil, fmt.Errorf("production/dialogue.json 的 schema 必须是 %s，收到 %q", dialogue.SchemaVersion, result.Schema)
	}
	if len(result.Lines) == 0 {
		return nil, fmt.Errorf("production/dialogue.json 没有任何台词行")
	}
	for i, l := range result.Lines {
		if l.EndSeconds <= l.StartSeconds {
			return nil, fmt.Errorf("production/dialogue.json 第 %d 行时间倒挂：[%v, %v]", l.SRTIndex, l.StartSeconds, l.EndSeconds)
		}
		if i > 0 && l.StartSeconds < result.Lines[i-1].EndSeconds-minBeatSeconds {
			return nil, fmt.Errorf("production/dialogue.json 第 %d 行与上一行重叠：上一行止于 %.3f 秒，本行起于 %.3f 秒",
				l.SRTIndex, result.Lines[i-1].EndSeconds, l.StartSeconds)
		}
	}
	return result.Lines, nil
}

// writeSceneBeats 只替换 scene.json 里 cast.beats 一个字段，其余字节原样
// 保留（键顺序、数字写法、字符串转义都不变），整份文件重新按两空格缩进。
//
// 内容没变时不落盘：重复执行本命令不该刷新任何文件的 mtime，否则下游
// 那些拿 mtime 判"产物是不是过期"的逻辑会被无谓地触发。
func writeSceneBeats(path string, beats []scene.Beat) (bool, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	fields, err := decodeObject(original)
	if err != nil {
		return false, err
	}
	castRaw, ok := findField(fields, "cast")
	if !ok {
		return false, fmt.Errorf("scene.json 没有 cast 块")
	}
	castFields, err := decodeObject(castRaw)
	if err != nil {
		return false, fmt.Errorf("scene.json 的 cast 块解析失败：%w", err)
	}
	if beats == nil {
		beats = []scene.Beat{}
	}
	beatsRaw, err := json.Marshal(beats)
	if err != nil {
		return false, err
	}
	castFields = setField(castFields, "beats", beatsRaw)
	castEncoded, err := encodeObject(castFields)
	if err != nil {
		return false, err
	}
	fields = setField(fields, "cast", castEncoded)
	encoded, err := encodeObject(fields)
	if err != nil {
		return false, err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, encoded, "", "  "); err != nil {
		return false, err
	}
	pretty.WriteByte('\n')
	if bytes.Equal(pretty.Bytes(), original) {
		return false, nil
	}
	return true, fsutil.AtomicWrite(path, pretty.Bytes(), 0o644)
}
