// 本文件收拢 am cast 命令组：new / add / validate / preview。
//
// internal/cast 包只定义角色包格式与校验规则，不碰文件系统之外的东西
// （怎么落地、怎么登记进项目班底）。这里是唯一把它接到 CLI 上的地方。
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/cast"
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
仍是人工编辑的唯一真相源，character.json 只是产物，不要手改。`,
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
			_ = os.RemoveAll(dir)
			return fmt.Errorf("写入 %s 失败：%w", name, err)
		}
	}
	pack, err := cast.Load(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("生成的骨架未通过自检，已回滚：%w", err)
	}
	if err := pack.WriteJSON(dir); err != nil {
		_ = os.RemoveAll(dir)
		return err
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
		_ = os.RemoveAll(dest)
		return fmt.Errorf("拷贝角色包失败：%w", err)
	}
	copied, err := cast.Load(dest)
	if err != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("拷贝后的角色包无法通过自检：%w", err)
	}
	if err := copied.WriteJSON(dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	relPath := filepath.ToSlash(dest)
	if err := registerCastPack(".", relPath); err != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("更新 %s 失败：%w", cast.RosterFile, err)
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
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
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
修一个报一个。`,
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
		Long: `cast.Load 校验 PACK 后，把它声明的每个姿势渲成 PNG 并拼成一张
contact sheet，写到 production/cast-preview/，供人眼验收关节角度是否合理。

依赖 rsvg-convert（渲染单张姿势）和 ImageMagick 的 magick（拼图），
两者缺一都会失败并说明缺的是哪个命令。`,
		Example: `  am cast preview cast/heiwa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pack, err := cast.Load(args[0])
			if err != nil {
				return err
			}
			outDir := filepath.Join("production", "cast-preview")
			sheet, err := cast.Preview(pack, outDir)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "预览图已生成：%s\n", sheet)
			return nil
		},
	}
}
