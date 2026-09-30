---
schema_version: 1
style_id: esports-broadcast-v1
style_name: 电竞赛事转播
scope:
  - head_to_head_comparison
  - product_and_tool_review
  - rankings_and_leaderboards
  - data_recap_and_scorecards
  - competitive_timeline_explainer
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
  canvas: "#0B0F1A"
  panel: "#141B2D"
  ink: "#F2F5FA"
  support_gray: "#8C9BB5"
  structure_line: "#26314A"
  team_blue: "#2F8CFF"
  team_blue_fill: "#1558C0"
  team_red: "#FF3B4E"
  team_red_fill: "#B8182E"
  highlight_gold: "#FFC23D"
  live_green: "#3DDC84"
typography:
  primary_stack: '"Noto Sans SC", sans-serif'
  mono_stack: '"Oxanium", "Noto Sans SC", monospace'
  display_stack: '"Bebas Neue", "Noto Sans SC", sans-serif'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "Bebas Neue", weight: 400, file: "assets/fonts/bebas-neue-400.woff2"}
    - {family: "Oxanium", weight: 600, file: "assets/fonts/oxanium-600.woff2"}
  sizes_px:
    cover_min: 80
    cover_max: 112
    title_min: 46
    title_max: 64
    numeral_min: 96
    numeral_max: 220
    body_min: 24
    body_max: 34
    label_min: 20
    label_max: 26
  weights:
    display: 900
    heading: 700
    body: 400
    body_emphasis: 600
    metadata: 600
    numeral: 400
  line_heights:
    display_min: 1.00
    display_max: 1.08
    numeral: 0.90
    body: 1.36
    metadata: 1.2
spacing:
  content_left_px: 120
  content_width_px: 1680
  group_min_px: 12
  group_max_px: 20
  card_stack_min_px: 12
  card_main_padding_min_px: 28
  card_main_padding_max_px: 36
  card_secondary_padding_min_px: 20
  card_secondary_padding_max_px: 26
  section_min_px: 32
  section_max_px: 56
  element_edge_min_px: 22
  skeleton_bottom_min_px: 22
radius:
  sm_px: 2
  md_px: 4
  lg_px: 6
  xl_px: 8
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: MVP 卡
    use_for: 封面、钩子、章节开场、单点强结论与关键数字
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 双方对位
    use_for: 两种方案、两个产品、旧做法与新做法的逐项对位
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 比赛时间线
    use_for: 步骤、阶段、时间节点、局势转折
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 积分榜
    use_for: 排行、多项打分、多方案横评、最终战报
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.25, breathe: 0.50, resolve: 0.25}
  entrance_seconds: {min: 0.24, max: 0.50}
  exit_seconds: {min: 0.16, max: 0.36}
  transition_seconds: {min: 0.18, max: 0.36}
  first_action_delay_seconds: {min: 0.10, max: 0.24}
  entrance_ease: [power4.out, expo.out, power3.out]
  exit_ease: [power4.in, expo.in]
  verbs: [WIPE, SLAM, SPLIT, TICK, STACK, FLAG, CROWN]
  ambient_per_scene_max: 1
  max_same_ease_tweens: 2
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
  line_height: {min: 1.30, max: 1.40}
  background: "rgba(11, 15, 26, 0.88)"
  text_color: "#F2F5FA"
  padding_px: {vertical: 14, horizontal: 22}
  radius_px: 4
  break_rules:
    - keep_proper_nouns_intact
    - keep_english_phrases_intact
    - keep_numbers_and_units_intact
    - preserve_real_speech_gaps
cover:
  frame_zero_complete: true
  default_lines: 2
  max_lines: 3
  max_fullwidth_chars_per_line: 10
  font_size_px: {min: 80, max: 112}
  line_height: {min: 1.00, max: 1.08}
  max_width_px: 1600
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - real_league_team_game_or_sponsor_marks
  - copied_broadcast_package_layouts
  - fake_betting_odds_or_gambling_ui
  - arbitrary_new_team_colors
  - colors_outside_palette_tokens
  - gradient_text
  - neon_purple_blue_gradient
  - motion_blur_or_smear_frames
  - excessive_glitch_or_rgb_split
  - camera_shake_on_text
  - elastic_or_bouncy_motion
  - text_moving_during_hold
  - continuous_ticker_crawl_behind_body
  - all_elements_same_direction_same_speed
  - labels_or_icons_touching_edges
  - skeleton_or_list_without_bottom_padding
  - sfx_on_every_element
  - crowd_roar_or_airhorn_over_voice
  - music_masking_voice
---# 电竞赛事转播

这是对比评测、排行榜与数据战报类横屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。画面借用赛事转播包装的视觉语法——记分牌、斜切下三分之一条、VS 分屏、MVP 卡、积分榜——但讲的是任意题材里「两方或多方的较量」。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择 MVP 卡、双方对位、比赛时间线或积分榜镜头。
- 渲染工具负责镜头内部的图形隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、圆角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景、中景和前景三个层次。
- 「队伍」只是比喻。两种方案、两款产品、两种做法都可以是蓝方与红方；不要虚构赔率、下注或真实战队。

