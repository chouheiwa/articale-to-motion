package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"

	assets "github.com/chouheiwa/articale-to-motion"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
	"github.com/chouheiwa/articale-to-motion/internal/project"
	"github.com/chouheiwa/articale-to-motion/internal/scene"
	"github.com/chouheiwa/articale-to-motion/internal/schedule"
	"github.com/chouheiwa/articale-to-motion/internal/song"
	"github.com/spf13/cobra"
)

func songScaffold(root string) error {
	if _, e := os.Stat(filepath.Join(root, "cast.yaml")); e == nil {
		return fmt.Errorf("首版不支持 song + cast")
	}
	extra := map[string][]byte{"song.yaml": []byte(song.DefaultConfig)}
	shared, e := assets.Shared()
	if e != nil {
		return e
	}
	for _, prefix := range []string{"PROMPT-SONG.md", ".agents/skills/song-explainer"} {
		e = fs.WalkDir(shared, prefix, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			b, err := fs.ReadFile(shared, path)
			if err == nil {
				extra[path] = b
			}
			return err
		})
		if e != nil {
			return e
		}
	}
	_, e = project.InitializeWithFiles(root, extra)
	return e
}
func newSongCmd(out io.Writer) *cobra.Command {
	root := "."
	var timeout time.Duration
	cmd := &cobra.Command{Use: "song", Short: "单人歌曲讲解：生成、试听选择、歌词对齐与镜头节拍"}
	cmd.Long = "歌曲讲解以 song.yaml 启用，与默认口播独立。先生成或导入候选，用户试听 select 后 prepare；核对时间轴再 cues、validate song 和渲染。运行 am song providers 查看平台与 API Key 申请入口，am song configure 选择平台。首版不支持 cast。所有产物保存在 production/song，密钥仅来自进程环境。"
	cmd.PersistentFlags().StringVar(&root, "project-root", ".", "歌曲项目根目录")
	cmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Minute, "生成/对齐的最长等待时间；ACE-Step/fal/Mureka 可续跑")
	add := func(use, short string, n int, fn func(context.Context, []string) error) *cobra.Command {
		sub := &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(n), RunE: func(c *cobra.Command, a []string) error {
			if timeout <= 0 {
				return fmt.Errorf("--timeout 必须为正")
			}
			ctx, cancel := context.WithTimeout(c.Context(), timeout)
			defer cancel()
			return fn(ctx, a)
		}}
		sub.Long = short + "。\n\n" + songHelp[use]
		sub.Example = "  am song " + songExamples[use]
		cmd.AddCommand(sub)
		return sub
	}
	providers := &cobra.Command{Use: "providers [PLATFORM]", Short: "查看歌曲平台、申请入口和密钥配置步骤", Long: "列出可用歌曲平台；可指定 bailian、elevenlabs、fal、minimax、acestep、mureka、lyria 或 manual 查看开通要求、环境变量和配置示例。只显示密钥是否设置，不读取展示其内容，不请求网络或生成歌曲，无需先初始化项目。", Example: "  am song providers\n  am song providers bailian", Args: cobra.MaximumNArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		id := ""
		if len(args) > 0 {
			id = args[0]
		}
		return song.PlatformGuide(out, id, currentEnvironment())
	}}
	cmd.AddCommand(providers)
	var provider, model, workspace, endpoint string
	configure := &cobra.Command{Use: "configure", Short: "选择歌曲平台并引导配置 API Key", Long: "修改现有 song.yaml 的平台、模型及服务地址，保留歌词、曲风和其他设置；先运行 song init。切换平台清理旧 endpoint/workspace，默认采用新平台模型。密钥仅通过进程环境读取，本命令不收集、不保存密钥，不登录或调用收费 API。未设置密钥仍可保存平台设置，随后按输出步骤配置。", Example: "  am song configure --provider bailian --workspace YOUR_WORKSPACE_ID\n  am song configure --provider elevenlabs\n  am song configure --provider fal\n  am song configure --provider mureka\n  am song configure --provider lyria", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		if e := song.Configure(root, provider, model, workspace, endpoint); e != nil {
			return e
		}
		fmt.Fprintln(out, "已更新 song.yaml；原有候选保留，生成前请完成以下接入步骤。")
		return song.PlatformGuide(out, provider, currentEnvironment())
	}}
	configure.Flags().StringVar(&provider, "provider", "", "平台：bailian|elevenlabs|fal|minimax|acestep|mureka|lyria|manual（必填）")
	configure.Flags().StringVar(&model, "model", "", "留空使用所选平台默认模型")
	configure.Flags().StringVar(&workspace, "workspace", "", "百炼北京地域业务空间 ID（非密钥）")
	configure.Flags().StringVar(&endpoint, "endpoint", "", "仅自建 ACE-Step 的 HTTP(S) 服务地址（不含密钥）")
	cmd.AddCommand(configure)
	add("init", "为已有项目补充歌曲资产，不覆盖已有内容", 0, func(_ context.Context, _ []string) error {
		if e := songScaffold(root); e != nil {
			return e
		}
		fmt.Fprintln(out, "歌曲资产已初始化。运行 am song providers 查看平台，再用 am song configure --provider PLATFORM 配置。")
		return nil
	})
	var refreshHandoff bool
	handoff := add("handoff", "输出网页生成用的歌词和风格，记录等待用户音频", 0, func(_ context.Context, _ []string) error { return song.HandoffWithRefresh(root, refreshHandoff, out) })
	handoff.Flags().BoolVar(&refreshHandoff, "refresh", false, "明确用新版歌词/曲风替换待接收材料；用户需重新在网页生成")
	add("receive AUDIO", "接收网页生成音频，绑定交接时的歌词快照", 1, func(_ context.Context, a []string) error {
		m, e := mediaprobe.New(currentEnvironment())
		if e != nil {
			return e
		}
		c, e := song.Receive(root, a[0], m)
		if e != nil {
			return e
		}
		fmt.Fprintf(out, "已接收候选 %s；请核对唱词并由用户确认后运行 am song select %s。尚未选定或对齐。\n", c.ID, c.ID)
		return nil
	})
	add("doctor", "检查服务、对齐环境与媒体工具", 0, func(ctx context.Context, _ []string) error { return song.Doctor(ctx, root, currentEnvironment(), out) })
	count := 2
	batch := "default"
	gen := add("generate", "串行生成候选并停止，等待用户试听选择", 0, func(ctx context.Context, _ []string) error {
		return song.Generate(ctx, root, count, batch, currentEnvironment(), out)
	})
	gen.Flags().IntVar(&count, "count", 2, "候选数 1..8；相同请求复用候选")
	gen.Flags().StringVar(&batch, "batch", "default", "显式使用新批次标识才会重新生成")
	lyrics := ""
	imp := add("import AUDIO", "复制外部音频和歌词为候选", 1, func(_ context.Context, a []string) error {
		if lyrics == "" {
			return fmt.Errorf("必须提供 --lyrics")
		}
		m, e := mediaprobe.New(currentEnvironment())
		if e != nil {
			return e
		}
		c, e := song.Import(root, a[0], lyrics, m)
		if e == nil {
			fmt.Fprintln(out, "已导入候选：", c.ID)
		}
		return e
	})
	imp.Flags().StringVar(&lyrics, "lyrics", "", "经核对的歌词文件")
	add("select CANDIDATE", "记录用户试听选定的歌曲及摘要", 1, func(_ context.Context, a []string) error {
		if e := song.Select(root, a[0]); e != nil {
			return e
		}
		fmt.Fprintln(out, "已选定：", a[0])
		return nil
	})
	manual := ""
	reviewed := false
	prep := add("prepare", "对齐歌词、分析节拍，或导入修正时间轴", 0, func(ctx context.Context, _ []string) error {
		canvas, e := scene.CanvasOf(root, "frame.md")
		if e != nil {
			return e
		}
		if e = song.Prepare(ctx, root, manual, reviewed, currentEnvironment(), out); e != nil {
			return e
		}
		return song.ExportPhrases(root, canvas.FPS)
	})
	prep.Flags().StringVar(&manual, "timeline", "", "导入人工修正的统一时间轴 JSON")
	prep.Flags().BoolVar(&reviewed, "reviewed", false, "确认导入的时间轴已核对歌词、术语和句首时间")
	add("cues SCENES", "计算镜头局部歌词和节拍，绑定全局摘要", 1, func(_ context.Context, a []string) error { return applySongCues(root, a[0], out) })
	output := ""
	mux := add("mux SILENT-MASTER", "将完整歌曲与整数帧静音母版合成", 1, func(ctx context.Context, a []string) error {
		if output == "" {
			return fmt.Errorf("必须提供 --out")
		}
		t, e := song.LoadTimeline(root)
		if e != nil {
			return e
		}
		if !t.Reviewed {
			return fmt.Errorf("时间轴尚未人工复核")
		}
		c, e := song.Selected(root)
		if e != nil {
			return e
		}
		dir, _ := song.CandidateDir(root, c.ID)
		canvas, e := scene.CanvasOf(root, "frame.md")
		if e != nil {
			return e
		}
		m, e := mediaprobe.New(currentEnvironment())
		if e != nil {
			return e
		}
		report, e := m.Verify(a[0], mediaprobe.VerifyOptions{Spec: mediaprobe.Spec{WidthPx: canvas.WidthPx, HeightPx: canvas.HeightPx}})
		if e != nil {
			return e
		}
		if !report.OK {
			return fmt.Errorf("静音母版画幅不符：%v", report.Problems)
		}
		scenes, e := schedule.Plan(filepath.Join(root, "scenes"))
		if e != nil {
			return e
		}
		if e = songCoverage(root, scenes, t, true); e != nil {
			return e
		}
		if e = songOutputsCurrent(scenes, a[0]); e != nil {
			return e
		}
		return m.MuxSong(ctx, a[0], filepath.Join(dir, c.Audio), output, t.Duration, canvas.FPS)
	})
	mux.Flags().StringVar(&output, "out", "", "成片输出路径（MP4）")
	return cmd
}
func songCoverage(root string, scenes []scene.Scene, t song.Timeline, requireCues bool) error {
	canvas, e := scene.CanvasOf(root, "frame.md")
	if e != nil {
		return e
	}
	if !t.Reviewed {
		return fmt.Errorf("歌曲时间轴尚未人工复核；使用 prepare --timeline FILE --reviewed")
	}
	target := float64(t.Frames(canvas.FPS)) / float64(canvas.FPS)
	if p := schedule.CoverageProblems(scenes, target, 1e-6); len(p) > 0 {
		return fmt.Errorf("歌曲需从零覆盖前奏至尾奏：%v", p)
	}
	cursor := 0.0
	for _, s := range scenes {
		if s.Cast != nil {
			return fmt.Errorf("首版不支持 song + cast")
		}
		if math.Abs(s.DurationSeconds*float64(canvas.FPS)-math.Round(s.DurationSeconds*float64(canvas.FPS))) > 1e-5 {
			return fmt.Errorf("镜头 %s 时长必须对齐整数帧", s.ID)
		}
		if requireCues {
			if s.Song == nil || s.Song.FPS != canvas.FPS || math.Abs(s.Song.Start-cursor) > 1e-6 {
				return fmt.Errorf("镜头 %s 歌曲起点不一致，请运行 am song cues", s.ID)
			}
			targetPath, _ := filepath.Abs(filepath.Join(root, song.Store, "timeline.json"))
			cuePath, _ := filepath.Abs(filepath.Join(s.Directory, s.Song.Timeline))
			if targetPath != cuePath {
				return fmt.Errorf("镜头 %s 引用了其他项目歌曲", s.ID)
			}
			if e = song.VerifyCue(s.Directory, *s.Song, s.DurationSeconds); e != nil {
				return e
			}
		}
		cursor += s.DurationSeconds
	}
	return nil
}
func applySongCues(root, scenesRoot string, out io.Writer) error {
	t, e := song.LoadTimeline(root)
	if e != nil {
		return e
	}
	scenes, e := schedule.Plan(scenesRoot)
	if e != nil {
		return e
	}
	if e = songCoverage(root, scenes, t, false); e != nil {
		return e
	}
	timeline, e := filepath.Abs(filepath.Join(root, song.Store, "timeline.json"))
	if e != nil {
		return e
	}
	sha, e := song.HashFile(timeline)
	if e != nil {
		return e
	}
	canvas, e := scene.CanvasOf(root, "frame.md")
	if e != nil {
		return e
	}
	type update struct {
		path string
		body any
	}
	var updates []update
	cursor := 0.0
	for _, s := range scenes {
		dir, e := filepath.Abs(s.Directory)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(dir, timeline)
		if e != nil {
			return e
		}
		data := t.Slice(cursor, s.DurationSeconds)
		encoded, e := json.MarshalIndent(data, "", "  ")
		if e != nil {
			return e
		}
		encoded = append(encoded, '\n')
		// Compute the exact file digest before mutating any scene.
		dataSHA := song.BytesHash(encoded)
		cue := song.Cue{FPS: canvas.FPS, Timeline: rel, SHA: sha, Start: cursor, Data: "song-cues.json", DataSHA: dataSHA}
		sceneFile, e := song.LocalPath(dir, "scene.json")
		if e != nil {
			return e
		}
		cueFile, e := song.LocalPath(dir, cue.Data)
		if e != nil {
			return e
		}
		raw, e := os.ReadFile(sceneFile)
		if e != nil {
			return e
		}
		var fields map[string]json.RawMessage
		if e = json.Unmarshal(raw, &fields); e != nil {
			return e
		}
		fields["song"], e = json.Marshal(cue)
		if e != nil {
			return e
		}
		updates = append(updates, update{cueFile, data}, update{sceneFile, fields})
		cursor += s.DurationSeconds
	}
	for _, u := range updates {
		if e = song.WriteJSON(u.path, u.body); e != nil {
			return e
		}
	}
	fmt.Fprintf(out, "已更新 %d 个镜头的歌曲依赖\n", len(scenes))
	return nil
}
func newValidateSongCommand(out io.Writer) *cobra.Command {
	root := "."
	scenesRoot := ""
	cmd := &cobra.Command{Use: "song", Short: "校验歌曲选择、歌词、时间轴和镜头覆盖", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		t, e := song.LoadTimeline(root)
		if e != nil {
			return e
		}
		if !t.Reviewed {
			return fmt.Errorf("时间轴尚未人工复核")
		}
		dir := scenesRoot
		if dir == "" {
			dir = filepath.Join(root, "scenes")
		}
		scenes, e := schedule.Plan(dir)
		if e != nil {
			return e
		}
		if e = songCoverage(root, scenes, t, true); e != nil {
			return e
		}
		fmt.Fprintln(out, "歌曲与镜头时间轴校验通过")
		return nil
	}}
	cmd.Long = "检查已选歌曲和歌词摘要、已人工复核的统一时间轴，以及所有镜头从零至尾奏的整数帧覆盖。检查每镜头局部歌词和全局摘要；发现过期依赖时必须重跑 song cues 并重渲染。失败退出 1。"
	cmd.Example = "  am validate song --project-root ."
	cmd.Flags().StringVar(&root, "project-root", ".", "项目根目录")
	cmd.Flags().StringVar(&scenesRoot, "scenes", "", "镜头目录，默认项目 scenes/")
	return cmd
}

