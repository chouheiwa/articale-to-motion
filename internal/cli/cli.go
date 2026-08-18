package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	assets "github.com/chouheiwa/articale-to-motion"
	"github.com/chouheiwa/articale-to-motion/internal/archive"
	"github.com/chouheiwa/articale-to-motion/internal/config"
	"github.com/chouheiwa/articale-to-motion/internal/envutil"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
	"github.com/chouheiwa/articale-to-motion/internal/preset"
	"github.com/chouheiwa/articale-to-motion/internal/project"
	"github.com/chouheiwa/articale-to-motion/internal/scene"
	"github.com/chouheiwa/articale-to-motion/internal/schedule"
	"github.com/chouheiwa/articale-to-motion/internal/srt"
	"github.com/chouheiwa/articale-to-motion/internal/tools"
	"github.com/chouheiwa/articale-to-motion/internal/validate"
	"github.com/spf13/cobra"
)

const Version = "1.0.0"

// hyperframesVersion 是 am init 安装的 HyperFrames 官方技能版本。
//
// 固定版本不自动前进是刻意的：技能内容变化会直接改变成片动效。升级前要对比
// 两版 skills/ 树与 fontData.generated.ts，确认动效 rule 与自动内联字体清单
// 没变，并同步 internal/validate 的 autoEmbeddedFonts 注释。
//
// 这里是唯一来源：命令行、错误信息与 --help 都从它取值，避免三处字面量各自漂移。
const hyperframesVersion = "0.7.108"

func currentEnvironment() map[string]string {
	return envutil.EnvMap()
}

func addExecutableLocation(env map[string]string) {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：无法获取可执行文件路径：%v\n", err)
		return
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：无法解析可执行文件路径：%v\n", err)
		return
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	env["AM_EXECUTABLE"] = executable
	directory := filepath.Dir(executable)
	if current := env["PATH"]; current != "" {
		env["PATH"] = directory + string(os.PathListSeparator) + current
	} else {
		env["PATH"] = directory
	}
}

func Execute(args []string, stdout, stderr io.Writer) int {
	return ExecuteContext(context.Background(), args, stdout, stderr)
}

func ExecuteContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := newRoot(stdout, stderr)
	root.SetContext(ctx)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, err)
		var coded *exitError
		if errors.As(err, &coded) {
			return coded.code
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return 130
		}
		if strings.Contains(err.Error(), "缺少必需工具") {
			return 127
		}
		return 1
	}
	return 0
}

