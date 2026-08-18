<p align="right">简体中文 | <a href="./README.en.md">English</a></p>

# ArticleToMotion

ArticleToMotion 是面向 macOS 和 Linux 的竖屏 MG 视频生产 CLI。它可以把定稿 SRT 拆成并发渲染的动画镜头，也可以从文章或口播稿完成 TTS、字幕、封面、声音制作和发布交付。

▶ [观看 ArticleToMotion 宣传视频（MP4，4 分 47 秒）](https://github.com/chouheiwa/articale-to-motion/blob/main/docs/article-to-motion-tutorial.mp4)

Go 版本以单个 `am` 二进制发布，不依赖 Python。项目 Prompt、发布模板和视觉规范由二进制内置；HyperFrames 技能在初始化时按固定官方版本装进项目，项目因此自包含——整个目录拷到另一台机器就能渲染。

## 安装

从 GitHub Releases 下载适合平台的压缩包，校验 `checksums.txt` 后把 `am` 放入 `PATH`。运行需要：

- macOS 或 Linux（amd64/arm64）
- Node.js 22+
- Git、FFmpeg、FFprobe
- Codex、Claude Code、Qoder、CodeBuddy、OpenCode 中至少一个已登录的 CLI

## 创建项目

```bash
am init my-video                          # 交互选择画幅
am init my-video --canvas vertical-9x16   # 或显式指定
cd my-video
```

画幅在初始化时一次性选定，写入项目的 `frame.md` 后不再更改。当前内置：

| 预设 | 画幅 | 用途 |
|---|---|---|
| `vertical-3x4` | 1080×1440，30fps | 默认，知识与科技类竖屏解说 |
| `vertical-9x16` | 1080×1920，30fps | 抖音等全屏竖屏平台 |

非交互环境（CI、管道）必须显式传入 `--canvas`，不会静默取默认值。

`am init` 不覆盖内容不同的已有文件。默认联网安装固定版本 0.8.1 的 HyperFrames 官方技能到项目的 `.agents/skills/`，与随二进制下发的 `text-to-lottie`、`algorithmic-art` 同处一个目录；离线或 CI 环境可使用 `--skip-hyperframes`。

技能装进项目而不是 HOME，是因为版本固定只有这样才成立：上游安装器只认 `homedir`，一台机器上只有一份技能，项目 A 固定 0.8.1、项目 B 固定 0.7.108 时谁后初始化谁说了算。安装全程不写用户 HOME——上游安装器在项目外的临时目录里运行，产物再搬进项目。

初始化还会把二进制内置的技能树写到项目的 `.agents/skills/`，渲染工具会自动发现：

| 技能 | 用途 |
|---|---|
| `text-to-lottie` | 镜头里的 Lottie 图层：logo 演绎、图标、loader、SVG 描边、矢量特效 |
| `algorithmic-art` | 算法与生成式视觉效果 |

这两个随二进制下发，不需要联网安装，`--skip-hyperframes` 也不影响它们。

编辑 `article-to-motion.conf` 选择编排工具与渲染工具；两者可以相同：

```text
ORCHESTRATOR=codex
RENDERER=claude
TTS_PROVIDER=minimax
SCENE_JOBS=3
```

优先级为环境变量、项目 `.env`、配置文件、内置默认。项目 `.env` 不允许保存 API Key、Token、Secret 或 Password。

## 工作流

已有定稿 SRT 时，将它保存为 `transcription.srt`，然后运行：

```bash
am run
```

从文章、口播稿或参考字幕开始完整制作：

```bash
am run PROMPT-PRODUCTION.md
```

`am run` 会自动将当前 `am` 可执行文件所在目录放到编排子进程的 `PATH` 首位，并注入其绝对路径为 `AM_EXECUTABLE`。因此 Prompt 中的 `am scene ...` 会继续使用启动本次流程的同一个 CLI，无需把二进制复制进项目。

镜头命令也可独立使用：

```bash
am scene run scenes/scene-001
am scene run-all scenes/ --jobs 3 --retries 2 --report-json production/run-report.json
```

已有合格产物会跳过；输入更新后的产物标记为 stale，使用 `--force` 明确重渲染。中断时停止新任务、终止在飞进程组，并写出部分报告。

## 安全模式

外部 AI CLI 默认限制在项目工作区。若某个已安装版本不支持受限的非交互模式，命令会失败关闭，不会自动扩大权限。

只有确认项目和输入可信时才使用：

```bash
am --unsafe run
```

`--unsafe` 会传递给嵌套镜头任务。安全模式需要额外传递的环境变量通过 `AM_PASSTHROUGH_ENV=NAME1,NAME2` 显式列出。

## 视觉检查与拼接

渲染成功不等于画面正确：文字压在图形上看不清、中文字体静默回退成方框、动画在中途就停住，这些都能拿到退出码 0。抽帧看图是唯一能提前发现它们的手段。

```bash
# 抽帧供视觉验收，--at 支持秒数、百分比与 end
am scene frames scenes/scene-001 --at 0,50%,end
am scene frames scenes/scene-001 --at 0 --check-blank   # 空白封面直接判失败

# 按镜头顺序拼成静音母版，规格一致时全程 -c copy
am concat scenes/ --out production/silent-master.mp4 --dry-run
am concat scenes/ --out production/silent-master.mp4
```

`am concat` 不提供重定时能力。分辨率或帧率与母版不符时直接失败并要求回镜头工程重渲染，而不是缩放或改帧率——前者损失画质，后者会动到时间轴。

## 校验与归档

```bash
am validate publish publish.md --project-root .
am validate style --project-root .
am validate style --project-root . --regenerate-examples  # 需要 rsvg-convert 与 magick

# 成片机器验收：画幅、帧率、编码、像素格式、帧数、音轨一次全查，不符项一次列全
am validate video production/silent-master.mp4 --silent --decode
am validate video final.mp4 --audio --expect-frames 8340 --check-frame-zero \
  --report-json production/final-check.json

am archive --dry-run
am archive
```

`--expect-frames` 与 `--decode` 会完整解码一遍，代价与文件长度成正比，所以默认关闭。判断结果读 `--report-json` 落盘的文件，不要解析终端输出。

归档在共享文件有修改或存在 ignored 文件时拒绝执行。成功后单片文件连同 SHA-256 清单移到仓库外归档目录，工作区 detach 到冻结的 `main` 提交。

## 开发

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/am
```

项目采用 [Apache License 2.0](./LICENSE)。HyperFrames 是独立的 Apache-2.0 项目，ArticleToMotion 初始化时使用固定版本，不在本仓库复制本机技能目录。

内置技能 `text-to-lottie` 的 `references/` 裁剪自 [diffusionstudio/lottie](https://github.com/diffusionstudio/lottie)（MIT），版权声明与改动记录见 `assets/shared/.agents/skills/text-to-lottie/` 下的 `LICENSE` 与 `ATTRIBUTION.md`。
