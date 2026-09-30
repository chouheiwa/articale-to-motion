---
schema_version: 1
style_id: scifi-hud-v1
style_name: 科幻HUD
scope:
  - aerospace_and_space_explainer
  - hardware_and_systems_architecture
  - engineering_mission_walkthrough
  - technology_explainer
  - diagnostics_and_monitoring
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
  canvas: "#0A1622"
  panel: "#0F2233"
  panel_raised: "#132C40"
  ink: "#E6F6FA"
  hud_cyan: "#3FE0F0"
  cyan_tint: "#0F3A48"
  readout_gray: "#8FB3C4"
  grid_line: "#16304A"
  line_dim: "#2E5A73"
  lock_teal: "#37D6B5"
  warning_amber: "#FFB547"
typography:
  primary_stack: '"Noto Sans SC", sans-serif'
  mono_stack: '"Orbitron", "JetBrains Mono", "Noto Sans SC", monospace'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "Orbitron", weight: 700, file: "assets/fonts/orbitron-variable.woff2"}
  sizes_px:
    cover_min: 78
    cover_max: 108
    title_min: 46
    title_max: 64
    body_min: 24
    body_max: 34
    label_min: 18
    label_max: 23
  weights:
    display: 900
    heading: 700
    body: 400
    body_emphasis: 600
    metadata: 700
  line_heights:
    display_min: 1.00
    display_max: 1.10
    body: 1.38
    metadata: 1.2
spacing:
  content_left_px: 120
  content_width_px: 1680
  group_min_px: 12
  group_max_px: 20
  card_stack_min_px: 16
  card_main_padding_min_px: 28
  card_main_padding_max_px: 36
  card_secondary_padding_min_px: 22
  card_secondary_padding_max_px: 28
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
    name: 目标锁定
    use_for: 封面、钩子、章节开场、强结论
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 扫描对比
    use_for: 两个对象的参数对照、旧方案与新方案、预期与实测
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 启动序列
    use_for: 步骤、发射或上线流程、因果链、状态切换
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 系统诊断
    use_for: 系统总览、子系统状态、能力清单、验证结果
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.30, breathe: 0.45, resolve: 0.25}
  entrance_seconds: {min: 0.25, max: 0.60}
  exit_seconds: {min: 0.15, max: 0.40}
  transition_seconds: {min: 0.15, max: 0.35}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: [power2.out, power3.out, expo.out]
  exit_ease: [power2.in, power3.in]
  verbs: [BOOT, DRAW, SCAN, TRACK, LOCK_ON, TICK, CONFIRM]
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
  background: "rgba(10, 22, 34, 0.86)"
  text_color: "#E6F6FA"
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
  font_size_px: {min: 78, max: 108}
  line_height: {min: 1.00, max: 1.10}
  max_width_px: 1600
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - copying_commercial_game_or_film_ui_logos_or_characters
  - arbitrary_new_brand_colors
  - warm_or_multi_hue_neon_palette
  - magenta_yellow_glitch_aesthetic
  - gradient_text
  - glow_blur_on_body_text
  - persistent_full_frame_flicker
  - excessive_glitch
  - elastic_or_bouncy_motion
  - decorative_fake_data_without_meaning
  - warning_amber_as_decoration
  - hairline_key_linework_under_2px
  - all_elements_same_direction_same_speed
  - labels_or_icons_touching_edges
  - skeleton_or_list_without_bottom_padding
  - sfx_on_every_element
  - music_masking_voice
---# 科幻HUD

这是航天、硬件、系统架构与任务式讲解类横屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择目标锁定、扫描对比、启动序列或系统诊断镜头。
- 渲染工具负责镜头内部的图形隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、圆角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景、中景和前景三个层次。
- 每一个读数、刻度、准星和状态灯都必须对应文案里的真实信息。没有语义的「假数据」只能作为极弱的背景纹理，不得抢主焦点。

## 品牌人格

冷静、精确、可控、机械、可信。画面像一台仪器正在开机、扫描目标、读取遥测并锁定结论：信息不是「飘进来」，而是被系统「检测到」。

## 视觉语言

