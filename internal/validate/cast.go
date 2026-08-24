package validate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/chouheiwa/articale-to-motion/internal/cast"
	"github.com/chouheiwa/articale-to-motion/internal/dialogue"
	"github.com/chouheiwa/articale-to-motion/internal/schedule"
	"github.com/chouheiwa/articale-to-motion/internal/srt"
)

// beatCoverageToleranceSeconds 是把镜头本地 beats 换算回全局时间后，与
// dialogue.json 对应行做覆盖比对时的容差。dialogue.json 的时间戳本身就是
// Rebuild 按实测时长缩放算出来的浮点数，1 毫秒足够吸收累加误差，又远小于
// 「beats 真的漏标了一整行」这类实际问题的量级。
const beatCoverageToleranceSeconds = 0.001

// CastProblems 校验多角色项目的项目级一致性：班底与角色包是否自洽、
// dialogue.json 是否完整覆盖字幕、每个镜头的 cast.beats 换算回全局时间后
// 是否合并覆盖了 dialogue.json 的全部台词行。
//
// cast.HasRoster(root) 为假时直接返回 nil：老项目（没有 cast.yaml）不受
// 这组检查影响，这是最重要的一条约束。
//
// 后续检查逐步依赖前面的产物：拿不到角色包就没法查音色，拿不到
// dialogue.json 就没法查字幕覆盖和镜头覆盖，所以某一步失败时会跳过依赖它
// 的后续步骤，但已经收集到的问题仍然全部返回，不是查到第一个就停。
func CastProblems(root, provider string) []string {
	if !cast.HasRoster(root) {
		return nil
	}

	var problems []string

	roster, err := cast.LoadRoster(root)
	if err != nil {
		return append(problems, fmt.Sprintf("读取 %s 失败：%v", cast.RosterFile, err))
	}

	// 早前的裁定：LoadRoster 接受 packs: []（am init 刚建的项目本就还没
	// 角色），"班底为空"要在发布前由这一层拦下，不是 LoadRoster 的职责。
	if len(roster.Packs) == 0 {
		problems = append(problems, fmt.Sprintf("班底为空：%s 没有登记任何角色包，发布前必须至少有一个角色", cast.RosterFile))
	}

	packs, err := roster.LoadPacks(root)
	if err != nil {
		// cast.yaml 列了包但目录被手工删掉、或包本身不自洽：后续检查都依赖
		// packs，拿不到就没法继续。
		return append(problems, fmt.Sprintf("加载角色包失败：%v", err))
	}

	ids := make([]string, 0, len(packs))
	for id := range packs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := packs[id].VoiceFor(provider); err != nil {
			problems = append(problems, fmt.Sprintf("角色 %s 缺少 %s 音色：%v", id, provider, err))
		}
	}

	dialoguePath := filepath.Join(root, "production", "dialogue.json")
	result, dialogueProblems := loadDialogueResult(dialoguePath)
	problems = append(problems, dialogueProblems...)
	if result == nil {
		// 拿不到 dialogue.json 就没法继续查字幕覆盖和镜头覆盖。
		return problems
	}

	problems = append(problems, dialogueLineProblems(result.Lines)...)

	srtPath := filepath.Join(root, "transcription-production.srt")
	if span, err := srt.ReadSpan(srtPath); err != nil {
		problems = append(problems, fmt.Sprintf("读取 transcription-production.srt 失败：%v", err))
	} else if span.Cues != len(result.Lines) {
		problems = append(problems, fmt.Sprintf(
			"dialogue.json 的字幕行数 %d 与 transcription-production.srt 的字幕条数 %d 不一致",
			len(result.Lines), span.Cues))
	}

	for _, line := range result.Lines {
		if _, ok := packs[line.Speaker]; !ok {
			problems = append(problems, fmt.Sprintf("dialogue.json 第 %d 行的说话人 %q 不在班底里", line.SRTIndex, line.Speaker))
		}
	}

	problems = append(problems, sceneCastProblems(root, result.Lines)...)

	return problems
}

// dialogueResultJSON 只解出 CastProblems 需要的字段。dialogue.Result 本身
// 的 Line.Text 带 json:"-"，反序列化用不到；这里复用 dialogue 包的类型而不是
// 自己重新定义一遍字段，避免 schema 出现第二个不同步的真相源。
func loadDialogueResult(path string) (*dialogue.Result, []string) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("找不到或无法读取 production/dialogue.json：%v", err)}
	}
	var result dialogue.Result
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, []string{fmt.Sprintf("production/dialogue.json 不是合法 JSON：%v", err)}
	}
	if result.Schema != dialogue.SchemaVersion {
		return nil, []string{fmt.Sprintf("production/dialogue.json 的 schema 必须是 %s，收到 %q", dialogue.SchemaVersion, result.Schema)}
	}
	if len(result.Lines) == 0 {
		return nil, []string{"production/dialogue.json 没有任何字幕行"}
	}
	return &result, nil
}