func newRoot(stdout, stderr io.Writer) *cobra.Command {
	unsafe := false
	root := &cobra.Command{
		Use:   "am",
		Short: "ArticleToMotion 竖屏 MG 视频制作工具",
		Long: `ArticleToMotion 竖屏 MG 视频制作工具。

把定稿 SRT 拆成并发渲染的动画镜头，或从文章/口播稿走完 TTS、字幕、封面、
声音与发布交付的全流程。渲染由外部 AI CLI 完成，am 负责契约、调度与校验。

两种入口：
  am run                          项目已有定稿 transcription.srt，读项目根 PROMPT.md
  am run PROMPT-PRODUCTION.md     从文章或口播稿开始完整制作
  am scene run / run-all          只跑镜头渲染，可脱离 am run 单独使用

配置解析优先级（高到低）：环境变量 > 项目 .env > article-to-motion.conf > 内置默认。
用 am config get 读取解析结果，不要自己解析配置文件。
项目 .env 不允许出现以 API_KEY / TOKEN / SECRET / PASSWORD 结尾的键，出现则拒绝启动。

外部 AI CLI 默认运行在限制于项目工作区的安全模式。安全模式下子进程只继承白名单
环境变量加上 .env 覆盖层；缺变量用 AM_PASSTHROUGH_ENV=NAME1,NAME2 显式列出，
不要改用 --unsafe 绕过隔离。

退出码：
  0    成功
  1    一般失败（参数、校验、配置、IO）
  127  缺少必需的外部命令
  130  被中断（SIGINT / 上下文取消）
  其他 透传子进程退出码：am run 用编排工具的退出码，am scene run-all 用镜头汇总退出码

所有错误写 stderr，正常输出写 stdout；失败时不打印 usage，只打印一行错误。`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().BoolVar(&unsafe, "unsafe", false,
		"关闭 AI CLI 的权限隔离（危险）。只对会启动 AI CLI 的 run / scene run / scene run-all 生效，"+
			"对 init、config、validate、archive 无作用；会传递给嵌套镜头任务")

	var skipHyperframes bool
	var canvasID string
	initCmd := &cobra.Command{
		Use:   "init [DIR]",
		Short: "初始化一个可复现的视频项目",
		Long: `在 DIR（省略则为当前目录）写入视频项目骨架。

画幅在初始化时一次性选定并写入 frame.md，之后不再更改；改画幅意味着重建项目。
可选值 vertical-3x4（1080x1440）与 vertical-9x16（1080x1920），均 30fps。
非交互环境（CI、管道）必须显式传 --canvas，不会静默取默认值；不传且无法交互时报错退出。

写入内容：
  PROMPT.md、PROMPT-PRODUCTION.md、article-to-motion.conf、.env.example
  frame.md、docs/ 风格说明书、assets/fonts/ 中文字体、assets/style-guide/ 示例
  templates/project-rules.md（am run 据此派生 AGENTS.md / CLAUDE.md 等）
  .agents/skills/ 内置技能树（text-to-lottie、algorithmic-art），随二进制下发

不覆盖内容不同的已有文件：目标已存在且字节不同就整体失败并回滚，不做合并。
字节相同则视为已存在并跳过，因此对同一画幅重复执行是幂等的，可安全用于补齐缺失文件。

默认联网执行 npx --yes hyperframes@` + hyperframesVersion + ` skills 安装官方动效技能；
离线或 CI 用 --skip-hyperframes 跳过。该开关不影响内置技能树，它随二进制下发。`,
		Example: `  # 交互选择画幅
  am init my-video

  # 非交互环境必须显式指定画幅
  am init my-video --canvas vertical-9x16

  # 离线或 CI：跳过联网安装 HyperFrames 技能
  am init my-video --canvas vertical-3x4 --skip-hyperframes

  # 在当前目录补齐缺失的骨架文件（幂等）
  am init --canvas vertical-3x4`,
		Args: maxArgs(1, "DIR"),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			target, _ = filepath.Abs(target)
			chosen, err := resolveCanvas(canvasID, os.Stdin, stdout)
			if err != nil {
				return err
			}
			shared, err := assets.Shared()
			if err != nil {
				return err
			}
			presetFiles, err := assets.Preset(chosen.ID)
			if err != nil {
				return err
			}
			result, err := project.Initialize(target, shared, presetFiles)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "项目已初始化：%s（画幅 %s，新增 %d，未变 %d）\n", target, chosen.Label, result.Created, result.Unchanged)
			if skipHyperframes {
				fmt.Fprintln(stdout, "警告：已跳过 HyperFrames 技能安装")
				return nil
			}
			pinned := "hyperframes@" + hyperframesVersion
			npx := exec.Command("npx", "--yes", pinned, "skills")
			npx.Dir, npx.Stdout, npx.Stderr = target, stdout, stderr
			if err := npx.Run(); err != nil {
				return fmt.Errorf("项目文件已写入，但 HyperFrames 技能安装失败；可在项目目录重试 npx --yes %s skills: %w", pinned, err)
			}
			return nil
		},
	}
	initCmd.Flags().BoolVar(&skipHyperframes, "skip-hyperframes", false, "跳过联网安装 HyperFrames 官方技能；不影响随二进制下发的内置技能树")
	initCmd.Flags().StringVar(&canvasID, "canvas", "", "画幅预设："+strings.Join(preset.IDs(), " | ")+"；不传则在终端里交互选择")
	root.AddCommand(initCmd)

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "读取解析后的配置",
		Long: `读取 am 解析后的配置值。

不要自己去读 article-to-motion.conf 或 .env——解析涉及四层优先级
（环境变量 > 项目 .env > article-to-motion.conf > 内置默认），只有本命令的
输出等于 am 实际会用的值。`,
	}
	configCmd.AddCommand(&cobra.Command{
		Use:   "get ORCHESTRATOR|RENDERER|TTS_PROVIDER",
		Args:  exactArgs(1, "KEY"),
		Short: "打印单个配置键",
		Long: `把单个配置键的解析结果打印到 stdout，只有值本身加一个换行，便于直接捕获。

可用键：
  ORCHESTRATOR   编排工具：codex | claude | qoder | codebuddy | opencode
  RENDERER       渲染工具：同上取值范围
  TTS_PROVIDER   语音合成：minimax | bailian

在当前工作目录解析配置，因此要在项目根运行。键名未知或解析出的值非法时退出码 1。`,
		Example: `  # 捕获解析后的渲染工具
  renderer="$(am config get RENDERER)"

  # 环境变量优先级最高，可临时覆盖
  RENDERER=codex am config get RENDERER`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(".", nil)
			if err != nil {
				return err
			}
			values := map[string]string{"ORCHESTRATOR": cfg.Orchestrator, "RENDERER": cfg.Renderer, "TTS_PROVIDER": cfg.TTSProvider}
			value, ok := values[args[0]]
			if !ok {
				return fmt.Errorf("未知配置键：%s", args[0])
			}
			fmt.Fprintln(stdout, value)
			return nil
		},
	})
	root.AddCommand(configCmd)

	sceneCmd := &cobra.Command{
		Use:   "scene",
		Short: "镜头相关操作",
		Long: `渲染镜头。可以在 am run 的编排流程里被调用，也可以完全独立使用。

一个镜头是一个目录，至少包含：
  scene.json   执行契约，字段见下
  prompt.md    本镜头的创意方向（可选，缺失时使用内置的通用创意方向）
  transcript   完整字幕，供渲染工具理解上下文

scene.json 只接受五个必填字段 id、duration_seconds、output、transcript、text，
以及两个可选字段 style_guide、renderer。出现任何未知字段直接失败。
duration_seconds 保留毫秒精度写成小数秒（如 2.833），不得取整。
text 不得含 [[USER_MESSAGE]] 或 <scene-text> 定界标记，含则失败。

渲染工具由 scene.json 的 renderer 决定，未声明则取配置的 RENDERER；
解析不到直接报错，不存在静默回退。

渲染输出中只有以 [[USER_MESSAGE]] 开头的行会转给用户，其余进日志文件：
  render-<镜头编号>.stream.jsonl / .stderr.log / .user.log`,
	}
	var tolerance float64
	runScene := &cobra.Command{
		Use: "run DIRECTORY", Args: exactArgs(1, "DIRECTORY"), Short: "执行单个镜头",
		Long: `渲染 DIRECTORY 这一个镜头，产出 scene.json 里 output 指定的 MP4。

无条件重新渲染：不做已有产物检查，也没有 --force。需要跳过已完成镜头时用 run-all。

渲染完成后校验产物时长是否落在 duration_seconds ± --duration-tolerance 内，
超出则失败。容差必须是有限非负数。

启动前解析技能目录并把绝对路径写进渲染提示词。动效技能未找到时打印警告并降级为
按技能名引用，不中断渲染——警告写 stderr，成片动效质量会下降，值得处理。`,
		Example: `  am scene run scenes/scene-001

  # 放宽产物时长容差到 0.3 秒
  am scene run scenes/scene-001 --duration-tolerance 0.3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTolerance(tolerance); err != nil {
				return err
			}
			cfg, err := config.Load(".", nil)
			if err != nil {
				return err
			}
			s, err := scene.Load(args[0])
			if err != nil {
				return err
			}
			return scene.Run(cmd.Context(), s, cfg, envutil.IsUnsafe(unsafe), currentEnvironment(), stdout, tolerance)
		},
	}
	runScene.Flags().Float64Var(&tolerance, "duration-tolerance", 0.15, "产物时长与 duration_seconds 的允许偏差（秒），有限非负数")
	sceneCmd.AddCommand(runScene)

	var jobs, retries int
	var force bool
	var reportPath, srtPath string
	var strictCoverage bool
	var coverageTolerance float64
	runAll := &cobra.Command{
		Use: "run-all ROOT", Args: exactArgs(1, "ROOT"), Short: "并行执行多个镜头",
		Long: `并发渲染 ROOT 下的所有镜头。镜头之间无创作依赖，跨镜头视觉一致性由 frame.md 保证。

每个镜头开跑前先判定状态，判定结果直接决定要不要重渲染：
  已有合格产物   产物存在且通过时长校验 → 跳过
  Stale         scene.json 或 prompt.md 的修改时间晚于产物 → 不自动重渲染，
                标记为需要显式 --force，避免悄悄覆盖人工确认过的成片
  其余           渲染

--force 跳过全部上述判定，无条件重渲染每个镜头。

--jobs 取值 1..16，缺省读配置 SCENE_JOBS（其自身默认为 3）。--retries 取值 0..5。
越界直接失败，不做截断。

重试只针对渲染器非零退出，退避 10 秒与 30 秒。产物规格校验失败不重试：
同样的提示词和渲染器重跑只会得到同样不合规的产物，而每次重试都是一次完整的
AI CLI 调用。这类失败要先改提示词或时长声明再重跑。

渲染开始前先做覆盖校验：镜头编号是否唯一，以及给了 --srt 时，镜头总时长是否
等于字幕跨度（容差 --coverage-tolerance，默认 0.1 秒）。默认只警告，
--strict-coverage 让它在渲染前直接失败——漏掉一镜要等全部渲染完拼接时才发现，
那时每一镜都已经烧掉一次完整的 AI CLI 调用。

被中断（SIGINT）时停止派发新任务、终止在飞进程组，并写出部分报告。

退出码在有镜头失败时非 0，由汇总报告决定；此时 --report-json 写出的文件仍然完整，
应当读它来判断是哪些镜头失败，而不是解析 stdout 的人类可读表格。`,
		Example: `  am scene run-all scenes/

  # 并发 3、失败重试 2 次，并写出机器可读报告
  am scene run-all scenes/ --jobs 3 --retries 2 --report-json production/run-report.json

  # 渲染前先核对镜头拆分是否完整覆盖字幕，不通过就不开工
  am scene run-all scenes/ --srt transcription.srt --strict-coverage

  # 输入改过、镜头被判为 Stale 时，显式重渲染
  am scene run-all scenes/ --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTolerance(tolerance); err != nil {
				return err
			}
			cfg, err := config.Load(".", nil)
			if err != nil {
				return err
			}
			if jobs == 0 {
				jobs = cfg.SceneJobs
			}
			if jobs < 1 || jobs > 16 || retries < 0 || retries > 5 {
				return fmt.Errorf("--jobs 必须为 1..16，--retries 必须为 0..5")
			}
			scenes, err := schedule.Plan(args[0])
			if err != nil {
				return err
			}
			// 覆盖校验在渲染之前跑：漏掉一镜或某镜时长写错，等全部渲染完
			// 拼接时才发现的话，每一镜都已经烧掉一次完整的 AI CLI 调用。
			span := 0.0
			if srtPath != "" {
				parsed, err := srt.ReadSpan(srtPath)
				if err != nil {
					return err
				}
				span = parsed.Seconds()
			}
			if problems := schedule.CoverageProblems(scenes, span, coverageTolerance); len(problems) > 0 {
				message := fmt.Sprintf("镜头覆盖校验未通过：\n  - %s", strings.Join(problems, "\n  - "))
				if strictCoverage {
					return errors.New(message)
				}
				fmt.Fprintf(stderr, "警告：%s\n用 --strict-coverage 让这类问题在渲染前直接失败。\n", message)
			}
			initial := make(map[string]error)
			if !force {
				env := currentEnvironment()
				for _, item := range scenes {
					if _, statErr := os.Stat(item.OutputPath()); statErr != nil {
						continue
					}
					if verifyErr := scene.VerifyOutput(item, env, tolerance); verifyErr != nil {
						continue
					}
					outputInfo, statErr := os.Stat(item.OutputPath())
					if statErr != nil {
						continue
					}
					status := schedule.Skipped
					reason := "已有合格产物"
					for _, input := range []string{filepath.Join(item.Directory, "scene.json"), filepath.Join(item.Directory, "prompt.md")} {
						if info, statErr := os.Stat(input); statErr == nil && info.ModTime().After(outputInfo.ModTime()) {
							status, reason = schedule.Stale, "输入比产物新，需要显式 --force 重渲染"
							break
						}
					}
					initial[item.ID] = schedule.Outcome(status, reason)
				}
			}
			report := schedule.RunAll(cmd.Context(), scenes, jobs, retries, func(ctx context.Context, s scene.Scene) error {
				if outcome, ok := initial[s.ID]; ok {
					return outcome
				}
				err := scene.Run(ctx, s, cfg, envutil.IsUnsafe(unsafe), currentEnvironment(), stdout, tolerance)
				// 产物规格不符是确定性的：同样的提示词和渲染器重跑只会得到同样的产物。
				// 不在这里截断的话，每个规格不符的镜头都要白烧 retries 次 AI CLI 调用。
				var verification *scene.VerificationError
				if errors.As(err, &verification) {
					return schedule.Fatal(err)
				}
				return err
			})
			fmt.Fprintln(stdout, report.Render())
			if reportPath != "" {
				if err := report.WriteJSON(reportPath); err != nil {
					return err
				}
			}
			if report.ExitCode() != 0 {
				return &exitError{code: report.ExitCode(), message: "镜头执行未全部成功"}
			}
			return nil
		},
	}
	runAll.Flags().IntVar(&jobs, "jobs", 0, "并发上限，取值 1..16；缺省读配置 SCENE_JOBS，其自身默认为 3")
	runAll.Flags().IntVar(&retries, "retries", 2, "单个镜头失败后的最大重试次数，取值 0..5")
	runAll.Flags().Float64Var(&tolerance, "duration-tolerance", 0.15, "产物时长与 duration_seconds 的允许偏差（秒），有限非负数")
	runAll.Flags().BoolVar(&force, "force", false, "跳过「已有合格产物」与 Stale 判定，无条件重渲染每个镜头")
	runAll.Flags().StringVar(&reportPath, "report-json", "", "把逐镜头结果写成 JSON 报告到该路径；镜头失败时应读它而不是解析 stdout")
	runAll.Flags().StringVar(&srtPath, "srt", "", "定稿字幕路径；给了就核对镜头总时长是否等于字幕跨度")
	runAll.Flags().Float64Var(&coverageTolerance, "coverage-tolerance", 0.1, "镜头总时长与字幕跨度的允许偏差（秒）")
	runAll.Flags().BoolVar(&strictCoverage, "strict-coverage", false, "覆盖校验不通过时直接失败，而不是只打印警告")
	sceneCmd.AddCommand(runAll)
	sceneCmd.AddCommand(newSceneFramesCommand(stdout))
	root.AddCommand(sceneCmd)

	var workdir string
	runCmd := &cobra.Command{Use: "run [PROMPT]", Short: "按配置启动编排工具",
		Long: `把 prompt 文件交给配置的编排工具执行，由它驱动整条制作流程。

PROMPT 省略时读项目根的 PROMPT.md——该入口假定项目里已有定稿 transcription.srt。
传入 PROMPT-PRODUCTION.md 则从文章、口播稿或参考字幕开始完整制作。
两者都是随 am init 下发的内置 prompt，可按项目改写。找不到该文件时失败。

启动前会做三件事：
  1. 按 ORCHESTRATOR 解析出该工具会自动读取的项目级指令文件名
     （codex/qoder/opencode 读 AGENTS.md，claude 读 CLAUDE.md，codebuddy 读 CODEBUDDY.md），
     再从 templates/project-rules.md 派生并写到项目根。模板缺失时只打印警告继续，
     本次不下发项目规则；目标文件已存在且内容不同时失败，不覆盖。
  2. 把当前 am 可执行文件所在目录放到子进程 PATH 首位，并注入绝对路径为 AM_EXECUTABLE，
     因此 prompt 里的 am scene ... 会用回启动本次流程的同一个 CLI，无需把二进制拷进项目。
  3. 安全模式下按白名单构造子进程环境；额外变量用 AM_PASSTHROUGH_ENV 列出。

启动后先打印一段以 --- 结尾的头部（编排工具、渲染工具、Prompt 路径、项目规则路径），
其后全部是编排工具自身的输出。编排工具失败时透传它的退出码。`,
		Example: `  # 项目已有定稿 transcription.srt
  am run

  # 从文章或口播稿开始完整制作
  am run PROMPT-PRODUCTION.md

  # 只有确认项目与输入可信时才关闭权限隔离
  am --unsafe run

  # 安全模式下额外透传两个环境变量
  AM_PASSTHROUGH_ENV=HTTPS_PROXY,NO_PROXY am run`,
		Args: maxArgs(1, "PROMPT"), RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, _ := os.Getwd()
			cfg, err := config.Load(rootDir, nil)
			if err != nil {
				return err
			}
			if workdir == "" {
				workdir = rootDir
			}
			workdir, _ = filepath.Abs(workdir)
			promptPath := filepath.Join(rootDir, "PROMPT.md")
			if len(args) == 1 {
				promptPath, _ = filepath.Abs(args[0])
			}
			prompt, err := os.ReadFile(promptPath)
			if err != nil {
				return fmt.Errorf("找不到 prompt 文件：%s", promptPath)
			}
			rulesName, err := tools.ProjectRulesFilename(cfg.Orchestrator)
			if err != nil {
				return err
			}
			rulesPath, err := project.WriteRules(rootDir, rulesName)
			if err != nil {
				return err
			}
			isUnsafe := envutil.IsUnsafe(unsafe)
			argv, stdin, err := tools.OrchestratorInvocation(cfg.Orchestrator, workdir, string(prompt), isUnsafe)
			if err != nil {
				return err
			}
			binary, err := exec.LookPath(argv[0])
			if err != nil {
				return fmt.Errorf("缺少必需工具：%s", argv[0])
			}
			process := exec.Command(binary, argv[1:]...)
			process.Dir, process.Stdout, process.Stderr = workdir, stdout, stderr
			childEnvironment := cfg.ChildEnvironment(currentEnvironment(), isUnsafe, envutil.ParsePassthrough(os.Getenv("AM_PASSTHROUGH_ENV")))
			addExecutableLocation(childEnvironment)
			process.Env = envList(childEnvironment)
			if stdin != "" {
				process.Stdin = strings.NewReader(stdin)
			}
			if rulesPath == "" {
				fmt.Fprintf(stderr, "警告：项目缺少 %s，本次不下发项目规则；可运行 am init 补齐\n", project.RulesTemplate)
			}
			fmt.Fprintf(stdout, "编排工具: %s\n渲染工具: %s\nPrompt: %s\n项目规则: %s\n---\n", cfg.Orchestrator, cfg.Renderer, promptPath, rulesDisplay(rulesPath))
			if err := runProcessGroup(cmd.Context(), process); err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					return &exitError{code: exit.ExitCode(), message: "编排工具执行失败"}
				}
				return err
			}
			return nil
		}}
	runCmd.Flags().StringVar(&workdir, "workdir", "", "编排工具的工作目录，缺省为当前项目根")
	root.AddCommand(runCmd)

	var mainRef, archiveRoot, archiveName string
	var yes, dryRun bool
	archiveCmd := &cobra.Command{Use: "archive", Short: "归档当前项目并复位到 main",
		Long: `把本片的单片文件连同 SHA-256 清单移出仓库，并把工作区复位到冻结的 main 提交。

破坏性操作，且不属于成片验收的一部分。未经用户明确确认不要执行：
先跑 --dry-run 看计划，把结果原样汇报给用户，由用户决定是否继续。

会把相对 main 的改动分成三类，只有第一类会被归档：
  候选文件   main 提交里不存在的新增文件，以及全部未跟踪文件 → 移入归档目录
  共享修改   main 提交里已存在却被改动的文件 → 视为阻断
  ignored   被 .gitignore 忽略但存在于工作区的文件 → 视为阻断

存在共享修改或 ignored 文件时拒绝执行并报告。不要为了让它通过而删文件、移文件、
改 .gitignore 或重置工作区——那会掩盖问题。也不要删除项目分支或改写 Git 历史。

--archive-name 缺省为 project-<年月日-时分秒>。成功后工作区 detach 到 --main-ref
指向的提交。除非传 --yes，否则会在 stdin 上交互确认；非交互环境下没有 --yes 会取消。`,
		Example: `  # 先看计划，不做任何改动
  am archive --dry-run

  # 用户确认后执行
  am archive

  # 非交互环境必须显式跳过确认
  am archive --yes --archive-root ~/video-archive --archive-name my-video-2026-08`,
		Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
			if archiveName == "" {
				archiveName = "project-" + time.Now().Format("20060102-150405")
			}
			plan, err := archive.BuildPlan(".", mainRef, archiveRoot, archiveName)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "归档目标：%s\n候选文件：%d\n共享修改：%d\nignored：%d\n", plan.Destination, len(plan.Candidates), len(plan.Shared), len(plan.Ignored))
			if dryRun {
				return nil
			}
			if !yes {
				fmt.Fprint(stdout, "确认归档并复位到 main？[y/N] ")
				answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.ToLower(strings.TrimSpace(answer)) != "y" {
					return fmt.Errorf("用户取消归档")
				}
			}
			return archive.Execute(plan)
		}}
	archiveCmd.Flags().StringVar(&mainRef, "main-ref", "main", "冻结的 main ref，归档后工作区 detach 到它指向的提交")
	archiveCmd.Flags().StringVar(&archiveRoot, "archive-root", "", "归档父目录，须在仓库之外")
	archiveCmd.Flags().StringVar(&archiveName, "archive-name", "", "归档目录名，缺省为 project-<年月日-时分秒>")
	archiveCmd.Flags().BoolVar(&yes, "yes", false, "跳过交互确认；非交互环境执行归档时必须传")
	archiveCmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印计划，不移动任何文件、不改动工作区")
	root.AddCommand(archiveCmd)

	validateCmd := &cobra.Command{Use: "validate", Short: "校验发布配置与风格规范",
		Long: `只读校验，不修改任何项目文件（唯一例外是 style --regenerate-examples）。
校验通过时打印一行结论并退出 0，失败时把具体不符项写到 stderr 并退出 1。`,
	}
	var projectRoot string
	var regenerateExamples bool
	publishCmd := &cobra.Command{Use: "publish PATH", Args: exactArgs(1, "PATH"), Short: "校验 publish.md",
		Long: `校验 PATH 指向的发布配置。

publish.md 必须以安全 YAML frontmatter 提供机器字段，并以固定 Markdown 章节
提供人工可复制内容。校验会核对字段完整性，以及它引用的产物是否与项目实际一致，
因此需要 --project-root 定位项目（缺省为当前目录）。

通过时打印 publish 校验通过：<路径>（<状态>）。`,
		Example: `  am validate publish publish.md --project-root .`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir := projectRoot
			if rootDir == "" {
				rootDir = "."
			}
			data, err := validate.Publish(args[0], rootDir)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "publish 校验通过：%s（%v）\n", args[0], data["publish_status"])
			return nil
		}}
	publishCmd.Flags().StringVar(&projectRoot, "project-root", "", "项目根目录，缺省为当前目录")
	validateCmd.AddCommand(publishCmd)
	styleCmd := &cobra.Command{Use: "style", Args: noArgs, Short: "校验风格规范",
		Long: `校验项目的视觉规范：画布规格、配色、字体层级、安全区，以及字体文件是否齐备。

字体检查是重点：渲染机是一台干净的无头 Chrome，不装任何系统字体。字体栈里没有
@font-face 的字体族会静默回退——本地看着正常、成片排版却是错的。HyperFrames 会
自动内联一批字体，但其中没有任何简体中文字体，所以中文字形必须由项目自带文件提供。

--regenerate-examples 会重新渲染 assets/style-guide/examples/ 下的示例 PNG，
这是本命令唯一会写文件的路径。它需要两个外部命令，缺任意一个都会失败：
  rsvg-convert   来自 librsvg，负责 SVG 转 PNG
  magick         来自 ImageMagick，负责把多张 PNG 拼成概览图

SVG 转 PNG 这一步不能用 ImageMagick 代替：多数 ImageMagick 构建自带内置 XML SVG
渲染器，会抢在 rsvg 委托之前接管 .svg，产出黑底无字的图却返回退出码 0。`,
		Example: `  am validate style --project-root .

  # 重新生成示例 PNG，需要 rsvg-convert 与 magick 都可用
  am validate style --project-root . --regenerate-examples`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir := projectRoot
			if rootDir == "" {
				rootDir = "."
			}
			if err := validate.Style(rootDir); err != nil {
				return err
			}
			if regenerateExamples {
				if err := validate.RegenerateExamples(rootDir, stdout); err != nil {
					return err
				}
			}
			fmt.Fprintln(stdout, "风格规范校验通过")
			return nil
		}}
	styleCmd.Flags().StringVar(&projectRoot, "project-root", "", "项目根目录，缺省为当前目录")
	styleCmd.Flags().BoolVar(&regenerateExamples, "regenerate-examples", false, "重新生成通用风格示例 PNG，需要 rsvg-convert（librsvg）与 magick（ImageMagick）两个命令")
	validateCmd.AddCommand(styleCmd)
	validateCmd.AddCommand(newValidateVideoCommand(stdout, &projectRoot))
	root.AddCommand(validateCmd)
	return root
}

