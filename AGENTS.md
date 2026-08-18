# 仓库规则

本仓库是 ArticleToMotion 的 Go CLI 工具本体，交付物是单个 `am` 二进制。它**不是**视频项目：这里不制作视频，也不存放任何单片内容。给视频项目用的规则是 `assets/shared/templates/project-rules.md`，那份文件随项目骨架下发到用户项目的 `templates/project-rules.md`，不约束本仓库的开发。

## 命令

```bash
go build ./cmd/am
go test ./...
go test -race ./...
go vet ./...
```

## 内置资产

项目骨架分两棵源树，`assets.go` 的 `//go:embed` 清单是唯一来源：

| 源树 | 内容 |
|---|---|
| `assets/shared/` | 与画幅无关，所有预设共用（`PROMPT.md`、配置模板、字体、`templates/`、`.agents/skills/`） |
| `assets/presets/<id>/` | 含画幅数字，每套预设一份（`frame.md`、`PROMPT-PRODUCTION.md`、风格说明书、示例图） |

**每棵树的内部路径就是它在用户项目根下的目标路径**，`project.Initialize` 把选中的两棵树叠加拷贝，不做任何路径改写。所以 `assets/shared/assets/fonts/` 这层嵌套是刻意的——它对应项目里的 `assets/fonts/`。两棵树写入同一路径会直接报错，不静默覆盖。

**新增一个需要随 `am init` 下发的文件时，要放进对的那棵树，并确认它落在 `//go:embed` 清单覆盖范围内**，否则它只存在于本仓库，永远到不了用户项目。`.env.example` 和 `.agents` 这类点开头的条目必须在 embed 指令里单列——目录模式会跳过它们，漏了不报错，只是那份内容从二进制里静默消失。

`am init` 不覆盖内容不同的已有文件：目标已存在且字节不同就整体失败并回滚，不做合并。

## 技能树

`assets/shared/.agents/skills/<name>/` 是随 `am init` 下发到用户项目根 `.agents/skills/` 的技能树。`scene.ResolveSkill` 从镜头目录逐级向上找 `.agents/skills`，落到项目根就能被发现，用户无需另行安装。

`hyperframes-animation` 等官方技能不在这棵树里，但**落点相同**：`am init` 通过 `internal/hyperframes` 把它们按固定版本装进同一个 `.agents/skills/`。这棵源树只放本仓库自带的技能。

官方技能不能直接跑 `npx hyperframes skills` 装——上游安装器只认 `homedir()`，没有任何选项能改落点（`skills update --dir` 的帮助原文写着 "scopes the prune, not the install"）。装在机器级会让版本固定名存实亡：一台机器只有一份技能，项目 A 固定 0.8.1、项目 B 固定 0.7.108 时谁后初始化谁说了算。

`internal/hyperframes` 的做法是在项目外开一个临时 HOME 让上游安装器照常工作，再把产出搬进项目。三个细节是承重的，改动前先读那里的注释：

| 细节 | 不这么做会怎样 |
|---|---|
| 用 `node -e process.execPath` 解析真实 npx | asdf / nvm / volta 的 shim 靠 HOME 找 node，改写 HOME 后直接报 `unknown command: npx` |
| 接回 `npm_config_cache` | 缓存写进临时 HOME，每个项目重新下载整包 |
| 接回 `npm_config_userconfig` | 读不到用户 `.npmrc`，私有 registry、代理和鉴权全丢，公司内网装不上 |

上游产物里出现符号链接时直接报错而不是照搬：那些链接指回临时 HOME，搬进项目后会成为静默失效的空技能。受保护技能名从嵌入树现读（`assets.BuiltinSkills`），新增内置技能自动受保护，不会被上游同名技能覆盖。

技能分两种来源，`SkillDescriptor.Source` 是它们的分水岭——不是元数据，而是契约：缺失补救、升级方式和守护测试全都不同。

| 来源 | 谁装 | 能否进嵌入树 |
|---|---|---|
| `SourceEmbedded` | 随二进制下发，`project.Initialize` 写进项目 | **必须**在 `assets/shared/.agents/skills/` |
| `SourceUpstream` | `am init` 按固定版本从上游装进项目 | **禁止**——那会留一份永远追不上上游的陈旧副本 |

两条门禁互为反面：`TestEveryRegisteredSkillIsEmbedded` 查前者在不在嵌入树，`TestUpstreamSkillsAreNotVendoredIntoTheEmbeddedTree` 查后者在不在。把上游技能拷进 `assets/` 能让第一条变绿，但会被第二条抓住。

新增一个**内置**技能要动的地方：

