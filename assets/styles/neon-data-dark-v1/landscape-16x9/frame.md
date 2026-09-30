---
schema_version: 1
style_id: neon-data-dark-v1
style_name: 暗夜数据霓虹
scope:
  - ai_and_frontier_technology
  - data_storytelling
  - metrics_and_benchmarks
  - technology_explainer
  - business_and_industry_trends
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
  canvas: "#070A12"
  surface: "#0E1422"
  surface_raised: "#151D2F"
  ink: "#EEF2FA"
  muted_ink: "#9AA6BF"
  grid_line: "#1C2538"
  axis_line: "#5F6B85"
  signal_lime: "#C8F560"
  lime_tint: "#1B2A10"
  pulse_violet: "#A493FF"
  alert_coral: "#FF7A6B"
typography:
  primary_stack: '"Noto Sans SC", sans-serif'
  mono_stack: '"Space Grotesk", "JetBrains Mono", "Noto Sans SC", monospace'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "Space Grotesk", weight: 500, file: "assets/fonts/space-grotesk-500.woff2"}
    - {family: "Space Grotesk", weight: 700, file: "assets/fonts/space-grotesk-700.woff2"}
  sizes_px:
    cover_min: 80
    cover_max: 112
    hero_number_min: 160
    hero_number_max: 280
    title_min: 46
    title_max: 64
    body_min: 24
    body_max: 34
    label_min: 18
    label_max: 23
  weights:
    display: 900
    heading: 700
    number: 700
    body: 400
    body_emphasis: 600
    metadata: 500
  line_heights:
    display_min: 1.00
    display_max: 1.10
    number: 0.92
    body: 1.40
    metadata: 1.20
spacing:
  content_left_px: 120
  content_width_px: 1680
  group_min_px: 12
  group_max_px: 20
  card_stack_min_px: 16
  card_main_padding_min_px: 32
  card_main_padding_max_px: 40
  card_secondary_padding_min_px: 22
  card_secondary_padding_max_px: 28
  section_min_px: 40
  section_max_px: 64
  element_edge_min_px: 22
  skeleton_bottom_min_px: 22
radius:
  sm_px: 6
  md_px: 12
  lg_px: 16
  xl_px: 24
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 巨型指标
    use_for: 封面、钩子、一个决定性数字、强结论
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 双线对比
    use_for: 两条趋势、前后对照、两种方案的指标差距
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 漏斗流程
    use_for: 转化漏斗、逐级筛选、管线各阶段的损耗与留存
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 指标看板
    use_for: 多指标总览、系统健康度、评测成绩单、阶段性结果
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.35, breathe: 0.40, resolve: 0.25}
  entrance_seconds: {min: 0.40, max: 0.90}
  exit_seconds: {min: 0.20, max: 0.45}
  transition_seconds: {min: 0.25, max: 0.45}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: [expo.out, power3.out, power2.out]
  exit_ease: [power2.in, power3.in]
  verbs: [COUNT_UP, DRAW_LINE, GROW, REVEAL, NARROW, HIGHLIGHT, SETTLE]
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
  background: "rgba(14, 20, 34, 0.88)"
  text_color: "#EEF2FA"
  padding_px: {vertical: 14, horizontal: 22}
  radius_px: 12
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
  line_height: {min: 1.00, max: 1.10}
  max_width_px: 1600
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - count_up_unfinished_on_frame_zero
  - pure_black_canvas
  - colors_outside_palette_tokens
  - gradient_text
  - rainbow_multi_series_palette
  - glow_on_body_text
  - stacked_heavy_glow_or_bloom
  - chartjunk_3d_charts_or_pie_explosion
  - truncated_axis_without_marker
  - fake_precision_decimals
  - unlabelled_illustrative_numbers
  - copying_brand_dashboards
  - glitch_or_scanline_effects
  - elastic_or_bouncy_motion
  - endless_number_ticking
  - all_elements_same_direction_same_speed
  - labels_or_icons_touching_edges
  - sfx_on_every_element
  - music_masking_voice
---

# 暗夜数据霓虹

这是 AI、前沿技术与数据类横屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择巨型指标、双线对比、漏斗流程或指标看板镜头。
- 渲染工具负责镜头内部的图表形式、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、圆角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景网格、中景数据面板和前景读数三个层次。
- 数字先于装饰。每一个发光的元素都必须是一个值得被看见的数据点。

