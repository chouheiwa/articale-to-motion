// Package hyperframes 把上游官方技能装进用户项目，而不是装进用户 HOME。
//
// 上游的 `hyperframes skills` 只认 homedir()：它把技能写进 ~/.claude/skills 与
// ~/.agents/skills，再镜像到本机其余几十个 agent 目录。没有任何命令行选项或环境
// 变量能改这个落点——`skills update --dir` 的帮助原文就写着 "scopes the prune,
// not the install"。
//
// 装在机器级有三个后果，第三个是硬伤：
//
//   - 项目不自包含。整个项目拷到另一台机器就渲不出来，技能不在里面。
//   - 污染用户 HOME，一次安装会写进本机所有已安装 agent 的目录。
//   - 版本固定名存实亡。下发的 PROMPT 要求「固定 HyperFrames 版本为 X」，
//     可机器上只有一份技能：项目 A 固定 0.8.1、项目 B 固定 0.7.108，
//     谁后初始化谁说了算，另一个项目的固定版本悄悄失效。
//
// 所以这里改成：在项目之外开一个临时 HOME 让上游安装器照常工作，再把它产出的
// 技能树整体搬进项目的 .agents/skills/。scene.ResolveSkill 本来就优先项目级
// 候选，搬进去即可被优先命中，不需要改解析逻辑。
package hyperframes

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/envutil"
)

// SkillsSubdir 是技能在项目里的落点，与 scene.portableSkillsDir 一致。
const SkillsSubdir = ".agents/skills"

// manifestFile 是判定一个目录是不是技能的依据，与 scene 包一致。
const manifestFile = "SKILL.md"

// Options 描述一次安装。
type Options struct {
	// ProjectDir 是用户项目根目录。
	ProjectDir string
	// Version 是要固定安装的上游版本，例如 "0.8.1"。
	Version string
	// Env 是当前进程环境。安装器要在改写过 HOME 的副本里运行，
	// 但 PATH、代理、npm 配置这些必须沿用，否则公司内网装不上。
	Env map[string]string
	// Protected 列出不允许被上游同名技能覆盖的技能，即随二进制下发的那些。
	Protected map[string]bool
	// Output 接收安装器的进度输出。
	Output io.Writer
}

// Result 汇报安装结果。
type Result struct {
	// Installed 是写进项目的技能名，已排序。
	Installed []string
	// Protected 是因为与内置技能重名而跳过的上游技能名。
	Protected []string
}

// Install 把固定版本的上游技能装进 opts.ProjectDir 下的 .agents/skills/。
//
// 全程不碰用户 HOME：安装器在一个临时目录里以为自己在写 home，结束后整棵树
// 被搬进项目，临时目录删除。
func Install(ctx context.Context, opts Options) (Result, error) {
	if opts.Version == "" {
		return Result{}, fmt.Errorf("必须指定 HyperFrames 版本")
	}
	npx, nodeBin, err := resolveNPX(opts.Env)
	if err != nil {
		return Result{}, err
	}
	// 临时 HOME 必须在项目之外：放项目里会把 npm 缓存和 .hyperframes
	// 这类中间产物留在交付目录，而且 am archive 会连它们一起归档。
	tmpHome, err := os.MkdirTemp("", "am-hyperframes-")
	if err != nil {
		return Result{}, fmt.Errorf("无法创建临时目录：%w", err)
	}
	defer os.RemoveAll(tmpHome)

	if err := runInstaller(ctx, npx, nodeBin, tmpHome, opts); err != nil {
		return Result{}, err
	}
	source := filepath.Join(tmpHome, SkillsSubdir)
	if _, err := os.Stat(source); err != nil {
		return Result{}, fmt.Errorf("安装器没有产出 %s，无法确定它把技能装到了哪里", SkillsSubdir)
	}
	return copySkills(source, filepath.Join(opts.ProjectDir, SkillsSubdir), opts.Protected)
}

// resolveNPX 返回可直接执行的 npx 路径，以及它所在的 bin 目录。
//
// 不能直接用 PATH 里的 npx：asdf / nvm / volta / mise 这类版本管理器在 PATH 上
// 放的是 shim，而 shim 要靠 HOME 找到真正的 node——一旦改写 HOME，shim 就会报
// "unknown command: npx. Perhaps you have to reshim?"。
//
// 让 node 自己汇报 process.execPath 可以绕过全部这些差异：拿到的是真实二进制
// 路径，与用哪个版本管理器无关，之后改 HOME 也不影响它。
func resolveNPX(env map[string]string) (npx string, nodeBin string, err error) {
	node, err := envutil.LookPath("node", env["PATH"])
	if err != nil {
		return "", "", err
	}
	probe := exec.Command(node, "-e", "process.stdout.write(process.execPath)")
	probe.Env = envutil.EnvList(env)
	output, probeErr := probe.Output()
	real := strings.TrimSpace(string(output))
	if probeErr != nil || real == "" {
		// 问不出来就退回 PATH 上的那个，附带说明——总比直接失败好。
		real = node
	}
	nodeBin = filepath.Dir(real)
	candidate := filepath.Join(nodeBin, "npx")
	if info, statErr := os.Stat(candidate); statErr == nil && info.Mode()&0o111 != 0 {
		return candidate, nodeBin, nil
	}
	// node 旁边没有 npx（少见的打包方式），退回 PATH 查找。
	fallback, lookErr := envutil.LookPath("npx", env["PATH"])
	if lookErr != nil {
		return "", "", lookErr
	}
	return fallback, nodeBin, nil
}