| 位置 | 作用 |
|---|---|
| `assets/shared/.agents/skills/<name>/SKILL.md` | 技能本体，`SKILL.md` 是必需文件 |
| `assets.go` 的 `//go:embed` 清单 | 已含 `assets/shared/.agents`，新技能自动覆盖 |
| `internal/scene/skills.go` 的 `RegisteredSkills` | 追加描述符，`Source: SourceEmbedded` |
| 同文件的 `<name>Prompt` 函数 | 该技能注入镜头提示词的片段 |
| `assets/shared/templates/project-rules.md` | 下发规则要点名它，有门禁强制 |

新增一个**上游**技能只要在 `RegisteredSkills` 加一条 `Source: SourceUpstream` 的描述符加提示词片段——它已经被 `am init` 装进项目了，注册只是让 `am` 把绝对路径和用法边界写进渲染提示词。

注册上游技能时要收窄范围，上游技能的职责范围通常比单镜头渲染大得多：

- `hyperframes-core` 同时覆盖建子项目与 `STORYBOARD.md` / `SCRIPT.md` 计划格式，那是整片工作流。分镜由上层决定并已写进 `scene.json`，不收窄的话渲染器会产出计划文件并试图自己排布多镜头。
- `hyperframes-cli` 覆盖 `cloud` / `cloudrun` / `lambda` / `publish` 远端渲染路径，以及 `skills` / `upgrade` 这类改动已装技能的命令。前者绕开本机确定性前提，后者会顶掉固定版本、影响同项目其他镜头。

这两条禁令只存在于我们自己写的提示词片段里，上游不会替我们守，所以 `skills_test.go` 有对应的门禁盯着。**入口技能 `hyperframes` 刻意不注册**：它的职责是「选择并安装 owning workflow」，会把渲染器往整片制作上带。

`internal/project/skilltree_test.go` 守着这条链：注册了但没随二进制下发、下发了但镜头目录找不到、技能目录缺 `SKILL.md`，都会失败。**这些失败必须修，不能豁免**——`ResolveSkill` 按设计在找不到技能时不报错只降级，没有这道门就会一路静默到成片质量变差才被发现。

fork 自外部项目的技能要带 `LICENSE` 和 `ATTRIBUTION.md`，后者记清上游 commit、保留了什么、丢了什么、改了哪几处，供日后同步上游时重做。`text-to-lottie` 的 `references/` 还有一道门禁止上游播放器契约（`Skottie`、`public/projects` 等）回流。

## 画幅预设

`assets/presets/<id>/` 下的**文本产物全部由 `go generate ./internal/preset/` 生成，不要手改**。真相源是 `internal/preset/table.go` 的预设表与 `internal/preset/gen/templates/` 下的模板。安全区在预设表里用 anchor 声明（贴顶 / 贴底 / 上下固定），构建期推导成绝对像素写进 `frame.md`——anchor 不进项目文件，渲染工具读到的始终是现成像素值。

改完预设表后必须重跑 `go generate` 并提交产物，CI 的 `generated-assets` job 会校验二者一致。

示例 SVG 是手绘的排版基准，生成器不改它们的内容；改完 SVG 用 `go run ./internal/preset/gen -examples -preset <id>` 更新 PNG。PNG 不参与 CI 幂等门——不同版本的 `rsvg-convert` 输出字节不同，纳进来会让门随 runner 随机失败。

SVG 转 PNG 必须走 `rsvg-convert`（`internal/styleimage`），**不能用 ImageMagick**：多数 ImageMagick 构建自带内置 XML SVG 渲染器，会抢在 rsvg 委托前接管 `.svg`，渲染不出文字也画不对背景填充，却返回退出码 0。

画幅只在 `am init` 时选定一次。新增画幅要动的地方：`internal/preset/table.go` 加一条、手绘 4 张该尺寸的示例 SVG、跑 `go generate` 与 `-examples`。`validate` 与 `scene.BuildPrompt` 都向预设表反查，不持有画幅常量，无需改动。

## Prompt 文件

`assets/shared/PROMPT.md` 和 `assets/presets/<id>/PROMPT-PRODUCTION.md` 是内置资产，不是本仓库的文档。它们随 `am init` 下发到项目根，被 `am run` 直接喂给编排工具。`PROMPT-PRODUCTION.md` 是生成产物，改它要改 `internal/preset/gen/templates/PROMPT-PRODUCTION.md`。

