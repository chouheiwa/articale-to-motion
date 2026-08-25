// 本文件收拢 am cast 命令组：new / add / validate / preview。
//
// internal/cast 包只定义角色包格式与校验规则，不碰文件系统之外的东西
// （怎么落地、怎么登记进项目班底）。这里是唯一把它接到 CLI 上的地方。
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/cast"
	"github.com/chouheiwa/articale-to-motion/internal/castbeats"
	"github.com/chouheiwa/articale-to-motion/internal/dialogue"
	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// castDefaultGroundY、castDefaultGapMs 是新建 cast.yaml 时写入的默认班底参数。
const castDefaultGroundY = 0.78

var castDefaultGapMs = cast.GapMs{Turn: 240, Interject: 100}

func newCastCmd(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cast",
		Short: "角色包相关操作",
		Long: `管理角色包（character pack）：生成骨架、引入外部包、校验、生成预览图。

角色包是 cast/<id>/ 下的一个目录，至少包含 character.yaml（清单）与
rig.svg（骨架图）。项目根的 cast.yaml 登记了哪些角色包属于本项目的班底，
它存在与否就是叙事模式本身。

每次 new / add / validate 成功后都会在角色包目录里重新生成 character.json：
它是 character.yaml 的等价 JSON，供驱动库 cast.js 在无头 Chrome 里用
JSON.parse 读取——渲染机的 CSP 环境下引不进 YAML 解析库。character.yaml
仍是人工编辑的唯一真相源，character.json 只是产物，不要手改。

⚠️ 手改 character.yaml 后不会自动同步：character.json 只在 new / add /
validate 成功时重新生成，am 不会比较两者的修改时间。手改完 yaml 却忘记跑一次
am cast validate，渲染机读到的仍是旧 json——不会报错、不会警告，成片里角色
的音色、姿势、关节范围仍是改动前的值，问题只会在肉眼比对渲染结果时才会
发现。改完 character.yaml，一律先跑 am cast validate 再渲染。`,
	}
	cmd.AddCommand(newCastNewCommand(stdout))
	cmd.AddCommand(newCastAddCommand(stdout))
	cmd.AddCommand(newCastValidateCommand(stdout))
	cmd.AddCommand(newCastPreviewCommand(stdout))
	return cmd
}

func newCastNewCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "new ID",
		Args:  exactArgs(1, "ID"),
		Short: "生成一个可自洽的角色包骨架",
		Long: `在 cast/ID/ 下写出 character.yaml、rig.svg、dna.md 三件套骨架。

骨架里的 voiceId 留空：真实音色由人工在 character.yaml 里补上，dna.md 会
提示这一点。写完立即用 cast.Load 自检，一旦不自洽就删除已写文件并报错——
不会在项目里留下半成品。自检通过后额外写出 character.json（见 am cast --help）。

目标目录已存在且非空时拒绝执行，不会覆盖已有内容。`,
		Example: `  am cast new heiwa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCastNew(stdout, args[0])
		},
	}
}

// cleanupAndFail 在失败路径上删除 dir 并返回 cause；删除本身失败时把两个
// 错误一起报出来，而不是用 `_ = os.RemoveAll(dir)` 吞掉——静默吞掉清理失败
// 会在项目里留下残骸（半成品角色包）且完全没有提示。
func cleanupAndFail(dir string, cause error) error {
	if removeErr := os.RemoveAll(dir); removeErr != nil {
		return fmt.Errorf("%w；此外回滚清理 %s 也失败，需要手动删除：%v", cause, dir, removeErr)
	}
	return cause
}

func runCastNew(stdout io.Writer, id string) error {
	dir := filepath.Join("cast", id)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("角色包目录已存在且非空：%s", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建角色包目录失败：%w", err)
	}
	files := map[string]string{
		cast.PackFile: castSkeletonYAML(id),
		"rig.svg":     castSkeletonSVG,
		"dna.md":      castSkeletonDNA(id),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return cleanupAndFail(dir, fmt.Errorf("写入 %s 失败：%w", name, err))
		}
	}
	pack, err := cast.Load(dir)
	if err != nil {
		return cleanupAndFail(dir, fmt.Errorf("生成的骨架未通过自检，已回滚：%w", err))
	}
	if err := pack.WriteJSON(dir); err != nil {
		return cleanupAndFail(dir, err)
	}
	fmt.Fprintf(stdout, "角色包骨架已生成：%s\n", dir)
	fmt.Fprintln(stdout, "请在 character.yaml 里填入真实 voiceId、summary，并按需要调整 rig.svg 后再交付。")
	return nil
}

// castSkeletonYAML 是 am cast new 写出的最小可自洽 character.yaml。
// 关节、viewBox、baselineY 都要与 castSkeletonSVG 严丝合缝，否则骨架自己都
// 过不了 cast.Load 的自检。
func castSkeletonYAML(id string) string {
	return fmt.Sprintf(`schema: %s
