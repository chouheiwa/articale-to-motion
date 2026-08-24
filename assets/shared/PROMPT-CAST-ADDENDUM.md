# 多角色叙事作业规程（cast 模式附录）

本附录只在项目根存在 `cast.yaml` 时生效——`cast.yaml` 存在与否就是叙事模式本身，不需要另外判断任何开关。它覆盖 `PROMPT-PRODUCTION.md` 第三阶段（TTS 和断句校对）与第四阶段（依据实际语音重建 SRT）里与"整段配音、单一说话人"相关的条款；其余阶段——稿件盘点、首帧封面、BGM/SFX、混音、最终验收、归档——不受影响，仍按 `PROMPT-PRODUCTION.md` 原文执行。

没有 `cast.yaml` 的单口播项目忽略本文件全部内容。

## 适用范围与生效条件

- 触发条件只有一个：项目根存在 `cast.yaml`。不存在则本附录不适用。
- 出场角色必须先通过 `am cast add <外部角色包目录>` 或 `am cast new <id>` 登记进 `cast.yaml` 的 `packs` 列表，才能出现在对话体脚本和 `scene.json` 的 `cast` 块里。
- `am init --narration cast` 建出的 `cast.yaml` 里 `packs` 是空列表——这是预期状态，不是错误。项目在第一次 `am cast add` / `am cast new` 之前本来就还没有可用角色，此时若强行渲染角色相关镜头会在角色装载阶段清楚报错，好过带着不存在的角色渲到一半才发现。

## 一、对话体脚本格式

终稿从单口播的自然段落改为对话体脚本：每一行一个说话人的一句或一段台词，格式为

```
<角色 id>: <台词>
```

角色 id 必须是已登记进 `cast.yaml` 的角色包 id（即 `cast/<id>/character.yaml` 里的 `id`）。示例：

```
heiwa: 今天我们来看看这个新功能到底解决了什么问题。
zhaocai: 喵，这个我熟，说来听听。
heiwa: 你先别急，我们从背景讲起。
```

- 出现未登记的角色 id 是脚本错误，必须先 `am cast add` / `am cast new` 引入角色再继续，不得先写脚本再补角色。
- 稿件审核关卡的检查项与单口播一致（预计朗读时长、开场全文、与原稿的主要变化、需要保护的专有名词和不可拆分短语），额外增加：各说话人的台词量是否严重失衡、每个角色的用词和语气是否符合人设。
- 稿件未确认时，不进入正式分段 TTS。

## 二、按说话人连续段分段，不按字幕行

分段单位是"同一说话人连续说的一段话"：只有说话人切换才起新段，一段内允许包含多句后续会拆成多条字幕的文本。

不按字幕行分段的理由：逐条字幕单独调一次 TTS，语调、呼吸和重音在人为切点处被打断，同一个人一整段连贯发言会被合成成一串听感割裂的短句拼接，是本附录反复强调的"语调碎裂"问题。整段合成才能让停顿、重音落在语义该在的地方。

- 说话人切换处必须切段，哪怕上下两句在内容上是一次紧密的问答呼应，也不能把两个说话人的台词合并进同一次 TTS 调用。
- 一段内部如果因为过长需要分批合成，仍必须保持"这一批只属于同一个说话人"，不得为了缩短单次请求而把说话人切换点藏进一段里。

## 三、逐段调用 mmx / bl，保存段内时间戳

TTS 服务提供方仍由 `article-to-motion.conf`（或环境变量 `TTS_PROVIDER`）决定，`mmx` / `bl` 两条路径的凭据、鉴权方式、密钥保管规则与 `PROMPT-PRODUCTION.md` 第三阶段完全一致，此处不重复；音色、语速试听确认，以及易误读词发音控制（`mmx` 的 `--pronunciation`、百炼 SSML `<phoneme>`）的要求也一致，只是作用范围从"整篇口播"缩小到"每一段"。

- 每个说话人使用其 `cast/<id>/character.yaml` 里 `voice.minimax.voiceId` 或 `voice.bailian.voiceId`（取决于当前 `TTS_PROVIDER`）对应的音色；同一说话人从头到尾使用同一个 voiceId，不得中途切换。
- 逐段合成时依旧按第三阶段的方式取真实时间戳：MiniMax 用 `--subtitles` 直接产出，百炼走合成 + `bl speech recognize` 两步识别。这里拿到的时间戳是相对本段音频起点的段内时间，不是全局时间轴——全局时间轴由 `am dialogue assemble` 统一计算，不要在这一步手工换算或累加偏移。
- 每段落盘为独立音频文件，建议命名 `production/audio/segments/<index>-<speaker>.wav`，`index` 从 0 开始，与后面 `plan.json` 里 `segments[].index` 一一对应。
- 断句检查、发音控制覆盖范围的要求与单口播一致，只是逐段各自核对：产品名/模型名是否被拆开、数字和英文缩写是否读对、长句是否留出呼吸、段尾是否留出画面展示时间。

## 四、写 `production/audio/plan.json`

`schema` 固定为 `cast-dialogue/v1`（与装配产物 `production/dialogue.json` 共用同一个 schema 标识）。结构示例：

