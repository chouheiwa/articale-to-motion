package scene

import (
	"fmt"
	"os"
	"path/filepath"
)

// 渲染工具各自把技能装在不同目录，提示词里不得写死任何一条路径。
// 这里在 Go 侧解析出本机真实位置，解析不到时由 BuildPrompt 退回按技能名引用。
const (
	// SkillsDirEnv 是逃生阀：显式指定技能目录，优先级高于一切自动探测。
	SkillsDirEnv = "HYPERFRAMES_SKILLS_DIR"
	// AnimationSkillName 是承载全部动效知识的技能目录名。
	AnimationSkillName = "hyperframes-animation"

	rulesIndexFile      = "rules-index.md"
	blueprintsIndexFile = "blueprints-index.md"
	skillManifestFile   = "SKILL.md"

	// portableSkillsDir 是不绑定具体工具的通用约定，作为兜底候选。
	portableSkillsDir = ".agents/skills"

	// maxWalkUpLevels 限制向上查找项目级技能目录的层数，避免病态路径下空转。
	maxWalkUpLevels = 40
)

// skillLocations 是一个渲染工具的技能目录：家目录级与项目级的相对路径可能不同。
type skillLocations struct {
	home    string
	project string
}

var rendererSkillDirs = map[string]skillLocations{
	"claude":    {home: ".claude/skills", project: ".claude/skills"},
	"codex":     {home: ".codex/skills", project: ".codex/skills"},
	"qoder":     {home: ".qoder/skills", project: ".qoder/skills"},
	"codebuddy": {home: ".codebuddy/skills", project: ".codebuddy/skills"},
	"opencode":  {home: ".config/opencode/skills", project: ".opencode/skills"},
}

// hasAnimationSkill 校验候选目录里确实装了动效技能，并且提示词引用的索引文件存在。
// 只认全套：缺 rules-index.md 时宁可判定为未找到并让上层告警，也不给出一条死路径。
func hasAnimationSkill(dir string) bool {
	if dir == "" {
		return false
	}
	skill := filepath.Join(dir, AnimationSkillName)
	for _, name := range []string{skillManifestFile, rulesIndexFile} {
		info, err := os.Stat(filepath.Join(skill, name))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// projectCandidates 从镜头目录逐级向上，收集每一层的项目级技能目录候选。
// 项目级安装会覆盖全局安装，因此必须先于家目录被检查。
func projectCandidates(sceneDir, relative string) []string {
	current, err := filepath.Abs(sceneDir)
	if err != nil {
		return nil
	}
	var out []string
	for level := 0; level < maxWalkUpLevels; level++ {
		out = append(out, filepath.Join(current, filepath.FromSlash(relative)))
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return out
}

// SkillsEnvironment 把 .env 覆盖层里的 SkillsDirEnv 并入查找用的环境。
// 优先级与 config.Load 一致：真实环境变量 > .env；不修改传入的任何一个 map。
func SkillsEnvironment(baseEnv, overlay map[string]string) map[string]string {
	if baseEnv[SkillsDirEnv] != "" || overlay[SkillsDirEnv] == "" {
		return baseEnv
	}
	merged := make(map[string]string, len(baseEnv)+1)
	for key, value := range baseEnv {
		merged[key] = value
	}
	merged[SkillsDirEnv] = overlay[SkillsDirEnv]
	return merged
}

// ResolveSkillsDir 返回本机装有 AnimationSkillName 的技能目录。
//
// 解析顺序：SkillsDirEnv → 渲染器专属目录（项目级自镜头目录逐级向上，再家目录级）
// → 通用 .agents/skills（同样先项目级再家目录级）。
// 未找到时返回空字符串且不报错，由调用方降级为按技能名引用；
// 只有 SkillsDirEnv 被显式设置却指向无效目录时才报错——配置写错了必须响。
func ResolveSkillsDir(renderer, sceneDir string, environ map[string]string) (string, error) {
	if explicit := environ[SkillsDirEnv]; explicit != "" {
		absolute, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("%s 不是合法路径：%s", SkillsDirEnv, explicit)
		}
		if !hasAnimationSkill(absolute) {
			return "", fmt.Errorf("%s 指向的目录缺少 %s/%s：%s", SkillsDirEnv, AnimationSkillName, rulesIndexFile, absolute)
		}
		return absolute, nil
	}

	home := environ["HOME"]
	homeCandidate := func(relative string) string {
		if home == "" {
			return ""
		}
		return filepath.Join(home, filepath.FromSlash(relative))
	}

	// 先把渲染器专属目录找完再考虑通用目录，否则项目根位于 $HOME 之下时，
	// 向上遍历会在 $HOME 那一层先命中 .agents/skills，抢在专属目录前面。
	var ordered []string
	if locations, known := rendererSkillDirs[renderer]; known {
		ordered = append(ordered, projectCandidates(sceneDir, locations.project)...)
		ordered = append(ordered, homeCandidate(locations.home))
	}
	ordered = append(ordered, projectCandidates(sceneDir, portableSkillsDir)...)
	ordered = append(ordered, homeCandidate(portableSkillsDir))

	for _, candidate := range ordered {
		if hasAnimationSkill(candidate) {
			return candidate, nil
		}
	}
	return "", nil
}

// motionRequirements 是注入每个镜头提示词的动效预算。
// 没有它，渲染器只会用 scale/x/y/opacity 做淡入淡出，把 48 条 rule 全部空置。
func motionRequirements(skillsDir string) string {
	reference := fmt.Sprintf(
		"- 加载 %s 技能，实现前必须读取该技能目录下的 %s（原子动效 rule 索引）。\n"+
			"  （本机未能定位该技能目录，请使用你自身的技能加载机制载入。）\n",
		AnimationSkillName, rulesIndexFile)
	if skillsDir != "" {
		skill := filepath.Join(skillsDir, AnimationSkillName)
		reference = fmt.Sprintf(
			"- 加载 %s 技能。本机该技能目录为：\n    %s\n"+
				"  实现前必须读取其中的 %s（原子动效 rule 索引）；若该路径不存在，\n"+
				"  改用你自身的技能加载机制载入 %s 后读取同名文件。\n",
			AnimationSkillName, skill, rulesIndexFile, AnimationSkillName)
	}
	return "动效要求（强制）：\n" + reference +
		"- 本镜头至少组合 3 条 rule，且必须来自 3 个不同分类" +
		"（文字排版 / 数据统计 / 相机视口 / 布局网络 / SVG 图标 / 环境待机 / 转场运动）。\n" +
		"- 除 scale、x、y、opacity 之外，至少再动用 2 个属性：" +
		"rotation、rotate3d、filter、clip-path、strokeDashoffset、backgroundPosition、translateZ 任选。\n" +
		"- 镜头包含 3 个及以上阶段时，先读同目录 " + blueprintsIndexFile + " 选一个模板再落地。\n" +
		"- 必须有 1 条持续性环境动效（如 sine-wave-loop、ambient-glow-bloom 一类）贯穿整个镜头时长垫底，" +
		"振幅小到不干扰阅读即可，但不得中断。\n" +
		"- 画面不得出现连续超过 45 帧（1.5 秒）的完全静止，镜头结尾同样适用：" +
		"「保留可读稳定状态」指语义元素不再变化，不等于画面冻结，环境动效必须继续运行到最后一帧。\n" +
		"- 新增动效只能落在装饰层（显式标记 decorative）或语义元素的入场 / 退场窗口内，" +
		"不得侵占任何元素的可读稳定区间，也不得改变已约定的短语帧。\n"
}