id: %s
name: %s
summary: "TODO：一句话概括这个角色的核心行为"
voice:
  minimax: { voiceId: "", speed: 1.0 }
  bailian: { voiceId: "", rate: 1.0 }
rig:
  file: rig.svg
  viewBox: [0, 0, 400, 520]
  baselineY: 512
  joints:
    head:     { pivot: [200, 168], rotate: [-18, 18] }
    frontLeg: { pivot: [176, 330], rotate: [-40, 55] }
scale:
  heightRatio: [0.22, 0.32]
poses:
  idle: {}
  pointing: { frontLeg: 48, head: -6 }
`, cast.SchemaVersion, id, id)
}

// castSkeletonSVG 是与 castSkeletonYAML 的 rig 声明配套的最小占位骨架图：
// 两个 j- 分组，viewBox 与 joints 都对得上。
const castSkeletonSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 520">
  <g id="j-head"><circle cx="200" cy="168" r="60" fill="#fff" stroke="#000" stroke-width="6"/></g>
  <g id="j-frontLeg"><path d="M176 330 L176 420" stroke="#000" stroke-width="6"/></g>
</svg>
`

func castSkeletonDNA(id string) string {
	return fmt.Sprintf(`# %s 角色 DNA

（角色设定草稿，供音色与台词编写参考；不影响 cast.Load 的校验结果。）

## 待办

- [ ] 在 character.yaml 的 voice 里填入真实 voiceId 后才能实际用于配音
      （骨架里留空只是为了让 am cast new 能自洽生成，不会被 validate 拦下，
      但没有真实 voiceId 就没法真正合成语音）
- [ ] 补全 summary、外观与性格描述
- [ ] 按需要替换 rig.svg 里的占位图形，并同步 character.yaml 的关节声明
`, id)
}

func newCastAddCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "add SOURCE-DIR",
		Args:  exactArgs(1, "SOURCE-DIR"),
		Short: "引入一个外部角色包",
		Long: `校验 SOURCE-DIR 是一个自洽的角色包后，把它拷贝进 cast/<id>/ 并登记进
项目根的 cast.yaml。

先校验后拷贝，不是拷贝后回滚：SOURCE-DIR 校验不通过时，项目里不会留下任何
新文件。拷贝会拒绝源目录里出现的任何符号链接——理由与 am init 拒绝模板里的
符号链接一样：链接可能指向项目外，拷进项目后会被当成普通文件静默读取，是
一个隐蔽的路径逃逸口子。

cast.yaml 不存在时会新建，defaults 取 ground_y: 0.78、
gap_ms: {turn: 240, interject: 100}；已存在则在其 packs 列表里追加一条，
已登记过的角色包不会重复追加。

成功后会在 cast/<id>/ 里写出 character.json（见 am cast --help）。`,
		Example: `  am cast add ../shared-characters/heiwa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCastAdd(stdout, args[0])
		},
	}
}

func runCastAdd(stdout io.Writer, source string) error {
	pack, err := cast.Load(source)
	if err != nil {
		return fmt.Errorf("角色包校验未通过，未做任何改动：%w", err)
	}
	dest := filepath.Join("cast", pack.ID)
	if _, statErr := os.Stat(dest); statErr == nil {
		return fmt.Errorf("角色 %s 已存在于 %s，请先移除或换一个 id", pack.ID, dest)
	}
	if err := copyTree(source, dest); err != nil {
		return cleanupAndFail(dest, fmt.Errorf("拷贝角色包失败：%w", err))
	}
	copied, err := cast.Load(dest)
	if err != nil {
		return cleanupAndFail(dest, fmt.Errorf("拷贝后的角色包无法通过自检：%w", err))
	}
	if err := copied.WriteJSON(dest); err != nil {
		return cleanupAndFail(dest, err)
	}
	relPath := filepath.ToSlash(dest)
	if err := registerCastPack(".", relPath); err != nil {
		return cleanupAndFail(dest, fmt.Errorf("更新 %s 失败：%w", cast.RosterFile, err))
	}
	fmt.Fprintf(stdout, "角色包已引入：%s（id=%s），并已登记进 %s\n", dest, pack.ID, cast.RosterFile)
	return nil
}

// copyTree 把 src 整棵目录树拷贝到 dst，拒绝源目录里出现的任何符号链接。
//
// 与 internal/project.rejectSymlinkPath 同样的理由：符号链接可能指向项目外，
// 拷贝进项目后会被当作普通文件静默读取，是一个隐蔽的路径逃逸口子。
// filepath.WalkDir 给的 DirEntry 对符号链接本身不跟随（等价于 Lstat），
// 因此这里能在描述符号链接本身、而不是它指向的文件时就拦下来。
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("读取 %s 失败：%w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("源目录包含符号链接，拒绝拷贝：%s", path)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取 %s 失败：%w", path, err)
		}
		// 复用 fsutil.AtomicWrite：先写临时文件再 rename，建目录也是它内部做的。
		// 裸 os.WriteFile 不是原子的——进程被杀这类不走 error 返回路径的场景
		// 会在 cast/<id>/ 下留半截文件，而 runCastAdd 的回滚逻辑只在 error
		// 路径上触发，兜不住这种情况。
		return fsutil.AtomicWrite(target, body, 0o644)
	})
}

// registerCastPack 把 relPath 追加进项目根 root 的 cast.yaml。
// cast.yaml 不存在时按约定默认值新建；已登记过同一路径则不重复追加。
func registerCastPack(root, relPath string) error {
	var roster cast.Roster
	if cast.HasRoster(root) {
		loaded, err := cast.LoadRoster(root)
		if err != nil {
			return err
		}
		roster = loaded
	} else {
		roster = cast.Roster{
			Schema:   cast.SchemaVersion,
			Defaults: cast.Defaults{GroundY: castDefaultGroundY, GapMs: castDefaultGapMs},
		}
	}
	for _, existing := range roster.Packs {
		if existing == relPath {
			return nil
		}
	}
	roster.Packs = append(roster.Packs, relPath)
	body, err := yaml.Marshal(roster)
	if err != nil {
		return fmt.Errorf("序列化 %s 失败：%w", cast.RosterFile, err)
	}
	if err := os.WriteFile(filepath.Join(root, cast.RosterFile), body, 0o644); err != nil {
		return fmt.Errorf("写入 %s 失败：%w", cast.RosterFile, err)
	}
	return nil
}

func newCastValidateCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "validate [PACK...]",
		Short: "校验角色包",
		Long: `校验角色包能否通过 cast.Load，并重新生成它们的 character.json。

不传参数时：
  - 项目根有 cast.yaml，则校验它 packs 列表里登记的全部角色包；
  - 没有 cast.yaml，则扫描 cast/ 目录下每一个含 character.yaml 的子目录；
  - 两者都没有则视为无事可做，退出码 0。

传参数时，把每个参数当成角色包目录，逐个 cast.Load，与是否登记进
cast.yaml 无关。

任一角色包校验失败就返回非零退出码；问题按包分组打印，一次看到全部，不是
修一个报一个。