// dialogueLineProblems 校验 dialogue.json 的 lines 自身的一致性：srtIndex
// 从 1 连续递增、每行时间不倒挂、逐行之间不重叠。
func dialogueLineProblems(lines []dialogue.Line) []string {
	var problems []string
	prevEnd := math.Inf(-1)
	for i, line := range lines {
		if line.SRTIndex != i+1 {
			problems = append(problems, fmt.Sprintf(
				"dialogue.json 的 srtIndex 未从 1 连续递增：第 %d 行应为 %d，实际 %d", i+1, i+1, line.SRTIndex))
		}
		if line.EndSeconds <= line.StartSeconds {
			problems = append(problems, fmt.Sprintf(
				"dialogue.json 第 %d 行时间倒挂：[%v, %v]", line.SRTIndex, line.StartSeconds, line.EndSeconds))
		}
		if line.StartSeconds < prevEnd-beatCoverageToleranceSeconds {
			problems = append(problems, fmt.Sprintf(
				"dialogue.json 第 %d 行与上一行重叠：上一行止于 %.3f 秒，本行起于 %.3f 秒",
				line.SRTIndex, prevEnd, line.StartSeconds))
		}
		prevEnd = line.EndSeconds
	}
	return problems
}

// globalBeat 是把镜头本地 cast.beats 换算回项目全局时间后的一拍。
type globalBeat struct {
	speaker    string
	start, end float64
}

// sceneCastProblems 遍历 <root>/scenes/*/scene.json，做两件在渲染阶段会
// 静默坏掉、却从来没有任何一层校验过的事：
//
//  1. 把每个镜头的 cast.beats 按镜头在时间轴上的累积起点换算回全局时间，
//     合并后必须覆盖 dialogue.json 的全部 lines，一行都不能漏——拆镜头时
//     漏标一拍，成片里那句台词的角色要么站着不动、要么干脆不在台上，
//     但渲染本身不会报错。
//
//  2. 每个 on_stage 角色声明的 dna.md 是否真的存在于 BuildPrompt 会写进
//     提示词的那个路径（<镜头目录>/<cast.pack_dir>/<id>/dna.md）。这是
//     Task 16 交接下来的缺口 A：internal/cast 和 scene.validateCast 都不
//     查这个文件的存在性，文件缺失时提示词仍然会引用它，渲染 agent
//     读不到人设，只能自己编，且没有任何报错。放在这里检查而不是
//     internal/cast.Load 或 internal/scene.validateCast，是因为只有这里
//     才同时拿得到"镜头目录"与"这个镜头实际使用的 pack_dir"——同一个
//     角色 id 在不同镜头完全可以指向不同的 pack_dir 副本，cast.Load 校验
//     的是项目级班底目录，未必是某个具体镜头引用的那一份。
func sceneCastProblems(root string, lines []dialogue.Line) []string {
	scenesDir := filepath.Join(root, "scenes")
	scenes, err := schedule.Plan(scenesDir)
	if err != nil {
		return []string{fmt.Sprintf("读取镜头目录失败，无法校验 cast.beats 与角色 DNA：%v", err)}
	}

	var problems []string
	var beats []globalBeat
	cursor := 0.0
	for _, s := range scenes {
		if s.Cast != nil {
			for _, beat := range s.Cast.Beats {
				beats = append(beats, globalBeat{
					speaker: beat.Speaker,
					start:   cursor + beat.Start,
					end:     cursor + beat.End,
				})
			}
			for _, actor := range s.Cast.OnStage {
				dnaPath := filepath.Join(s.Directory, s.Cast.PackDir, actor.ID, "dna.md")
				if info, statErr := os.Stat(dnaPath); statErr != nil || !info.Mode().IsRegular() {
					problems = append(problems, fmt.Sprintf(
						"镜头 %s 的角色 %s 缺少 dna.md：%s 不存在，"+
							"渲染提示词会引用这个路径，缺失时渲染 agent 读不到人设、只能自己编",
						s.ID, actor.ID, dnaPath))
				}
			}
		}
		cursor += s.DurationSeconds
	}

	for _, line := range lines {
		if !lineCoveredByBeats(line, beats) {
			problems = append(problems, fmt.Sprintf(
				"dialogue.json 第 %d 行（说话人 %s，%.3f–%.3f 秒）没有被任何镜头的 cast.beats 覆盖",
				line.SRTIndex, line.Speaker, line.StartSeconds, line.EndSeconds))
		}
	}
	return problems
}

// lineCoveredByBeats 判断某一行对白是否被换算到全局时间后的某一拍完整覆盖：
// 同一说话人、且这一拍的 [start,end] 完整包住这一行的 [start,end]。
func lineCoveredByBeats(line dialogue.Line, beats []globalBeat) bool {
	for _, beat := range beats {
		if beat.speaker != line.Speaker {
			continue
		}
		if line.StartSeconds >= beat.start-beatCoverageToleranceSeconds &&
			line.EndSeconds <= beat.end+beatCoverageToleranceSeconds {
			return true
		}
	}
	return false
}