- 要像直接写给人类操作者的任务说明，不暴露执行者是 agent、Codex 或 AI，也不使用「主控 agent」这类身份设定；但可以明确说明他需要操作其他 AI。
- 不得出现 `./am` —— 用户项目里没有这个二进制，`am run` 通过 `PATH` 和 `AM_EXECUTABLE` 注入当前可执行文件。
- 这两条由 `internal/project/boundary_test.go` 的 `TestPromptsUseInstalledCLIAndDoNotExposeExecutorIdentity` 强制执行，违反会让测试失败。

## 媒体校验与拼接

`internal/mediaprobe` 是所有 ffprobe / ffmpeg 调用的唯一入口，别在其他包里直接拼这两个命令。

| 类型 | 作用 |
|---|---|
| `Spec` | 期望规格。**零值字段表示不检查**，调用方只想查一件事时不必填满全部字段 |
| `Spec.Check` | 返回**全部**不符项，不是第一条。每次只报一条会让调用方「修一条、重跑一次、再发现下一条」 |
| `Toolchain.Verify` | 完整校验流程，返回 `Report`。`error` 只表示「检查没跑成」，「跑了但不合格」是 `Report.OK == false` |
| `FrameStats` | 单帧灰度均值与标准差。`Blank()` 只看 `Flat()`，**不叠加 `Dark()`** |

空白帧阈值（`flatStdDev = 2.0`）刻意取得极保守。不能凭「真实帧标准差通常很高」来定：深色背景上只有一行标题时亮部占比不到 1%，标准差会低到 20 出头，那是合法封面。误判它比漏判一个真空白帧更糟——前者会让人很快不再信任这个检查。`TestFrameStatsDoesNotFlagSparseTextOnDarkBackground` 钉着这个上界。

`internal/concat` **刻意不提供重定时能力**。PROMPT 第八阶段写着「禁止通过 `setpts` 或改变帧率让镜头追赶配音」——提示词里的禁令可以被忽略，不存在的能力不能。所以分辨率或帧率不符归入 `Plan.Blocking` 要求重渲染，只有编码和像素格式差异走 `Plan.Normalize`。**不要为了「方便」给它加缩放或变速参数。**

镜头产物校验（`scene.VerifyOutput`）查画幅、帧率、时长、无音轨四项，不查编码和像素格式：PROMPT 第八阶段明确允许镜头产物规格不一致，由拼接前的规范化统一，在镜头层拦下会和那条既定流程打架。

校验失败返回 `*scene.VerificationError`，`cli` 把它包成 `schedule.Fatal` 停止重试。规格不符是确定性的，重跑只会得到同样的产物，而每次重试都是一次完整的 AI CLI 调用。**新增确定性失败时记得走同一条路。**

`mediaprobe` / `concat` 的集成测试需要真实 ffmpeg，缺失时自行跳过。CI 装了 ffmpeg——不装的话测试仍然"通过"但覆盖率掉到门禁线以下，真正跑外部命令的代码再没被验证过。

## 公开树边界

`TestPublicTreeContainsNoPrivateOrLegacyContent` 扫描 `git ls-files --cached --others --exclude-standard`，命中即失败。禁止出现的路径包括 `transcription.srt`、`final.mp4`、`publish.md`、`scenes/`、`production/`、`auto-test/`、`article_to_motion/`、`pyproject.toml`、`am`。

调试产生的单片内容留在仓库外，不要靠 `.gitignore` 遮掩——未跟踪文件同样会被这个测试扫到。

## 新增一个 AI CLI

工具矩阵散落在四个文件，加一个工具必须五处同步改，漏一处会在运行期才炸：

| 位置 | 作用 |
|---|---|
| `internal/tools/tools.go` `RendererInvocation` | 单镜头渲染的命令行与非交互权限模式 |
| `internal/tools/tools.go` `OrchestratorInvocation` | 编排调用的命令行与 prompt 传递方式（stdin 还是参数） |
| `internal/config/config.go` `validTools` | `ORCHESTRATOR` / `RENDERER` 取值校验 |
| `internal/scene/scene.go` `validRenderer` | `scene.json` 的 `renderer` 字段校验 |
| `internal/scene/skills.go` `rendererSkillDirs` | 该工具的技能目录约定（家目录级与项目级） |

同时更新 `assets/shared/article-to-motion.conf`、`assets/shared/.env.example`、`README.md`、`README.en.md` 里的可选值注释。

## 子进程环境

`Config.ChildEnvironment` 在安全模式下只透传白名单变量。`USER` / `LOGNAME` / `SHELL` 不是凭据但必须保留——macOS 上 claude CLI 缺 `USER` 就读不到 Keychain 登录态，报错却是「Not logged in」，与权限隔离毫无关联，极难排查。

新增白名单项要写清楚理由；需要临时透传由用户通过 `AM_PASSTHROUGH_ENV` 决定，不要在代码里放宽默认。