## 品牌人格

冷静、精确、有洞察、克制的未来感。画面像一块深夜仍在运行的数据大屏：暗底安静，只有关键数字与关键曲线亮起。

## 色彩

- 底色 `canvas` 是带蓝调的近黑，不用纯黑。面板用 `surface` / `surface_raised` 分层，不用阴影堆高。
- 全片只有两种强调色：`signal_lime` 表示主数据、当前、增长；`pulse_violet` 表示对照组、基线、第二序列。`alert_coral` 只表示下降、损耗、风险。
- 辉光只给数据标记（主数字、曲线端点、当前柱），不给正文。

## 字体

- 中文一律 `Noto Sans SC`；数字、单位、英文标签与坐标读数用 `Space Grotesk`（500 / 700）。
- 巨型数字 160–280px、字重 700、行高约 0.92；数字与单位分开排版，单位缩到数字的 30–40%。
- 中文正文不小于 24px，不加辉光，不用渐变文字。

## 构图

- 外层网格与角标可进入 `structural` 区；关键内容不得进入该区。
- 普通图表与面板使用 `main_content` 区。
- 主数字、标题、结论和图例使用 `critical_text` 区，主动避让播放器底部控制栏与进度条。
- 图表必须有基线、刻度或来源注记之一；示意数据必须标注「示意」。

## 数据诚实

- 坐标轴从零开始，或者用明显的断轴符号标出截断。
- 小数位与数据真实精度一致；不为了显得精确而加小数。
- 同一镜头内比较的数值用同一单位、同一刻度。
- 不照搬任何真实品牌的仪表盘、Logo 或配色。

## 动画

遵循 Build / Breathe / Resolve。入场以 COUNT UP、DRAW LINE、GROW、REVEAL、NARROW、HIGHLIGHT、SETTLE 为主要动作词，入场首选 `expo.out`，退场使用 ease-in；按信息优先级而非 DOM 顺序编排。

- 数字滚动在入场时长内结束并定格，不得持续跳动。
- 曲线描线从左到右，端点在描线结束后才亮起。
- 每镜头最多一种环境动作（例如背景网格极慢漂移或端点呼吸光），允许完全静止的稳定停留。

动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若平台会另加字幕，可以关闭；若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则。

第 0 帧必须已经是完整封面：主数字已经是最终值，不得从 0 开始滚动。标题稳定至少 18 帧后才允许转场，不得以黑帧、空白或淡入中间态开场。

## 声音

人声优先且保持原速。BGM 用低频脉冲或柔和合成垫底；有人声时按 frontmatter 下压。SFX 只标记钩子、数字定格、关键转折和结论，同时最多两个；数字滚动不配连续的滴答声。

## 验收

1. 核心颜色、字体、圆角和间距是否来自 frontmatter，强调色是否只有两种。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 关键文字是否避让平台 UI，标签与图标距边缘是否至少 22px。
4. 图表是否有基线或断轴标记，示意数据是否标注。
5. 第 0 帧是否完成、可读且稳定 18 帧。
6. 动画是否可寻址、确定性、无同质化入场；成片声音是否达到 loudness 和 true-peak 目标。


## 横屏构图要点

本项目是 16:9 横屏（1920×1080），画面的主方向是左右而不是上下。风格说明书里按竖屏描述的版式骨架，落到横屏时按下面的原则改排：

- 左右分栏优先于上下堆叠：命题型用「左文右图」或「左大字、右主视觉」；对照型直接左右对开；流程型沿水平方向展开，最多 5 步，超过就分两行；系统总览用横向网格。
- 不要把竖屏版式原样居中：中间一条窄栏、两侧大面积空白，是横屏最常见的「没排完」。
- 一行文字不超过 22 个汉字：横屏宽度大，单行过长会让视线来回扫，宁可断行。
- 标题字号以画面高度 1080px 为基准，与竖屏的同级字号保持一致，不因为画面变宽而放大。
- 底部控制栏与进度条是关键避让区：标题、数字、结论和字幕都不得进入 critical_text 下沿以下的区域；字幕带贴在进度条上方。