```json
{
  "schema": "cast-dialogue/v1",
  "segments": [
    {
      "index": 0,
      "speaker": "heiwa",
      "voiceId": "voice-heiwa-01",
      "audio": "production/audio/segments/0-heiwa.wav",
      "gapAfterMs": 240,
      "lines": [
        { "text": "今天我们来看看这个新功能到底解决了什么问题。", "startSeconds": 0.0, "endSeconds": 2.8 }
      ]
    },
    {
      "index": 1,
      "speaker": "zhaocai",
      "voiceId": "voice-zhaocai-01",
      "audio": "production/audio/segments/1-zhaocai.wav",
      "gapAfterMs": 240,
      "lines": [
        { "text": "喵，这个我熟，说来听听。", "startSeconds": 0.0, "endSeconds": 1.6 }
      ]
    }
  ]
}
```

- `segments` 按最终播放顺序排列，`index` 从 0 递增且不留空、不重复。
- `speaker` 必须是 `cast.yaml` 登记班底里的角色 id。
- `lines[].startSeconds` / `endSeconds` 是本段音频内部的相对时间（来自上一步的段内时间戳），不是全局时间。
- `gapAfterMs` 是这一段结束到下一段开始之间要插入的静音时长，单位毫秒：常规轮换取 `cast.yaml` 的 `defaults.gap_ms.turn`，抢话/打断式的衔接取 `defaults.gap_ms.interject`。最后一段没有下一段，字段仍要填写，装配时会被忽略。

## 五、调用 `am dialogue assemble`

```
am dialogue assemble
```

- 默认读取 `production/audio/plan.json`，可用 `--plan` 指定别的路径。
- 产出三个文件：`production/audio/voice.wav`（拼接后的完整配音）、`transcription-production.srt`（按全局时间轴生成的字幕）、`production/dialogue.json`（装配后的结构化时间线——里面的 `segments` 与 `lines` 都已经是全局时间，不再是段内相对时间）。
- 命令内置两道漂移断言（逐段声明时长与实测时长的比对、总时长漂移比对），任一超出容差都会失败并点名具体是第几段——原样报告给用户，不要自己吞掉重试或手工拼凑时间轴掩盖问题。
- 装配完成后，`PROMPT-PRODUCTION.md` 第四阶段其余条款（短语级语义时间轴、`timing-report.json`、`closingHoldFrames` 等）照常执行，只是输入换成这里产出的 `production/dialogue.json` 与新的 `transcription-production.srt`。

## 六、拆镜头时给 `scene.json` 填 `cast` 块

拆镜头的分段规则（按语义合并、时长精度、总时长核对）与 `PROMPT-PRODUCTION.md` 第五阶段一致。每个有角色出场的镜头，在 `scene.json` 里追加 `cast` 块：

```json
{
  "cast": {
    "pack_dir": "../../cast",
    "ground_y": 0.78,
    "on_stage": [
      { "id": "heiwa", "x": 0.32, "pose": "idle", "facing": "right" },
      { "id": "zhaocai", "x": 0.68, "pose": "idle", "facing": "left" }
    ],
    "beats": [
      { "speaker": "heiwa", "start": 0.0, "end": 2.8 },
      { "speaker": "zhaocai", "start": 3.04, "end": 4.64 }
    ]
  }
}
```

- `pack_dir` 是相对本镜头目录、指向项目根 `cast/` 的路径；`on_stage` 列出本镜头台上的全部角色及初始站位（`x`，画面归一化比例）、初始姿势（`pose`）和朝向（`facing`）；`ground_y` 默认沿用 `cast.yaml` 的 `defaults.ground_y`，除非本镜头有特殊构图需要单独覆盖。
- **`beats[].start` / `end` 必须是镜头本地时间**（相对本镜头起点，单位秒），不是 `production/dialogue.json` 里的全局时间——从 `dialogue.json` 按本镜头覆盖的时间区间切片后，每一条都要减去镜头起点，才能得到镜头本地时间。
- `beats` 必须按 `start` 递增排列、互不重叠，且每一条的 `speaker` 必须出现在同一镜头的 `on_stage` 里；这两条会在渲染前被强制校验，不通过直接报错，不会带着非法节拍继续渲染。
- 没有角色出场的镜头（纯转场、纯 B-roll、纯图表）不写 `cast` 块，与老镜头完全一样。

## 七、角色包的引入

- 角色包通过 `am cast add <外部角色包目录>` 引入一个已存在的角色，或 `am cast new <id>` 现场生成一个新角色骨架（`character.yaml` + `rig.svg` + `dna.md`）后手工补全真实 `voiceId`、外观与人设。
- 两个命令成功后都会把角色顺带登记进项目根 `cast.yaml` 的 `packs` 列表，并在角色包目录里重新生成 `character.json`。
- 写对话体脚本之前，先确认脚本里会用到的每一个说话人 id 都已经登记；未登记的说话人既过不了 `am cast validate`，也过不了渲染前的镜头级校验。
- 手改 `cast/<id>/character.yaml` 之后要重新跑一次 `am cast validate` 才会同步 `character.json`；渲染机实际读取的是 `character.json`，不会因为 yaml 改过就自动感知。
