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

	skillManifestFile   = "SKILL.md"
	rulesIndexFile      = "rules-index.md"
	blueprintsIndexFile = "blueprints-index.md"

	// portableSkillsDir 是不绑定具体工具的通用约定，作为兜底候选。
	portableSkillsDir = ".agents/skills"

	// maxWalkUpLevels 限制向上查找项目级技能目录的层数，避免病态路径下空转。
	maxWalkUpLevels = 40
)

// AnimationSkillName 是承载全部动效知识的技能目录名。
const AnimationSkillName = "hyperframes-animation"

// AlgorithmicArtSkillName 是生成式艺术技能目录名。
const AlgorithmicArtSkillName = "algorithmic-art"

// TextToLottieSkillName 是 Lottie 图层制作技能目录名。
const TextToLottieSkillName = "text-to-lottie"

// SkillDescriptor 描述一个可自动发现的技能。
type SkillDescriptor struct {
	// Name 是技能目录名。
	Name string
	// RequiredFiles 是技能被视为「完整安装」所必须存在的文件列表。
	// 至少包含 SKILL.md；动效技能额外要求 rules-index.md。
	RequiredFiles []string
	// PromptSection 返回该技能注入镜头提示词的片段。
	// skillsDir 为空时应退回按技能名引用，不写死本机路径。
	PromptSection func(skillsDir string) string
}

// RegisteredSkills 是所有自动发现的技能注册表。
// 新增技能只需在此追加一条，ResolveAllSkills 和 BuildPrompt 自动覆盖。
var RegisteredSkills = []SkillDescriptor{
	{
		Name:          AnimationSkillName,
		RequiredFiles: []string{skillManifestFile, rulesIndexFile},
		PromptSection: animationPrompt,
	},
	{
		Name:          AlgorithmicArtSkillName,
		RequiredFiles: []string{skillManifestFile},
		PromptSection: algorithmicArtPrompt,
	},
	{
		Name:          TextToLottieSkillName,
		RequiredFiles: []string{skillManifestFile},
		PromptSection: textToLottiePrompt,
	},
}

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