var songHelp = map[string]string{
	"handoff":       "读取已创作的歌词及 song.yaml 曲风，输出可复制文本，并保存不可变快照及 production/song/web-handoff.json 等待记录。不调用 API，不需要 API Key、Python 或媒体工具。正常退出后必须暂停等待用户提供音频，不能将命令成功当成歌曲生成成功；同一会话或再次 am run 可继续。",
	"receive AUDIO": "根据当前网页交接记录导入用户提供的本机音频，使用交接时的歌词快照，更新接收状态；不自动选择、对齐或渲染。同一音频重跑复用候选，不同音频创建新候选。必须先询问网页是否改词；改词时请用 song import AUDIO --lyrics FINAL_LYRICS 显式导入核对后的最终歌词。",

	"init":              "在当前项目补充 song.yaml、PROMPT-SONG.md 和 song-explainer 技能；已有文件内容不同则整体失败，不改用户提示词，不安装 Python 或模型。cast.yaml 存在时拒绝。",
	"doctor":            "检查 song.yaml 选定服务、ffprobe/ffmpeg 和独立 Python 环境的 Xingyu 0.6.1。librosa 0.11.0 缺失只警告。MiniMax 检查 CLI 能力，不调用收费服务；ACE-Step 仅请求 /health；百炼、ElevenLabs、fal、Mureka、Lyria 仅检查密钥是否存在，不验证远端权限或产生费用。",
	"generate":          "manual 模式仅输出网页交接材料并暂停等待文件；其他平台默认两首，串行执行，完成后必须停下来给用户试听，不代选。相同配置、歌词与 batch 复用已有候选；ACE-Step 已保存任务 ID 可续轮询；无 ID 的不明提交拒绝重发。明确需要新候选时换 --batch。运行 am song providers 查看平台密钥配置。MiniMax 使用 mmx；其他平台从环境读取密钥。Mureka、fal 和 ACE-Step 已知任务可续轮询；百炼、ElevenLabs 与 Lyria 同步请求中断不自动重发。",
	"import AUDIO":      "复制音频和核对过的歌词为新的不可变候选，实测时长并完整解码。歌词中的结构标签不作为字幕。导入不意味着选定，必须另行 select。",
	"select CANDIDATE":  "仅在用户试听确定后执行。选定记录保存候选、歌词和音频摘要；变化会使旧时间轴和镜头失效。本命令不对齐或渲染。",
	"prepare":           "默认调用独立 Python 环境中的 Xingyu 0.6.1，再用 librosa 0.11.0 提取真实节拍；模型由用户独立安装。保留原始报告、行映射和问题清单。失败时修正 timeline-draft.json，通过 --timeline 导入；确认歌词、术语和句首误差后加 --reviewed 才能分镜渲染。估算字词不参与逐字同步。",
	"cues SCENES":       "按镜头目录字典序累计整数帧时长，将全局歌词及节拍裁切为局部秒数。总长度必须为 ceil(实际歌曲秒数 × fps)，含前奏与尾奏。更新 scene.json 的 song 块和 song-cues.json；校验失败不写文件，成功后旧镜头需 --force 重渲染。",
	"mux SILENT-MASTER": "检查静音母版总帧数与歌曲时间轴一致，复制视频并加入完整歌曲。只允许末尾不足一帧的静音补齐，不变速、不叠 BGM、不做侧链。输出为 MP4 并执行完整解码与规格检查。",
}
var songExamples = map[string]string{
	"handoff": "handoff", "receive AUDIO": "receive /path/to/downloaded-song.mp3",
	"init": "init --project-root .", "doctor": "doctor", "generate": "generate --count 2 --batch first", "import AUDIO": "import song.mp3 --lyrics lyrics.txt", "select CANDIDATE": "select first-candidate", "prepare": "prepare\n  am song prepare --timeline production/song/timeline.json --reviewed", "cues SCENES": "cues scenes/", "mux SILENT-MASTER": "mux production/silent-master.mp4 --out final.mp4",
}

// Reject stale song renders even if callers bypass run-all and go straight to
// concat/mux. The existing renderer's file timestamps define artifact freshness.
func songOutputsCurrent(scenes []scene.Scene, master string) error {
	var masterInfo os.FileInfo
	if master != "" {
		var e error
		masterInfo, e = os.Stat(master)
		if e != nil {
			return e
		}
	}
	for _, s := range scenes {
		if s.Song == nil {
			continue
		}
		if e := scene.VerifySongInputs(s); e != nil {
			return e
		}
		output, e := os.Stat(s.OutputPath())
		if e != nil {
			return e
		}
		for _, name := range []string{"scene.json", "prompt.md", s.Song.Data} {
			info, e := os.Stat(filepath.Join(s.Directory, name))
			if os.IsNotExist(e) && name == "prompt.md" {
				continue
			}
			if e != nil {
				return e
			}
			if info.ModTime().After(output.ModTime()) {
				return fmt.Errorf("歌曲镜头 %s 已过期，需要 --force 重渲染", s.ID)
			}
		}
		if masterInfo != nil && output.ModTime().After(masterInfo.ModTime()) {
			return fmt.Errorf("静音母版早于镜头 %s，需要重新 concat", s.ID)
		}
	}
	return nil
}
