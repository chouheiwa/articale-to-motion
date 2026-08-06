<p align="right">简体中文 | <a href="./README.en.md">English</a></p>

# ArticleToMotion

ArticleToMotion 是面向 macOS 和 Linux 的竖屏 MG 视频生产 CLI。它可以把定稿 SRT 拆成并发渲染的动画镜头，也可以从文章或口播稿完成 TTS、字幕、封面、声音制作和发布交付。

▶ [观看 ArticleToMotion 宣传视频（MP4，4 分 47 秒）](https://github.com/chouheiwa/articale-to-motion/releases/download/v1.0.0/article-to-motion-tutorial.mp4)

Go 版本以单个 `am` 二进制发布，不依赖 Python。项目 Prompt、发布模板和视觉规范由二进制内置，HyperFrames 技能在初始化时从固定官方版本安装。

## 安装

从 GitHub Releases 下载适合平台的压缩包，校验 `checksums.txt` 后把 `am` 放入 `PATH`。运行需要：

- macOS 或 Linux（amd64/arm64）
- Node.js 22+
- Git、FFmpeg、FFprobe
- Codex、Claude Code、Qoder、CodeBuddy、OpenCode 中至少两个已登录的 CLI

## 创建项目

```bash
am init my-video
cd my-video
```

`am init` 不覆盖内容不同的已有文件。默认执行 `npx --yes hyperframes@0.7.94 skills`；离线或 CI 环境可使用 `--skip-hyperframes`。

编辑 `article-to-motion.conf` 选择不同的编排工具与渲染工具：

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

## 校验与归档

```bash
am validate publish publish.md --project-root .
am validate style --project-root .
am validate style --project-root . --regenerate-examples  # 需要 ImageMagick
am archive --dry-run
am archive
```

归档在共享文件有修改或存在 ignored 文件时拒绝执行。成功后单片文件连同 SHA-256 清单移到仓库外归档目录，工作区 detach 到冻结的 `main` 提交。

## 开发

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/am
```

项目采用 [Apache License 2.0](./LICENSE)。HyperFrames 是独立的 Apache-2.0 项目，ArticleToMotion 初始化时使用固定版本，不在本仓库复制本机技能目录。