- 深海军蓝画布 `canvas` 上用细而清晰的 `hud_cyan` 线条构建界面；面板用 `panel` / `panel_raised` 做实底，不做毛玻璃。
- 形状以直角、45° 切角、角标括号、圆环刻度、十字准星为主。圆角只用 2–8px，不做圆润卡片。
- 冷色单色系是主调；`warning_amber` 是唯一暖色，只用于警告、风险、异常读数和「需要注意」的那一个点。`lock_teal` 只表示正常、通过、已锁定。
- 关键线条至少 2px；1px 细线只用于背景网格与刻度纹理。
- 光晕只允许作用于线条和准星，不作用于正文；不得使用渐变文字。

## 构图

- 外层角标括号、边框刻度可进入 `structural` 区；关键内容不得进入该区。
- 普通画面使用 `main_content` 区。
- 标题、数字、结论和核心术语使用 `critical_text` 区，主动避让播放器底部控制栏与进度条。
- 主焦点通常是一个被锁定框、准星或仪表圆环框住的对象；读数面板围绕它排布，而不是均分成网格。

## 字体

- 中文一律使用 `Noto Sans SC`，正文不低于 24px。
- `Orbitron` 只用于拉丁字母与数字的短标签、读数、编号（如 `SYS 03`、`LOCK`、`98.6%`），不用于中文，也不用于长句。
- 代码、日志等长串英文可用 `JetBrains Mono`。

## 动画

遵循 Build / Breathe / Resolve。入场以 BOOT、DRAW、SCAN、TRACK、LOCK ON、TICK、CONFIRM 为主要动作词：描边绘制、扫描线扫过、数字滚动、准星收拢锁定。入场使用 ease-out，退场使用 ease-in；动作干脆、机械，不回弹。每镜头最多一种环境动作（例如一条缓慢的雷达扫线或一处呼吸状态灯），信息读完后允许画面完全静止地停留。

动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态；数字滚动与「闪烁」都必须由时间线驱动。

## 字幕与封面

字幕是可选层。若平台会另加字幕，可以关闭；若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则。

第 0 帧必须已经是完整封面：准星已锁定、标题已完整显示。标题稳定至少 18 帧后才允许转场，不得以黑帧、开机中间态或淡入中间态开场。

## 声音

人声优先且保持原速。BGM 只建立低频、稳定的仪器氛围；有人声时按 frontmatter 下压。SFX 只标记锁定、确认、警告和关键转场，同时最多两个，不给每条读数配音效。

## 可变项

题材、图形隐喻、仪表类型、读数数量、局部构图和内容素材可以变化。不得复制任何商业游戏、电影或剧集的界面、Logo、角色与标志性 HUD 布局；第三方品牌只能作为内容素材，不能改写本系列的核心 token。

## 验收

1. 核心颜色、字体、圆角和间距是否来自 frontmatter。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 关键文字是否避让平台 UI；中文正文是否不低于 24px。
4. 琥珀色是否只出现在真正的警告或异常上。
5. 标签、读数和角标距边缘是否至少 22px；列表与进度条底部是否至少留 22px。
6. 第 0 帧是否完成、可读且稳定 18 帧。
7. 动画是否可寻址、确定性、无同质化入场，是否只有一种环境动作。
8. 成片声音是否达到 loudness 和 true-peak 目标。


## 横屏构图要点

本项目是 16:9 横屏（1920×1080），画面的主方向是左右而不是上下。风格说明书里按竖屏描述的版式骨架，落到横屏时按下面的原则改排：

- 左右分栏优先于上下堆叠：命题型用「左文右图」或「左大字、右主视觉」；对照型直接左右对开；流程型沿水平方向展开，最多 5 步，超过就分两行；系统总览用横向网格。
- 不要把竖屏版式原样居中：中间一条窄栏、两侧大面积空白，是横屏最常见的「没排完」。
- 一行文字不超过 22 个汉字：横屏宽度大，单行过长会让视线来回扫，宁可断行。
- 标题字号以画面高度 1080px 为基准，与竖屏的同级字号保持一致，不因为画面变宽而放大。
- 底部控制栏与进度条是关键避让区：标题、数字、结论和字幕都不得进入 critical_text 下沿以下的区域；字幕带贴在进度条上方。
