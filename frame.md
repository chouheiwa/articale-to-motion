---
schema_version: 1
style_id: clear-system-blueprint-v1
style_name: 清晰系统蓝图
scope:
  - knowledge_explainer
  - technology_explainer
  - ai_and_programming
  - product_mechanism
  - business_and_science_education
canvas:
  width_px: 1080
  height_px: 1440
  fps: 30
  orientation: vertical
safe_area:
  structural: {left_px: 40, right_px: 40, top_px: 96, bottom_px: 60}
  main_content: {left_px: 88, right_px: 88, top_px: 120, bottom_px: 100}
  critical_text: {left_px: 88, right_px: 180, top_px: 120, bottom_px: 260}
  cover_title: {left_px: 88, right_px: 180, top_px: 260, bottom_px: 460}
  subtitles: {left_px: 88, right_px: 180, top_px: 990, bottom_px: 260}
colors:
  canvas: "#F5F7FB"
  ink: "#0E2340"
  engineering_blue: "#1857C4"
  capability_deck: "#12294A"
  blue_tint: "#EAF1FD"
  support_gray: "#4E6076"
  structure_line: "#D5DEEB"
  structure_line_strong: "#C3D2E6"
  white: "#FFFFFF"
  warning_red: "#A8412A"
  success_green: "#4FBF8B"
typography:
  primary_stack: '"Inter", "Hiragino Sans GB", "PingFang SC", sans-serif'
  mono_stack: '"JetBrains Mono", "Hiragino Sans GB", monospace'
  sizes_px:
    cover_min: 78
    cover_max: 112
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
    display_min: 0.98
    display_max: 1.08
    body: 1.35
    metadata: 1.2
spacing:
  content_left_px: 88
  content_width_px: 904
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
  sm_px: 8
  md_px: 12
  lg_px: 18
  xl_px: 22
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 命题型
    use_for: 封面、钩子、章节开场、强结论
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 对照型
    use_for: 原料与结果、旧方法与新方法、条件与结论
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 流程型
    use_for: 步骤、层级、因果、状态变化
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 能力板型
    use_for: 系统、能力、工程资产、验证结果
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.30, breathe: 0.40, resolve: 0.30}
  entrance_seconds: {min: 0.30, max: 0.60}
  exit_seconds: {min: 0.18, max: 0.45}
  transition_seconds: {min: 0.20, max: 0.40}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: [power2.out, power3.out, expo.out]
  exit_ease: [power2.in, power3.in]
  verbs: [DRAW, SLIDE, ASSEMBLE, FILL, LOCK_IN, STEP, VERIFY]
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
  max_width_px: 812
  max_lines: 2
  max_fullwidth_chars_per_line: 16
  line_height: {min: 1.30, max: 1.40}
  background: "rgba(18, 41, 74, 0.84)"
  text_color: "#F5F7FB"
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
  font_size_px: {min: 78, max: 112}
  line_height: {min: 0.98, max: 1.08}
  max_width_px: 812
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - centered_floating_web_card_stack
  - arbitrary_new_brand_colors
  - gradient_text
  - neon_purple_blue_gradient
  - excessive_glitch
  - elastic_or_bouncy_motion
  - all_elements_same_direction_same_speed
  - labels_or_icons_touching_edges
  - skeleton_or_list_without_bottom_padding
  - sfx_on_every_element
  - music_masking_voice
---

# 清晰系统蓝图

这是知识与科技类竖屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择命题型、对照型、流程型或能力板型镜头。
- 渲染工具负责镜头内部的图形隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、圆角、动作语法和声音目标。
- 每个镜头至少有两个视觉焦点，并包含背景、中景和前景三个层次。
- 先表达关系，再装饰画面。节点、连线、层级、状态和模块必须承载真实语义。

## 品牌人格

清晰、系统、工程化、克制、可信。画面像一张被逐步激活的系统蓝图：信息被发现、读取、组装、验证和锁定。

## 构图

- 外层结构角标可进入 `structural` 区；关键内容不得进入该区。
- 普通画面使用 `main_content` 区。
- 标题、数字、结论和核心术语使用 `critical_text` 区，主动避让抖音右侧互动栏和底部 UI。
- 深蓝能力板只用于系统、能力、工程资产和结果，不作为普通装饰色块。

## 动画

遵循 Build / Breathe / Resolve。入场以 DRAW、SLIDE、ASSEMBLE、FILL、LOCK IN、STEP、VERIFY 为主要动作词。入场使用 ease-out，退场使用 ease-in；按信息优先级而非 DOM 顺序编排。每镜头最多一种环境动作。

动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若平台会另加字幕，可以关闭；若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则。

第 0 帧必须已经是完整封面。标题稳定至少 18 帧后才允许转场，不得以黑帧、空白或淡入中间态开场。

## 声音

人声优先且保持原速。BGM 只建立克制的技术氛围；有人声时按 frontmatter 下压。SFX 只标记钩子、信息出现、确认、关键转场和结论，同时最多两个。

## 可变项

题材、图形隐喻、图标、节点数量、局部构图、内容素材、第三方 Logo 和产品截图可以变化。第三方品牌只能作为内容素材，不能改写本系列的核心 token。

## 验收

制作完成后检查：

1. 核心颜色、字体、圆角和间距是否来自 frontmatter。
2. 关键文字是否避让平台 UI。
3. 标签、状态和图标距边缘是否至少 22px。
4. 列表、骨架屏和进度条底部是否至少留 22px。
5. 第 0 帧是否完成、可读且稳定 18 帧。
6. 动画是否可寻址、确定性、无同质化入场。
7. 成片声音是否达到 loudness 和 true-peak 目标。
