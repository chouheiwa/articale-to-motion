---
schema_version: 1
style_id: whiteboard-doodle-v1
style_name: 白板手绘
scope:
  - tutorial_and_how_to
  - concept_breakdown
  - classroom_style_explainer
  - learning_methods_and_habits
  - science_and_business_basics
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
  canvas: "#FBFBF8"
  board_ghost: "#ECEEEA"
  ink: "#1D2327"
  marker_blue: "#1C5BB8"
  marker_red: "#C1312A"
  marker_green: "#23794A"
  highlighter: "#FFE26A"
  sticky_note: "#FFF1A6"
  blue_wash: "#E3ECF8"
  support_gray: "#596268"
  pencil_line: "#C9CED1"
typography:
  primary_stack: '"Xiaolai", "Noto Sans SC", sans-serif'
  mono_stack: '"Caveat", "Noto Sans SC", cursive'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "Xiaolai", weight: 400, file: "assets/fonts/xiaolai-400.woff2"}
    - {family: "Caveat", weight: 600, file: "assets/fonts/caveat-600.woff2"}
  sizes_px:
    cover_min: 84
    cover_max: 116
    title_min: 50
    title_max: 68
    body_min: 26
    body_max: 36
    label_min: 24
    label_max: 30
  weights:
    display: 400
    heading: 400
    body: 400
    body_emphasis: 600
    metadata: 600
  line_heights:
    display_min: 1.05
    display_max: 1.18
    body: 1.40
    metadata: 1.20
spacing:
  content_left_px: 120
  content_width_px: 1680
  group_min_px: 14
  group_max_px: 24
  card_stack_min_px: 20
  card_main_padding_min_px: 30
  card_main_padding_max_px: 40
  card_secondary_padding_min_px: 22
  card_secondary_padding_max_px: 30
  section_min_px: 36
  section_max_px: 64
  element_edge_min_px: 22
  skeleton_bottom_min_px: 22
radius:
  sm_px: 6
  md_px: 14
  lg_px: 24
  xl_px: 36
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 大观点
    use_for: 封面、钩子、章节开场、一句话结论
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: T 形对照
    use_for: 误区与正解、之前与之后、两种做法的取舍
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 箭头流程
    use_for: 步骤、因果链、循环、状态变化
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 思维导图
    use_for: 概念总览、知识结构、要素拆解、一镜收束全篇
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.40, breathe: 0.40, resolve: 0.20}
  entrance_seconds: {min: 0.40, max: 0.90}
  exit_seconds: {min: 0.20, max: 0.45}
  transition_seconds: {min: 0.25, max: 0.50}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: [power1.inOut, power2.out, sine.inOut]
  exit_ease: [power2.in, sine.in]
  verbs: [DRAW_ON, WRITE_ON, UNDERLINE, CIRCLE, HIGHLIGHT, POINT, STICK, ERASE]
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
  line_height: {min: 1.30, max: 1.40}
  background: "rgba(251, 251, 248, 0.92)"
  text_color: "#1D2327"
  padding_px: {vertical: 14, horizontal: 22}
  radius_px: 14
  break_rules:
    - keep_proper_nouns_intact
    - keep_english_phrases_intact
    - keep_numbers_and_units_intact
    - preserve_real_speech_gaps
cover:
  frame_zero_complete: true
  default_lines: 2
  max_lines: 3
  max_fullwidth_chars_per_line: 9
  font_size_px: {min: 84, max: 116}
  line_height: {min: 1.05, max: 1.18}
  max_width_px: 1600
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - runtime_random_stroke_jitter
  - boiling_line_on_every_element
  - drawing_hand_or_cursor_prop
  - clip_art_or_stock_icons
  - copied_branded_whiteboard_templates
  - stick_figure_characters
  - more_than_three_marker_colors_per_scene
  - colored_text_on_highlighter
  - gradient_or_glossy_fill
  - drop_shadow_on_strokes
  - elastic_or_bouncy_motion
  - all_elements_same_direction_same_speed
  - labels_or_icons_touching_edges
  - sfx_on_every_element
  - music_masking_voice
---# 白板手绘

这是教程、概念拆解和课堂式讲解类横屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择大观点、T 形对照、箭头流程或思维导图镜头。
- 渲染工具负责镜头内部的涂鸦隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景、中景和前景三个层次。
- 每一笔都要有语义：圈出的是关键词，箭头代表真实因果，下划线落在正在讲的那句话上。装饰性涂鸦只能极弱地存在于背景。

