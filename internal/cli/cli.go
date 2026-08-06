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
	"github.com/chouheiwa/articale-to-motion/internal/project"
	"github.com/chouheiwa/articale-to-motion/internal/scene"
	"github.com/chouheiwa/articale-to-motion/internal/schedule"
	"github.com/chouheiwa/articale-to-motion/internal/tools"
	"github.com/chouheiwa/articale-to-motion/internal/validate"
	"github.com/spf13/cobra"
)

const Version = "1.0.0"

func currentEnvironment() map[string]string {
	result := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}

func addExecutableLocation(env map[string]string) {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
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
		if errors.Is(ctx.Err(), context.Canceled) {
			return 130
		}
		var coded *exitError
		if errors.As(err, &coded) {
			return coded.code
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
		Use:           "am",
		Short:         "ArticleToMotion 竖屏 MG 视频制作工具",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().BoolVar(&unsafe, "unsafe", false, "显式关闭 AI CLI 权限隔离（危险）")

	var skipHyperframes bool
	initCmd := &cobra.Command{
		Use:   "init [DIR]",
		Short: "初始化一个可复现的视频项目",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			target, _ = filepath.Abs(target)
			result, err := project.Initialize(target, assets.Files)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "项目已初始化：%s（新增 %d，未变 %d）\n", target, result.Created, result.Unchanged)
			if skipHyperframes {
				fmt.Fprintln(stdout, "警告：已跳过 HyperFrames 技能安装")
				return nil
			}
			npx := exec.Command("npx", "--yes", "hyperframes@0.7.94", "skills")
			npx.Dir, npx.Stdout, npx.Stderr = target, stdout, stderr
			if err := npx.Run(); err != nil {
				return fmt.Errorf("项目文件已写入，但 HyperFrames 技能安装失败；可在项目目录重试 npx --yes hyperframes@0.7.94 skills: %w", err)
			}
			return nil
		},
	}
	initCmd.Flags().BoolVar(&skipHyperframes, "skip-hyperframes", false, "跳过联网安装 HyperFrames 技能")
	root.AddCommand(initCmd)

	configCmd := &cobra.Command{Use: "config", Short: "读取解析后的配置"}
	configCmd.AddCommand(&cobra.Command{
		Use:   "get ORCHESTRATOR|RENDERER|TTS_PROVIDER",
		Args:  cobra.ExactArgs(1),
		Short: "打印单个配置键",
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

	sceneCmd := &cobra.Command{Use: "scene", Short: "镜头相关操作"}
	var tolerance float64
	runScene := &cobra.Command{
		Use: "run DIRECTORY", Args: cobra.ExactArgs(1), Short: "执行单个镜头",
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
			return scene.Run(cmd.Context(), s, cfg, unsafe || os.Getenv("AM_UNSAFE") == "1", currentEnvironment(), stdout, tolerance)
		},
	}
	runScene.Flags().Float64Var(&tolerance, "duration-tolerance", 0.15, "产物时长容差（秒）")
	sceneCmd.AddCommand(runScene)

	var jobs, retries int
	var force bool
	var reportPath string
	runAll := &cobra.Command{
		Use: "run-all ROOT", Args: cobra.ExactArgs(1), Short: "并行执行多个镜头",
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
					outputInfo, _ := os.Stat(item.OutputPath())
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
				return scene.Run(ctx, s, cfg, unsafe || os.Getenv("AM_UNSAFE") == "1", currentEnvironment(), stdout, tolerance)
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
	runAll.Flags().IntVar(&jobs, "jobs", 0, "并发上限，默认读取 SCENE_JOBS")
	runAll.Flags().IntVar(&retries, "retries", 2, "最大重试次数")
	runAll.Flags().Float64Var(&tolerance, "duration-tolerance", 0.15, "产物时长容差（秒）")
	runAll.Flags().BoolVar(&force, "force", false, "忽略已有合格产物")
	runAll.Flags().StringVar(&reportPath, "report-json", "", "写入机器可读报告")
	sceneCmd.AddCommand(runAll)
	root.AddCommand(sceneCmd)

	var workdir string
	runCmd := &cobra.Command{Use: "run [PROMPT]", Short: "按配置启动编排工具", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
		isUnsafe := unsafe || os.Getenv("AM_UNSAFE") == "1"
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
		childEnvironment := cfg.ChildEnvironment(currentEnvironment(), isUnsafe, strings.FieldsFunc(os.Getenv("AM_PASSTHROUGH_ENV"), func(r rune) bool { return r == ',' || r == ' ' }))
		addExecutableLocation(childEnvironment)
		process.Env = envList(childEnvironment)
		if stdin != "" {
			process.Stdin = strings.NewReader(stdin)
		}
		fmt.Fprintf(stdout, "编排工具: %s\n渲染工具: %s\nPrompt: %s\n---\n", cfg.Orchestrator, cfg.Renderer, promptPath)
		if err := runProcessGroup(cmd.Context(), process); err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				return &exitError{code: exit.ExitCode(), message: "编排工具执行失败"}
			}
			return err
		}
		return nil
	}}
	runCmd.Flags().StringVar(&workdir, "workdir", "", "编排工具工作目录，默认当前项目根")
	root.AddCommand(runCmd)

	var mainRef, archiveRoot, archiveName string
	var yes, dryRun bool
	archiveCmd := &cobra.Command{Use: "archive", Short: "归档当前项目并复位到 main", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
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
	archiveCmd.Flags().StringVar(&mainRef, "main-ref", "main", "冻结的 main ref")
	archiveCmd.Flags().StringVar(&archiveRoot, "archive-root", "", "归档父目录")
	archiveCmd.Flags().StringVar(&archiveName, "archive-name", "", "归档目录名")
	archiveCmd.Flags().BoolVar(&yes, "yes", false, "跳过确认")
	archiveCmd.Flags().BoolVar(&dryRun, "dry-run", false, "仅打印计划")
	root.AddCommand(archiveCmd)

	validateCmd := &cobra.Command{Use: "validate", Short: "校验发布配置与风格规范"}
	var projectRoot string
	var templateMode bool
	var regenerateExamples bool
	publishCmd := &cobra.Command{Use: "publish PATH", Args: cobra.ExactArgs(1), Short: "校验 publish.md", RunE: func(cmd *cobra.Command, args []string) error {
		rootDir := projectRoot
		if rootDir == "" {
			rootDir = "."
		}
		data, err := validate.Publish(args[0], rootDir, templateMode)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "publish 校验通过：%s（%v）\n", args[0], data["publish_status"])
		return nil
	}}
	publishCmd.Flags().StringVar(&projectRoot, "project-root", "", "项目根目录")
	publishCmd.Flags().BoolVar(&templateMode, "template", false, "模板模式")
	validateCmd.AddCommand(publishCmd)
	styleCmd := &cobra.Command{Use: "style", Args: cobra.NoArgs, Short: "校验风格规范", RunE: func(cmd *cobra.Command, args []string) error {
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
	styleCmd.Flags().StringVar(&projectRoot, "project-root", "", "项目根目录")
	styleCmd.Flags().BoolVar(&regenerateExamples, "regenerate-examples", false, "使用 ImageMagick 重新生成通用风格示例")
	validateCmd.AddCommand(styleCmd)
	root.AddCommand(validateCmd)
	return root
}

func validateTolerance(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("--duration-tolerance 必须是有限且非负的数字")
	}
	return nil
}

func envList(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

type exitError struct {
	code    int
	message string
}

func (e *exitError) Error() string { return e.message + "（退出码 " + strconv.Itoa(e.code) + "）" }