// hasSkill 校验候选目录里确实装了指定技能，并且必需文件全部存在。
func hasSkill(dir, skillName string, requiredFiles []string) bool {
	if dir == "" {
		return false
	}
	skill := filepath.Join(dir, skillName)
	for _, name := range requiredFiles {
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

// ResolveSkill 在渲染器的技能目录树中查找指定技能。
//
// 查找顺序：SkillsDirEnv → 渲染器专属目录（项目级自镜头目录逐级向上，再家目录级）
// → 通用 .agents/skills（同样先项目级再家目录级）。
// 未找到时返回空字符串且不报错，由调用方降级为按技能名引用；
// 只有 SkillsDirEnv 被显式设置却指向无效目录时才报错——配置写错了必须响。
func ResolveSkill(renderer, sceneDir string, environ map[string]string, desc SkillDescriptor) (string, error) {
	if explicit := environ[SkillsDirEnv]; explicit != "" {
		absolute, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("%s 不是合法路径：%s", SkillsDirEnv, explicit)
		}
		if !hasSkill(absolute, desc.Name, desc.RequiredFiles) {
			return "", fmt.Errorf("%s 指向的目录缺少 %s/%s：%s", SkillsDirEnv, desc.Name, desc.RequiredFiles[0], absolute)
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
		if hasSkill(candidate, desc.Name, desc.RequiredFiles) {
			return candidate, nil
		}
	}
	return "", nil
}

// ResolveSkillsDir 返回本机装有 AnimationSkillName 的技能目录。
// 保留用于只关心动效技能的调用点。
func ResolveSkillsDir(renderer, sceneDir string, environ map[string]string) (string, error) {
	return ResolveSkill(renderer, sceneDir, environ, RegisteredSkills[0])
}

// ResolveAllSkills 解析所有注册技能，返回 name → 绝对路径的映射。
// 未找到的技能不出现 in map 中（不是报错）。
func ResolveAllSkills(renderer, sceneDir string, environ map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	for _, desc := range RegisteredSkills {
		dir, err := ResolveSkill(renderer, sceneDir, environ, desc)
		if err != nil {
			return nil, err
		}
		if dir != "" {
			result[desc.Name] = dir
		}
	}
	return result, nil
}

// skillPromptSections 为所有已注册技能生成提示词片段。
// 已解析的技能注入本机路径；未解析的退回按技能名引用。
func skillPromptSections(resolved map[string]string) string {
	var sections string
	for _, desc := range RegisteredSkills {
		dir := resolved[desc.Name]
		sections += desc.PromptSection(dir)
	}
	return sections
}

// animationPrompt 是动效技能的提示词片段。
//
// 其中的分类清单必须与 hyperframes-animation 的 rules-index.md 章节保持一致
// （截至 0.7.94 是 8 个：Text & Typography / Data & Stats / Camera & Viewport /
// Layout & Network / SVG & Icons / Idle & Ambient / Transition & Motion /
// Effect Recipes，共 48 条 rule）。
//
// 这里没有测试能守：清单在联网安装的技能里，本仓库拿不到。升 HyperFrames 固定
// 版本时要顺手核对一遍——漏掉一个分类不会报错，只会让「来自 3 个不同分类」这条
// 约束对该分类的 rule 失效。
func animationPrompt(skillsDir string) string {
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
		"（文字排版 / 数据统计 / 相机视口 / 布局网络 / SVG 图标 / 环境待机 / 转场运动 / 特效配方）。\n" +
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

// algorithmicArtPrompt 是生成式艺术技能的提示词片段。
func algorithmicArtPrompt(skillsDir string) string {
	if skillsDir != "" {
		skill := filepath.Join(skillsDir, AlgorithmicArtSkillName)
		return fmt.Sprintf("生成式艺术参考：\n"+
			"- 如需使用算法艺术/生成式视觉效果，加载 %s 技能。本机该技能目录为：\n    %s\n"+
			"  读取其中的 SKILL.md（工作流与不变量）和 references/ 目录（技术目录、配色规则、动画配方）。\n"+
			"- 该技能提供 50+ 算法技术（10 个分类）、OKLab 配色插值、完美循环动画模板。\n"+
			"- 使用时遵守反默认规则：禁止单独使用 Perlin 流场/粒子轨迹/圆填充/递归树/Voronoi 作为唯一想法，\n"+
			"  必须组合两个正交技术或使用目录中更冷门的算法。\n",
			AlgorithmicArtSkillName, skill)
	}
	return fmt.Sprintf("生成式艺术参考：\n"+
		"- 如需使用算法艺术/生成式视觉效果，加载 %s 技能。\n"+
		"  （本机未能定位该技能目录，请使用你自身的技能加载机制载入。）\n"+
		"- 该技能提供 50+ 算法技术、OKLab 配色、完美循环动画。使用时遵守反默认规则。\n",
		AlgorithmicArtSkillName)
}

// textToLottiePrompt 是 Lottie 图层技能的提示词片段。
//
// Lottie 是可选项而非默认项：烘进 JSON 的东西后期改不动，且文字与数据类动画
// 交给 Lottie 会与动效 rule 体系给出互相冲突的指令。因此这里的措辞必须把
// 「默认不用」和「不计入动效预算」两条写死，否则渲染器会拿一个 Lottie 顶替
// 整镜头的动效工作量。
func textToLottiePrompt(skillsDir string) string {
	reference := fmt.Sprintf(
		"- 需要时加载 %s 技能。\n"+
			"  （本机未能定位该技能目录，请使用你自身的技能加载机制载入。）\n",
		TextToLottieSkillName)
	if skillsDir != "" {
		skill := filepath.Join(skillsDir, TextToLottieSkillName)
		reference = fmt.Sprintf(
			"- 需要时加载 %s 技能。本机该技能目录为：\n    %s\n"+
				"  读取其中的 %s 判断该不该用，再按其路由表只读匹配的 references/ 文件；\n"+
				"  若该路径不存在，改用你自身的技能加载机制载入 %s。\n",
			TextToLottieSkillName, skill, skillManifestFile, TextToLottieSkillName)
	}
	return "Lottie 图层（可选）：\n" + reference +
		"- 默认不用 Lottie。只有矢量图形、时间线自成一体、且用 CSS/GSAP 表达明显更笨重时才考虑，" +
		"典型是 logo 演绎、图标变形、loader、状态反馈、SVG 描边绘制、矢量质感特效。\n" +
		"- 文字排版、逐字动画、数字滚动、图表、转场与运镜一律不得交给 Lottie，" +
		"这些归 HTML 与动效 rule 体系。任何时候都不得产出铺满画幅、承担镜头全部内容的 Lottie。\n" +
		"- Lottie 图层不计入上述动效预算的任何一条 rule，持续性环境动效也不得用 Lottie 循环顶替；" +
		"HTML 侧必须独立满足全部动效要求。\n" +
		"- Lottie 由渲染框架逐帧 seek：播放器实例 autoplay 与 loop 均为 false 并注册到框架约定的全局数组，" +
		"JSON 内不得使用表达式、时钟或未播种随机，粒子与物理一类系统必须烘成关键帧。\n"
}