## 品牌人格

利落、有对抗感、数据说话、节奏快但读得清。画面像一场刚打完的比赛的转播包装：比分被揭晓，双方被对位，关键时刻被标出，冠军被加冕。

## 颜色与字体角色

- 画布 `canvas` 为深藏青黑，面板 `panel` 承载所有信息条；不做纯黑，不做大面积渐变。
- `team_blue` / `team_red` 是两方阵营色，只标识归属，不作装饰轮换。填色块上放白字时改用 `team_blue_fill` / `team_red_fill`，保证对比度。
- `highlight_gold` 只给胜者、MVP、第一名和最终结论。`live_green` 只表示「进行中 / 通过」。
- 中文标题与正文用 `Noto Sans SC`（封面与大标题 900）。拉丁大数字、比分、队名缩写用 `display_stack`（`Bebas Neue`）；计时器、局数、元数据用 `mono_stack`（`Oxanium`）。`Bebas Neue` 与 `Oxanium` 没有中文字形，不得用来排中文。

## 构图

- 外层角标、斜切装饰线可进入 `structural` 区；关键内容不得进入该区。
- 普通画面使用 `main_content` 区；标题、比分、数字和结论使用 `critical_text` 区，主动避让底部控制栏与进度条。
- 形状语言是平行四边形与斜切角：信息条左端或右端切 12–18° 斜角，圆角只用 2–8px。胶囊只留给状态标签。
- 双方对位时左蓝右红，中轴放 VS 或比分；不做左右完全对称的镜像排版，让胜方一侧略重。

## 动画

遵循 Build / Breathe / Resolve。入场以 WIPE、SLAM、SPLIT、TICK、STACK、FLAG、CROWN 为主要动作词，使用 power4 / expo 的硬减速：快速到位、瞬间刹停，不带运动模糊、不回弹。入场使用 ease-out，退场使用 ease-in；按信息优先级而非 DOM 顺序编排。

文字一旦到位就保持静止：Breathe 阶段不抖动、不漂移、不闪烁。每镜头最多一种环境动作（例如背景斜纹缓移或 LIVE 点慢闪），数字滚动（TICK）只在揭晓时发生一次。章节之间允许一条 0.18–0.36s 的斜切色块转场（stinger），完全遮挡的帧不超过 3 帧。

动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若平台会另加字幕，可以关闭；若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则。

第 0 帧必须已经是完整封面：比分或主判断已经在位，不能等数字滚完才看懂。标题稳定至少 18 帧后才允许转场，不得以黑帧、空白、半截擦除或淡入中间态开场。

## 声音

人声优先且保持原速。BGM 可以更有推进感，但有人声时按 frontmatter 下压。SFX 只标记揭晓、对位、关键转折和加冕，同时最多两个；不用人群欢呼、气笛喇叭或解说员采样。

## 可变项

题材、双方身份、比分口径、队标图形（自绘几何徽记）、条目数量和局部构图可以变化。真实联赛、战队、游戏与赞助商标识一律不得出现；第三方产品 Logo 只能作为内容素材，不能改写本系列的核心 token。

## 验收

1. 核心颜色、字体、圆角和间距是否来自 frontmatter；阵营色是否只标识归属。
2. `typography.font_files` 里用到的每个字重（含 `Bebas Neue` 400、`Oxanium` 600）是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 中文是否全部落在 `Noto Sans SC`，正文不小于 24px。
4. 关键文字是否避让平台 UI；标签、状态和图标距边缘是否至少 22px；列表与积分榜底部是否至少留 22px。
5. 第 0 帧是否完成、可读且稳定 18 帧。
6. 文字到位后是否静止；动画是否可寻址、确定性、无回弹、无运动模糊。
7. 成片声音是否达到 loudness 和 true-peak 目标。


## 横屏构图要点

本项目是 16:9 横屏（1920×1080），画面的主方向是左右而不是上下。风格说明书里按竖屏描述的版式骨架，落到横屏时按下面的原则改排：

- 左右分栏优先于上下堆叠：命题型用「左文右图」或「左大字、右主视觉」；对照型直接左右对开；流程型沿水平方向展开，最多 5 步，超过就分两行；系统总览用横向网格。
- 不要把竖屏版式原样居中：中间一条窄栏、两侧大面积空白，是横屏最常见的「没排完」。
- 一行文字不超过 22 个汉字：横屏宽度大，单行过长会让视线来回扫，宁可断行。
- 标题字号以画面高度 1080px 为基准，与竖屏的同级字号保持一致，不因为画面变宽而放大。
- 底部控制栏与进度条是关键避让区：标题、数字、结论和字幕都不得进入 critical_text 下沿以下的区域；字幕带贴在进度条上方。