// installerEnv 组装安装器的运行环境。
//
// HOME 指向临时目录，其余尽量沿用原环境。两个 npm 变量必须显式接回真实位置：
//
//   - npm_config_cache：不接的话 npm 会把缓存写进临时 HOME，每个项目都要重新
//     下载整包，几百 MB 的临时占用还拿不到复用。
//   - npm_config_userconfig：不接的话读不到用户的 .npmrc，私有 registry、代理
//     和鉴权全部丢失，公司内网会直接装不上。
func installerEnv(tmpHome, nodeBin string, env map[string]string) map[string]string {
	out := make(map[string]string, len(env)+4)
	for key, value := range env {
		out[key] = value
	}
	originalHome := env["HOME"]
	out["HOME"] = tmpHome
	if originalHome != "" {
		if _, set := out["npm_config_cache"]; !set {
			out["npm_config_cache"] = filepath.Join(originalHome, ".npm")
		}
		if _, set := out["npm_config_userconfig"]; !set {
			out["npm_config_userconfig"] = filepath.Join(originalHome, ".npmrc")
		}
	}
	// 把真实 node 所在目录放到 PATH 首位，让安装器内部再调 node/npm 时
	// 拿到的是同一个版本，而不是被改写 HOME 之后失效的 shim。
	if nodeBin != "" {
		if current := out["PATH"]; current != "" {
			out["PATH"] = nodeBin + string(os.PathListSeparator) + current
		} else {
			out["PATH"] = nodeBin
		}
	}
	return out
}

func runInstaller(ctx context.Context, npx, nodeBin, tmpHome string, opts Options) error {
	pinned := "hyperframes@" + opts.Version
	cmd := exec.CommandContext(ctx, npx, "--yes", pinned, "skills")
	cmd.Dir = tmpHome
	cmd.Env = envutil.EnvList(installerEnv(tmpHome, nodeBin, opts.Env))
	cmd.Stdout, cmd.Stderr = opts.Output, opts.Output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("HyperFrames 技能安装失败（npx --yes %s skills）：%w", pinned, err)
	}
	return nil
}

// copySkills 把 source 下的每个技能目录整体搬进 dest。
//
// 上游技能由 am 按固定版本管理，所以同名目录整体替换而不是合并：残留上一版的
// 文件会让技能树处于两个版本混合的状态，而那正是「固定版本」要消除的东西。
// 随二进制下发的技能受 protected 保护，永远不会被上游同名技能覆盖。
func copySkills(source, dest string, protected map[string]bool) (Result, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return Result{}, fmt.Errorf("无法读取安装产物：%w", err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return Result{}, err
	}
	var result Result
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(source, name, manifestFile)); statErr != nil {
			continue // 不是技能目录
		}
		if protected[name] {
			result.Protected = append(result.Protected, name)
			continue
		}
		target := filepath.Join(dest, name)
		if err := os.RemoveAll(target); err != nil {
			return Result{}, err
		}
		if err := copyTree(filepath.Join(source, name), target); err != nil {
			return Result{}, fmt.Errorf("拷贝技能 %s 失败：%w", name, err)
		}
		result.Installed = append(result.Installed, name)
	}
	if len(result.Installed) == 0 {
		return Result{}, fmt.Errorf("安装产物里没有任何技能目录：%s", source)
	}
	sort.Strings(result.Installed)
	sort.Strings(result.Protected)
	return result, nil
}

// copyTree 递归拷贝目录。符号链接一律拒绝：上游在 Unix 上用符号链接做的是
// 「镜像到其他 agent 目录」，指回它自己的 home store；搬进项目后那些链接会指向
// 一个已经删掉的临时目录，成为静默失效的空技能。
func copyTree(source, dest string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, relative)
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, 0o755)
		case entry.Type()&os.ModeSymlink != 0:
			return fmt.Errorf("安装产物含符号链接，无法安全搬进项目：%s", relative)
		case !entry.Type().IsRegular():
			return nil // 设备、套接字之类一律忽略
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode().Perm())
	})
}