// parseTimePoints 解析 --at 的时间点列表，支持秒数、百分比和 end。
//
// 百分比和 end 都要先知道时长，所以 duration 必须是实测值而不是 scene.json 的
// 声明值：产物时长和声明时长不一致正是要靠抽帧去看的情况之一。
//
// end 取 duration - 一帧，而不是 duration：后者已经超出最后一帧，
// ffmpeg 会返回成功但什么都不输出。
func parseTimePoints(spec string, duration float64, fps int) ([]float64, error) {
	if fps <= 0 {
		fps = 30
	}
	lastFrame := duration - 1.0/float64(fps)
	if lastFrame < 0 {
		lastFrame = 0
	}
	var points []float64
	for _, raw := range strings.Split(spec, ",") {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		var value float64
		switch {
		case strings.EqualFold(item, "end"):
			value = lastFrame
		case strings.HasSuffix(item, "%"):
			percent, err := strconv.ParseFloat(strings.TrimSuffix(item, "%"), 64)
			if err != nil || percent < 0 || percent > 100 {
				return nil, fmt.Errorf("--at 的百分比必须是 0..100，实际收到 %q", item)
			}
			value = duration * percent / 100
			if value > lastFrame {
				value = lastFrame
			}
		default:
			parsed, err := strconv.ParseFloat(item, 64)
			if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
				return nil, fmt.Errorf("--at 的时间点必须是非负秒数、百分比或 end，实际收到 %q", item)
			}
			if parsed > lastFrame {
				return nil, fmt.Errorf("--at 的时间点 %s 秒超出视频时长 %.3f 秒", item, duration)
			}
			value = parsed
		}
		points = append(points, value)
	}
	if len(points) == 0 {
		return nil, fmt.Errorf("--at 至少要给一个时间点")
	}
	return points, nil
}