## 品牌人格

耐心、亲切、清楚、像一位在白板前边讲边画的老师。画面是「正在被画出来」的，而不是一张排好版的海报：先写字，再画框，最后用红笔圈出重点。

## 画面语言

- 背景层：`canvas` 白板底，可叠一层 `board_ghost` 残留擦痕；不画网格，不做纸张纹理照片。
- 中景层：`ink` 黑色马克笔写的标题与正文、手绘框、便利贴（`sticky_note`）和淡蓝底块（`blue_wash`）。
- 前景层：红笔圈、蓝色箭头、`highlighter` 荧光笔涂抹、绿色勾选，只用来指认当前焦点。
- 笔触是有轻微抖动的路径，线宽 4–8px，`stroke-linecap: round`、`stroke-linejoin: round`。抖动必须写死在路径坐标里，禁止运行时随机。
- 每个镜头最多使用三种马克笔色（`ink` 之外最多再从蓝、红、绿中选两种；荧光笔、便利贴、淡蓝底块是底色，不计入）。红色只表示否定、错误和「这里是重点」，绿色只表示正确、完成和通过。
- 荧光笔只垫在 `ink` 文字下；彩色文字不得压在荧光笔上。

## 构图

- 外层擦痕、页角涂鸦可进入 `structural` 区；关键内容不得进入该区。
- 普通画面使用 `main_content` 区。
- 标题、数字、结论和核心术语使用 `critical_text` 区，主动避让播放器底部控制栏与进度条。
- 中文手写体 `Xiaolai` 用于标题、正文和标签；超过三行的密集说明改用 `Noto Sans SC`。英文批注、数字小注用 `Caveat`。中文正文不得小于 24px。

## 动画

遵循 Build / Breathe / Resolve。主要动作词为 DRAW_ON、WRITE_ON、UNDERLINE、CIRCLE、HIGHLIGHT、POINT、STICK、ERASE：线条用 `strokeDashoffset` 画出，文字用逐字或自左向右遮罩写出，荧光笔以一次横扫铺开。节奏平稳，像手在写，不弹跳、不甩动。

Breathe 阶段允许画面完全静止，让观众读完；如需环境动作，每镜头最多一种（例如一支箭头轻微呼吸）。

动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。不画手或笔的道具跟随笔尖。

## 字幕与封面

字幕是可选层。若平台会另加字幕，可以关闭；若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则，使用白板底色半透明底与 `ink` 文字。

第 0 帧必须已经是完整封面：标题、圈注和荧光笔全部画完。标题稳定至少 18 帧后才允许转场，不得以黑帧、空白或「正在书写」的中间态开场。

## 声音

人声优先且保持原速。BGM 轻快、木质、低存在感；有人声时按 frontmatter 下压。SFX 只用于关键的落笔、圈出、贴便利贴和结论，可以用极轻的马克笔摩擦声，同时最多两个，不给每一笔配音。

## 验收

1. 核心颜色、字体和间距是否来自 frontmatter；单镜头马克笔色是否不超过三种。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 关键文字是否避让平台 UI；标签、便利贴和图标距边缘至少 22px。
4. 手绘抖动是否写死在路径里，任意帧重建结果一致。
5. 第 0 帧是否完成、可读且稳定 18 帧。
6. 成片声音是否达到 loudness 和 true-peak 目标。


## 横屏构图要点

本项目是 16:9 横屏（1920×1080），画面的主方向是左右而不是上下。风格说明书里按竖屏描述的版式骨架，落到横屏时按下面的原则改排：

- 左右分栏优先于上下堆叠：命题型用「左文右图」或「左大字、右主视觉」；对照型直接左右对开；流程型沿水平方向展开，最多 5 步，超过就分两行；系统总览用横向网格。
- 不要把竖屏版式原样居中：中间一条窄栏、两侧大面积空白，是横屏最常见的「没排完」。
- 一行文字不超过 22 个汉字：横屏宽度大，单行过长会让视线来回扫，宁可断行。
- 标题字号以画面高度 1080px 为基准，与竖屏的同级字号保持一致，不因为画面变宽而放大。
- 底部控制栏与进度条是关键避让区：标题、数字、结论和字幕都不得进入 critical_text 下沿以下的区域；字幕带贴在进度条上方。
