---
schema_version: 1
style_id: editorial-magazine-v1
style_name: 杂志编辑排版
scope:
  - opinion_essay
  - humanities_and_culture
  - business_commentary
  - books_and_history
  - long_form_reading
canvas:
  width_px: 1920
  height_px: 1080
  fps: 30
  orientation: landscape
safe_area:
  structural: {left_px: 48, right_px: 48, top_px: 40, bottom_px: 40}
  main_content: {left_px: 120, right_px: 120, top_px: 80, bottom_px: 96}
  critical_text: {left_px: 160, right_px: 160, top_px: 96, bottom_px: 200}
  cover_title: {left_px: 160, right_px: 160, top_px: 220, bottom_px: 340}
  subtitles: {left_px: 240, right_px: 240, top_px: 840, bottom_px: 90}
colors:
  canvas: "#F4EFE6"
  paper_shade: "#EAE3D5"
  paper_light: "#FBF8F2"
  ink: "#1A1714"
  body_ink: "#4A433B"
  caption_gray: "#6B6155"
  vermilion: "#A82E1C"
  vermilion_tint: "#F2D9CF"
  vermilion_on_ink: "#E0664F"
  hairline: "#CBC1AF"
  hairline_strong: "#9C9080"
typography:
  primary_stack: '"Noto Serif SC", serif'
  mono_stack: '"Noto Sans SC", sans-serif'
  display_stack: '"Playfair Display", "Noto Serif SC", serif'
  font_files:
    - {family: "Noto Serif SC", weight: 400, file: "assets/fonts/noto-serif-sc-400.woff2"}
    - {family: "Noto Serif SC", weight: 700, file: "assets/fonts/noto-serif-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
  sizes_px:
    cover_min: 84
    cover_max: 120
    title_min: 50
    title_max: 68
    body_min: 26
    body_max: 34
    label_min: 24
    label_max: 28
  weights:
    display: 700
    heading: 700
    body: 400
    body_emphasis: 700
    metadata: 600
  line_heights:
    display_min: 1.10
    display_max: 1.20
    body: 1.60
    metadata: 1.30
spacing:
  content_left_px: 120
  content_width_px: 1680
  group_min_px: 14
  group_max_px: 24
  card_stack_min_px: 24
  card_main_padding_min_px: 32
  card_main_padding_max_px: 44
  card_secondary_padding_min_px: 24
  card_secondary_padding_max_px: 32
  section_min_px: 40
  section_max_px: 72
  element_edge_min_px: 22
  skeleton_bottom_min_px: 22
radius:
  sm_px: 0
  md_px: 2
  lg_px: 4
  xl_px: 6
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 封面故事
    use_for: 封面、钩子、章节开篇、一句定调的强观点
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 双栏对照
    use_for: 两种观点、旧说与新说、表象与本质、正方与反方
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 编号长文
    use_for: 分节论证、时间线、因果链、逐层推进的论点
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 目录页
    use_for: 全片导读、章节总览、要点汇总、结语回顾
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.35, breathe: 0.45, resolve: 0.20}
  entrance_seconds: {min: 0.50, max: 1.10}
  exit_seconds: {min: 0.30, max: 0.60}
  transition_seconds: {min: 0.40, max: 0.80}
  first_action_delay_seconds: {min: 0.20, max: 0.40}
  entrance_ease: [power2.out, power3.out, sine.inOut]
  exit_ease: [power2.in, sine.in]
  verbs: [SET_LINE, DRAW_RULE, MASK_WIPE, DROP_CAP, PULL_QUOTE, MARK, TURN_PAGE]
  ambient_per_scene_max: 1
  max_same_ease_tweens: 3
audio:
  sample_rate_hz: 48000
  channels: 2
  voice_integrated_lufs: {min: -18, max: -16}
  bgm_integrated_lufs: {min: -30, max: -26}
  bgm_ducking_db: {min: 6, max: 10}
  ducking_attack_seconds: {min: 0.04, max: 0.12}
  ducking_release_seconds: {min: 0.25, max: 0.60}
  bgm_fade_seconds: {min: 0.80, max: 1.50}
  sfx_gain_db: {min: -18, max: -12}
  sfx_max_simultaneous: 2
  final_integrated_lufs: -16
  final_lufs_tolerance: 1
  true_peak_max_dbtp: -1
  lra_lu: {min: 2, max: 6}
subtitles:
  required: false
  font_size_px: {min: 32, max: 38}
  max_width_px: 1440
  max_lines: 2
  max_fullwidth_chars_per_line: 16
  line_height: {min: 1.35, max: 1.45}
  background: "rgba(26, 23, 20, 0.86)"
  text_color: "#F4EFE6"
  padding_px: {vertical: 14, horizontal: 24}
  radius_px: 2
  break_rules:
    - keep_proper_nouns_intact
    - keep_english_phrases_intact
    - keep_numbers_and_units_intact
    - keep_quotation_marks_with_quote
    - preserve_real_speech_gaps
cover:
  frame_zero_complete: true
  default_lines: 2
  max_lines: 3
  max_fullwidth_chars_per_line: 9
  font_size_px: {min: 84, max: 120}
  line_height: {min: 1.10, max: 1.20}
  max_width_px: 1600
  contrast_ratio_min: 7.0
  stable_frames: 24
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - real_magazine_masthead_or_logo
  - imitated_publication_trade_dress
  - more_than_one_accent_color
  - pure_black_or_pure_white_page
  - gradient_text
  - glow_or_neon_effects
  - drop_shadow_cards
  - rounded_web_card_stack
  - elastic_or_bouncy_motion
  - per_character_typewriter_on_body_text
  - all_lines_same_direction_same_speed
  - empty_lower_half
  - justified_text_with_rivers
  - body_text_below_24px
  - sfx_on_every_line
  - music_masking_voice