// parseCanvas 解析 "1080x1440" 形式的画幅覆盖值。
func parseCanvas(value string) (int, int, error) {
	width, height, ok := strings.Cut(strings.ToLower(strings.TrimSpace(value)), "x")
	if !ok {
		return 0, 0, fmt.Errorf("--canvas 必须写成 宽x高，例如 1080x1440，实际收到 %q", value)
	}
	w, errW := strconv.Atoi(strings.TrimSpace(width))
	h, errH := strconv.Atoi(strings.TrimSpace(height))
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("--canvas 的宽高必须是正整数，实际收到 %q", value)
	}
	return w, h, nil
}

func newSceneFramesCommand(stdout io.Writer) *cobra.Command {
	var at, outDir, reportPath string
	var checkBlank bool

	cmd := &cobra.Command{
		Use: "frames DIRECTORY", Args: exactArgs(1, "DIRECTORY"), Short: "从镜头产物抽帧供视觉检查",
		Long: `把 DIRECTORY 这一镜的产物按若干时间点抽成 PNG，用于视觉验收。

这个命令不判断画面对不对——那只有能看图的一方说得清。它的职责是把「渲染完的
mp4」变成「可以逐张打开看的 PNG」，并把机器能判的情况标出来。

渲染成功不等于画面正确：文字压在图形上看不清、中文字体静默回退成方框、
动画在第 30 帧就停住、转场只做了一半——这些今天全都能拿到退出码 0。
抽帧是把这类问题从「成片时才发现」提前到「单镜头渲完就发现」的唯一手段。

--at 接受三种写法，可混用：
  秒数     0、1.5、2.833
  百分比   25%、50%——按实测时长换算，不是 scene.json 的声明时长
  end      最后一帧。取时长减一帧，直接用时长会超出末帧，
           ffmpeg 会返回成功却什么都不输出

--check-blank 对每一帧做纯色判定，命中即退出 1。判定阈值刻意保守，只拦几乎
完全均匀的画面：深色背景配浅色标题是常见设计，误判它比漏判一个真空白帧更糟。
偏暗但有内容的帧只进报告的 hints，不影响退出码。

产物默认写到 <DIRECTORY>/visual-qc/，与 PROMPT 第七阶段的 production/visual-qc/
是同一用途。逐帧统计量写在 --report-json 里，判断结果请读它。`,
		Example: `  # 默认在首、四分位和末帧各抽一张
  am scene frames scenes/scene-001

  # 只看第 0 帧封面，并拦下空白帧
  am scene frames scenes/scene-001 --at 0 --check-blank

  # 检查转场区间，写到项目统一的视觉验收目录
  am scene frames scenes/scene-001 --at 0,0.5,1.2,end \
    --out production/visual-qc/scene-001 \
    --report-json production/visual-qc/scene-001.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := scene.Load(args[0])
			if err != nil {
				return err
			}
			canvas, err := scene.CanvasOf(s.Directory, s.StyleGuide)
			if err != nil {
				return err
			}
			toolchain, err := mediaprobe.New(currentEnvironment())
			if err != nil {
				return err
			}
			// 时间点按实测时长换算：产物时长与声明时长不一致，
			// 正是要靠抽帧去看的情况之一。
			media, err := toolchain.Probe(s.OutputPath())
			if err != nil {
				return err
			}
			points, err := parseTimePoints(at, media.DurationSeconds, canvas.FPS)
			if err != nil {
				return err
			}
			destination := outDir
			if destination == "" {
				destination = filepath.Join(s.Directory, "visual-qc")
			}
			report, err := toolchain.ExtractFrames(s.OutputPath(), points, destination)
			if err != nil {
				return err
			}
			if reportPath != "" {
				if err := report.WriteJSON(reportPath); err != nil {
					return err
				}
			}
			for _, frame := range report.Frames {
				fmt.Fprintf(stdout, "%.3f 秒  %s  灰度均值 %.1f  标准差 %.2f\n",
					frame.AtSeconds, frame.Image, frame.Stats.Mean, frame.Stats.StdDev)
			}
			for _, hint := range report.Hints {
				fmt.Fprintf(stdout, "提示：%s\n", hint)
			}
			if checkBlank && !report.OK {
				return fmt.Errorf("抽帧发现空白帧：%s\n  - %s",
					s.Output, strings.Join(report.Problems, "\n  - "))
			}
			return nil
		}}
	cmd.Flags().StringVar(&at, "at", "0,25%,50%,75%,end", "抽帧时间点，逗号分隔；支持秒数、百分比与 end")
	cmd.Flags().StringVar(&outDir, "out", "", "PNG 输出目录，缺省为 <DIRECTORY>/visual-qc")
	cmd.Flags().BoolVar(&checkBlank, "check-blank", false, "发现空白帧时退出码 1；不传则只抽帧和记录，不改变退出码")
	cmd.Flags().StringVar(&reportPath, "report-json", "", "把逐帧统计写成 JSON 报告到该路径")
	return cmd
}

func newValidateVideoCommand(stdout io.Writer, projectRoot *string) *cobra.Command {
	var canvasOverride string
	var fps, expectFrames, sampleRate, channels int
	var codec, pixelFormat, reportPath string
	var duration, tolerance float64
	var silent, withAudio, decode, checkFrameZero bool

	cmd := &cobra.Command{
		Use: "video PATH", Args: exactArgs(1, "PATH"), Short: "校验视频产物规格",
		Long: `按机器可判定的规格校验 PATH 指向的视频，用于静音母版和成片验收。

检查项与不给的默认值：
  分辨率        取项目 frame.md 的 canvas；--canvas 1080x1920 可覆盖
  帧率          同上取 canvas.fps；--fps 可覆盖
  视频编码      默认 h264，--codec 可覆盖，--codec "" 关闭该项检查
  像素格式      默认 yuv420p，--pixel-format 同上
  时长/帧数/音轨 默认不检查，由对应参数开启

不符项会一次全部列出，不是报第一条就停——逐条修再重跑一轮代价太高。

三个开关代价明显更高，默认关闭：
  --expect-frames  精确统计帧数，完整解码一遍。容器声明的 nb_frames 可能缺失
                   或不准，要断言「总帧数等于冻结时间轴」时必须用它。
  --decode         完整解码一遍，发现截断和损坏。仅读流头的检查发现不了：
                   moov 在文件头时，截断一半的文件照样能读出正确规格。
  --check-frame-zero  检查第 0 帧是否空白帧，对应 frame.md 的
                   black_or_blank_frame_zero 禁令。抖音类平台拿第 0 帧做封面。

退出码 0 表示全部通过，1 表示有不符项或检查跑不起来，127 表示缺 ffprobe/ffmpeg。
「检查跑不起来」（文件读不了、工具缺失）和「检查跑了但不合格」在 --report-json
里是能区分的：前者不产出报告文件，后者产出 ok=false 的完整报告。

判断结果请读 --report-json 写出的文件，不要解析 stdout 的中文输出。`,
		Example: `  # 静音母版：画幅取自 frame.md，断言无音轨且能完整解码
  am validate video production/silent-master.mp4 --silent --decode

  # 成片：断言总帧数、48kHz 双声道，并检查第 0 帧封面
  am validate video final.mp4 --expect-frames 8340 --audio \
    --check-frame-zero --report-json production/final-check.json

  # 脱离项目使用，显式给画幅
  am validate video clip.mp4 --canvas 1080x1920 --fps 30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if silent && withAudio {
				return fmt.Errorf("--silent 与 --audio 互斥：一个断言没有音轨，一个断言有")
			}
			rootDir := *projectRoot
			if rootDir == "" {
				rootDir = "."
			}
			spec := mediaprobe.Spec{
				Codec:           codec,
				PixelFormat:     pixelFormat,
				DurationSeconds: duration,
				Tolerance:       tolerance,
				Frames:          expectFrames,
			}
			if canvasOverride != "" {
				width, height, err := parseCanvas(canvasOverride)
				if err != nil {
					return err
				}
				spec.WidthPx, spec.HeightPx = width, height
				spec.FPS = fps
			} else {
				canvas, err := scene.CanvasOf(rootDir, "frame.md")
				if err != nil {
					return fmt.Errorf("无法从 %s 的 frame.md 确定画幅：%w；"+
						"脱离项目使用时请显式传 --canvas 宽x高 与 --fps", rootDir, err)
				}
				spec.WidthPx, spec.HeightPx, spec.FPS = canvas.WidthPx, canvas.HeightPx, canvas.FPS
				if fps > 0 {
					spec.FPS = fps
				}
			}
			switch {
			case silent:
				spec.Audio = mediaprobe.AudioAbsent
			case withAudio:
				spec.Audio = mediaprobe.AudioPresent
				spec.SampleRate, spec.Channels = sampleRate, channels
			}
			toolchain, err := mediaprobe.New(currentEnvironment())
			if err != nil {
				return err
			}
			report, err := toolchain.Verify(args[0], mediaprobe.VerifyOptions{
				Spec:           spec,
				ExactFrames:    expectFrames > 0,
				Decode:         decode,
				CheckFrameZero: checkFrameZero,
			})
			if err != nil {
				return err
			}
			if reportPath != "" {
				if err := report.WriteJSON(reportPath); err != nil {
					return err
				}
			}
			for _, hint := range report.Hints {
				fmt.Fprintf(stdout, "提示：%s\n", hint)
			}
			if !report.OK {
				return fmt.Errorf("视频规格校验未通过：%s\n  - %s",
					args[0], strings.Join(report.Problems, "\n  - "))
			}
			fmt.Fprintf(stdout, "视频规格校验通过：%s\n", args[0])
			return nil
		}}
	cmd.Flags().StringVar(projectRoot, "project-root", "", "项目根目录，缺省为当前目录；用于从 frame.md 反查画幅")
	cmd.Flags().StringVar(&canvasOverride, "canvas", "", "画幅覆盖值，写成 宽x高（如 1080x1920）；不传则读项目 frame.md")
	cmd.Flags().IntVar(&fps, "fps", 0, "帧率覆盖值；不传则读 frame.md 的 canvas.fps")
	cmd.Flags().StringVar(&codec, "codec", "h264", `期望的视频编码；传空串关闭该项检查`)
	cmd.Flags().StringVar(&pixelFormat, "pixel-format", "yuv420p", `期望的像素格式；传空串关闭该项检查`)
	cmd.Flags().Float64Var(&duration, "duration", 0, "期望时长（秒），0 表示不检查")
	cmd.Flags().Float64Var(&tolerance, "duration-tolerance", 0.15, "时长允许偏差（秒），有限非负数")
	cmd.Flags().IntVar(&expectFrames, "expect-frames", 0, "期望总帧数，0 表示不检查；开启后会完整解码一遍精确统计")
	cmd.Flags().BoolVar(&silent, "silent", false, "断言没有音轨，用于镜头产物和静音母版")
	cmd.Flags().BoolVar(&withAudio, "audio", false, "断言有音轨，用于混音后的成片；与 --silent 互斥")
	cmd.Flags().IntVar(&sampleRate, "sample-rate", 48000, "配合 --audio 断言采样率，0 表示不检查")
	cmd.Flags().IntVar(&channels, "channels", 2, "配合 --audio 断言声道数，0 表示不检查")
	cmd.Flags().BoolVar(&decode, "decode", false, "完整解码一遍确认没有截断或损坏；代价与文件长度成正比")
	cmd.Flags().BoolVar(&checkFrameZero, "check-frame-zero", false, "检查第 0 帧是否空白帧，交付封面前必查")
	cmd.Flags().StringVar(&reportPath, "report-json", "", "把校验结果写成 JSON 报告到该路径；判断结果应读它而不是解析 stdout")
	return cmd
}