校验通过顺带重新生成 character.json（渲染机实际读取的格式）。手改
character.yaml 后如果没跑这条命令，character.json 就是旧的且不会有任何
报错或警告——渲染机会静默用回旧值。改完 yaml，渲染前一律先跑一次。`,
		Example: `  am cast validate
  am cast validate cast/heiwa cast/xiaoming`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCastValidate(stdout, args)
		},
	}
}

type castTarget struct {
	name string
	dir  string
}

func runCastValidate(stdout io.Writer, args []string) error {
	targets, err := castValidateTargets(args)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Fprintln(stdout, "没有找到需要校验的角色包")
		return nil
	}
	report, failed := validateCastTargets(targets)
	fmt.Fprint(stdout, report)
	if len(failed) > 0 {
		return fmt.Errorf("以下角色包未通过校验：%s", strings.Join(failed, "、"))
	}
	return nil
}

func castValidateTargets(args []string) ([]castTarget, error) {
	if len(args) > 0 {
		targets := make([]castTarget, 0, len(args))
		for _, a := range args {
			targets = append(targets, castTarget{name: a, dir: a})
		}
		return targets, nil
	}
	if cast.HasRoster(".") {
		roster, err := cast.LoadRoster(".")
		if err != nil {
			return nil, err
		}
		targets := make([]castTarget, 0, len(roster.Packs))
		for _, rel := range roster.Packs {
			targets = append(targets, castTarget{name: rel, dir: rel})
		}
		return targets, nil
	}
	return discoverCastPacks(".")
}

// discoverCastPacks 在没有 cast.yaml 时兜底：把 cast/ 下每一个含
// character.yaml 的子目录都当作待校验的角色包。cast/ 不存在时返回空列表，
// 不是错误——刚 am init 的项目本来就还没有任何角色包。
func discoverCastPacks(root string) ([]castTarget, error) {
	castDir := filepath.Join(root, "cast")
	entries, err := os.ReadDir(castDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 %s 失败：%w", castDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	targets := make([]castTarget, 0, len(names))
	for _, name := range names {
		dir := filepath.Join(castDir, name)
		if _, err := os.Stat(filepath.Join(dir, cast.PackFile)); err != nil {
			continue
		}
		targets = append(targets, castTarget{name: name, dir: dir})
	}
	return targets, nil
}

// validateCastTargets 逐个 cast.Load，成功则重新生成 character.json。
// 返回按包分组的报告文本，以及未通过校验的包名列表。
func validateCastTargets(targets []castTarget) (string, []string) {
	var report strings.Builder
	var failed []string
	for _, target := range targets {
		pack, err := cast.Load(target.dir)
		if err != nil {
			failed = append(failed, target.name)
			fmt.Fprintf(&report, "[%s]\n  - %v\n", target.name, err)
			continue
		}
		if err := pack.WriteJSON(target.dir); err != nil {
			failed = append(failed, target.name)
			fmt.Fprintf(&report, "[%s]\n  - %v\n", target.name, err)
			continue
		}
		fmt.Fprintf(&report, "[%s] 通过\n", target.name)
	}
	return report.String(), failed
}

func newCastPreviewCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "preview PACK",
		Args:  exactArgs(1, "PACK"),
		Short: "把角色包的每个命名姿势渲成一张验收图",
		Long: `cast.Load 校验 PACK 后，为它的每一个视图各自把声明的每个姿势渲成 PNG
并拼成一张 contact sheet，写到 production/cast-preview/，供人眼验收关节
角度是否合理。角色包只有默认视图时只会生成一张。

依赖 rsvg-convert（渲染单张姿势）和 ImageMagick 的 magick（拼图），
两者缺一都会失败并说明缺的是哪个命令。`,
		Example: `  am cast preview cast/heiwa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCastPreview(stdout, args[0])
		},
	}
}

func runCastPreview(stdout io.Writer, packDir string) error {
	pack, err := cast.Load(packDir)
	if err != nil {
		return err
	}
	outDir := filepath.Join("production", "cast-preview")
	sheets, err := cast.Preview(pack, outDir)
	if err != nil {
		return err
	}
	viewNames := make([]string, 0, len(sheets))
	for viewName := range sheets {
		viewNames = append(viewNames, viewName)
	}
	sort.Strings(viewNames)
	for _, viewName := range viewNames {
		fmt.Fprintf(stdout, "预览图已生成（视图 %s）：%s\n", viewName, sheets[viewName])
	}
	return nil
}

// dialogueDefaultPlanPath 是 am dialogue assemble 未传 --plan 时的默认路径，
// 与编排流程里 TTS 分段产物的落地位置一致。
var dialogueDefaultPlanPath = filepath.Join("production", "audio", "plan.json")

// newDialogueCmd 收拢 am dialogue 命令组：目前只有 assemble 一个子命令。
//
// internal/dialogue 包只管纯粹的装配逻辑（读 plan、探测音频、拼接、写产物），
// 不碰 CLI 参数解析；这里是唯一把它接到命令行上的地方，与 newCastCmd 对
// internal/cast 的分工一致。
func newDialogueCmd(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dialogue",
		Short: "多角色对白装配相关操作",
		Long: `把分说话人合成的配音段装配成一条完整对白时间线。

分段 TTS 只给得到段内相对时间戳，拼成全局时间轴要逐段累加偏移、插入段间
静音、再拼接成一条完整音轨——这类机械计算算错了不会报错，只会让成片画面
正常、嘴和字却对不上，所以收进这个命令而不是留给编排 agent 自己拼。`,
	}
	cmd.AddCommand(newDialogueAssembleCommand(stdout))
	cmd.AddCommand(newDialogueBeatsCommand(stdout))
	return cmd
}

func newDialogueBeatsCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "beats",
		Args:  noArgs,
		Short: "按 dialogue.json 重算各镜头 scene.json 的 cast.beats",
		Long: `读取 ` + castbeats.DialogueRelPath + ` 与 scenes/*/scene.json，把每一行台词
按镜头切片、减去该镜头的全局起点，写回各镜头 cast 块的 beats。

镜头怎么切是创作判断（每个镜头的 duration_seconds 由你定），但"给定对白
时间线与已定的镜头时长，算出每个镜头的节拍"是纯机械计算——减错一个镜头
起点，成片画面正常、只是角色在不该说话的时候动嘴，什么都不会报错。所以
这一步收进本命令，不要手写 beats。

行为：

  - 一行台词横跨镜头切点时按切点拆成多拍，各自落进自己的镜头
  - 有 cast 块但整段无人说话的镜头写出空数组（"算过了，确实没人说话"）
  - 没有 cast 块又没有台词盖过的镜头一个字节都不碰（老镜头零影响）
  - 只改 cast.beats：pack_dir / ground_y / on_stage 与其它字段原样保留
  - 既有 beats 一律忽略并整体重写：它是本命令的输出，不是输入。改小过
    duration_seconds、旧节拍越界的镜头不需要先手工清理，直接重跑
  - 幂等：重复执行结果一致，内容没变的文件不重新落盘

项目根没有 cast.yaml（单口播项目）时直接报错退出：cast.beats 只在多角色
模式下存在。

有台词盖过、却没写 cast 块的镜头会报错而不是现编一个——本命令不替你决定
角色站位。台词落在所有镜头覆盖范围之外时同样报错，那是镜头 duration_seconds
与配音对不上，先对齐时长再重算。任一检查不通过就一个文件都不写。`,
		Example: `  am dialogue beats`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDialogueBeats(stdout)
		},
	}
	return cmd
}

func runDialogueBeats(stdout io.Writer) error {
	report, err := castbeats.Apply(".")
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "镜头节拍已按 %s 重算（%d 行台词）：\n", castbeats.DialogueRelPath, report.Lines)
	for _, s := range report.Scenes {
		mark := "（未变）"
		if s.Changed {
			mark = ""
		}
		fmt.Fprintf(stdout, "  %s  %d 拍%s\n", s.ID, s.Count, mark)
	}
	fmt.Fprintf(stdout, "已更新 %d 个 scene.json\n", report.Updated)
	return nil
}

func newDialogueAssembleCommand(stdout io.Writer) *cobra.Command {
	var planPath string
	var expectTotalSeconds float64
	cmd := &cobra.Command{
		Use:   "assemble",
		Args:  noArgs,
		Short: "把 plan.json 里的分段配音装配成一条对白时间线",
		Long: fmt.Sprintf(`读取 --plan 指定的 plan.json，把里面登记的每一段配音统一格式、按需插入
段间静音后拼接成一条完整音轨，同时算出全局时间轴，产出三个文件：

  %s       拼接后的完整配音
  %s     按全局时间轴生成的字幕
  %s         装配结果的结构化时间线（schema %s）

装配过程内置两道漂移断言：逐段声明时长与实测时长的比对、总时长漂移
（容差 %.3f 秒）比对，任一超出容差都会失败并点名具体是第几段、哪个文件、
声明多少、实测多少——本命令原样透出这些错误，不额外包装。

--expect-total 是可选的第三道校验：外部（通常是编排 agent）期望的总时长，
用于额外核对；不传就只做上面两道内部一致性断言。`,
			dialogue.VoiceRelPath, dialogue.SRTRelPath, dialogue.DialogueRelPath,
			dialogue.SchemaVersion, dialogue.DriftToleranceSeconds),
		Example: `  am dialogue assemble
  am dialogue assemble --plan production/audio/plan.json --expect-total 42.5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDialogueAssemble(cmd.Context(), stdout, planPath, expectTotalSeconds)
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", dialogueDefaultPlanPath, "plan.json 路径")
	cmd.Flags().Float64Var(&expectTotalSeconds, "expect-total", 0, "外部期望总时长（秒），用于额外校验；可选，缺省不做这道校验")
	return cmd
}

func runDialogueAssemble(ctx context.Context, stdout io.Writer, planPath string, expectTotalSeconds float64) error {
	result, err := dialogue.Assemble(ctx, dialogue.Options{
		Root:               ".",
		PlanPath:           planPath,
		ExpectTotalSeconds: expectTotalSeconds,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, "对白已装配：")
	fmt.Fprintf(stdout, "  音频：%s\n", dialogue.VoiceRelPath)
	fmt.Fprintf(stdout, "  字幕：%s\n", dialogue.SRTRelPath)
	fmt.Fprintf(stdout, "  时间线：%s\n", dialogue.DialogueRelPath)
	fmt.Fprintf(stdout, "总时长：%.3f 秒\n", result.TotalSeconds())
	return nil
}