---

# 杂志编辑排版

这是观点、人文与商业评论类横屏视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择封面故事、双栏对照、编号长文或目录页镜头。
- 渲染工具负责镜头内部的版面切分、引文取舍和动画编排，但不得改写核心颜色、字体角色、安全区、网格、间距、圆角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含纸面、版式结构和正文三个层次。
- 版面由文字本身建立秩序：标题、导语、细线、编号、引文。不要用图标堆叠或卡片矩阵代替排版。

## 品牌人格

沉静、有观点、讲究、可信。画面像一页正在被排版的杂志内页：标题落定，细线划开版面，引文被拎出来，读者被一行一行带进论证。

## 色彩

- 纸面 `canvas` 与墨色 `ink` 构成全片基调；禁止纯黑与纯白页面。
- 朱红 `vermilion` 是唯一强调色，只用于章节编号、首字下沉、引文符号、关键词下划线和一处结论标记。每镜头朱红面积不超过画面的 5%。
- `vermilion_tint` 只作荧光笔式的关键词底衬；`vermilion_on_ink` 只在墨色底板上替代朱红。
- 不得引入第二个强调色。否定、风险等语义用删除线、灰度或朱红细线表达，而不是新颜色。

## 字体

- 标题、导语、引文、正文：`Noto Serif SC`（400 / 700）。拉丁字母标题可用自动内嵌的 `Playfair Display`，中文必须落到 `Noto Serif SC`。
- 栏目眉题、图注、页码、署名等元数据：`Noto Sans SC`，600 或 700，字距 2–4px。
- 中文正文不小于 24px；标题行距 1.10–1.20，正文行距约 1.60。
- 首字下沉占三行正文高度，只在每镜头第一段使用一次。

## 构图与网格

- 关键文字区宽 1600px，按栏距 24px、栏宽约 180px 分栏（竖屏四栏、横屏八栏），左轴 160px。标题、导语、正文、细线都贴栏线。
- 关键文字区右侧到主内容区右缘之间是外侧页边，只放页码、边注编号、竖排栏目名等非关键元素。
- 标题、数字、结论和引文放在 `critical_text` 区，主动避让播放器底部控制栏与进度条。
- 留白是版面的一部分，但画面下半部不得整片空置：用导语、引文、署名行或页脚细线收住版面。
- 细线分两级：`hairline` 1.5px 用于栏间与段落分隔，`ink` 3–4px 粗线只用于版头与章节起点。

## 动画

遵循 Build / Breathe / Resolve。以 SET LINE（逐行上移出遮罩）、DRAW RULE（细线从左轴画出）、MASK WIPE（遮罩擦出）、DROP CAP、PULL QUOTE、MARK（下划线或底衬划出）、TURN PAGE（整版平移换页）为主要动作词。入场使用缓慢的 ease-out，退场使用 ease-in；按阅读顺序而非 DOM 顺序编排。每镜头最多一种环境动作；允许画面在完成后完全静止停留。

禁止弹跳、回弹、发光、逐字打字机式正文、所有行同向同速入场。动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则，引号内的引文不拆开。

第 0 帧必须已经是完整封面。标题稳定至少 24 帧后才允许转场，不得以黑帧、空白或淡入中间态开场。

## 声音

人声优先且保持原速。BGM 选用钢琴、弦乐或低饱和电子氛围，有人声时按 frontmatter 下压。SFX 只标记翻页、章节起点和结论落定，轻柔的纸张声优先，同时最多两个。

## 可变项

题材、引文内容、栏目名、图片素材、局部版面切分可以变化。不得复刻任何真实杂志的刊头、Logo、栏目标识或版式商标；真实出版物只能作为被引用的内容素材出现。

## 验收

1. 核心颜色、字体、圆角和间距是否来自 frontmatter，全片是否只有朱红一个强调色。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 标题、正文、细线是否贴齐栏网格与左轴。
4. 关键文字是否避让平台 UI；页码、边注距边缘是否至少 22px。
5. 画面下半部是否有内容收束，而非整片空白。
6. 第 0 帧是否完成、可读且稳定 24 帧。
7. 动画是否可寻址、确定性、无弹跳和发光。
8. 成片声音是否达到 loudness 和 true-peak 目标。


## 横屏构图要点

本项目是 16:9 横屏（1920×1080），画面的主方向是左右而不是上下。风格说明书里按竖屏描述的版式骨架，落到横屏时按下面的原则改排：

- 左右分栏优先于上下堆叠：命题型用「左文右图」或「左大字、右主视觉」；对照型直接左右对开；流程型沿水平方向展开，最多 5 步，超过就分两行；系统总览用横向网格。
- 不要把竖屏版式原样居中：中间一条窄栏、两侧大面积空白，是横屏最常见的「没排完」。
- 一行文字不超过 22 个汉字：横屏宽度大，单行过长会让视线来回扫，宁可断行。
- 标题字号以画面高度 1080px 为基准，与竖屏的同级字号保持一致，不因为画面变宽而放大。
- 底部控制栏与进度条是关键避让区：标题、数字、结论和字幕都不得进入 critical_text 下沿以下的区域；字幕带贴在进度条上方。