func rulesDisplay(path string) string {
	if path == "" {
		return "（缺模板，未下发）"
	}
	return path
}

// exactArgs 是 cobra.ExactArgs 的中文版：默认信息是 "accepts 1 arg(s), received 0"，
// 既与全中文界面不一致，也没说清缺的是哪个参数。调用方要能只读这一行就知道补什么。
func exactArgs(count int, name string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == count {
			return nil
		}
		return fmt.Errorf("%s 需要 %d 个位置参数 %s，实际收到 %d 个；用法：%s",
			cmd.CommandPath(), count, name, len(args), cmd.UseLine())
	}
}

// maxArgs 同上，用于可选位置参数。
func maxArgs(count int, name string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) <= count {
			return nil
		}
		return fmt.Errorf("%s 最多接受 %d 个位置参数 %s，实际收到 %d 个；用法：%s",
			cmd.CommandPath(), count, name, len(args), cmd.UseLine())
	}
}

// noArgs 同上，用于不接受位置参数的命令。
func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf("%s 不接受位置参数，实际收到 %d 个：%s；用法：%s",
		cmd.CommandPath(), len(args), strings.Join(args, " "), cmd.UseLine())
}

func validateTolerance(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("--duration-tolerance 必须是有限且非负的数字")
	}
	return nil
}

func envList(values map[string]string) []string {
	return envutil.EnvList(values)
}

type exitError struct {
	code    int
	message string
}

func (e *exitError) Error() string { return e.message + "（退出码 " + strconv.Itoa(e.code) + "）" }
