---
schema_version: 1
style_id: rebel-graphic-v1
style_name: 二次元锐利拼贴
scope:
  - strong_opinion
  - ranking_and_tier_list
  - character_style_explainer
  - game_and_anime_culture
  - contrarian_takes
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
  canvas: "#111111"
  plate_charcoal: "#2A2A2A"
  paper: "#F4F1EA"
  white: "#FFFFFF"
  strike_yellow: "#FFE14A"
  slash_red: "#D8152F"
  deep_red: "#9E0F22"
  halftone_gray: "#B9B4AA"
  muted_ink: "#6E6A63"
  ink: "#111111"
typography:
  primary_stack: '"Noto Sans SC", sans-serif'
  mono_stack: '"Bungee", "Noto Sans SC", sans-serif'
  display_stack: '"ZCOOL QingKe HuangYou", "Noto Sans SC", sans-serif'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "ZCOOL QingKe HuangYou", weight: 400, file: "assets/fonts/zcool-qingke-huangyou-400.woff2"}
    - {family: "Bungee", weight: 400, file: "assets/fonts/bungee-400.woff2"}
  sizes_px:
    cover_min: 88
    cover_max: 128
    title_min: 52
    title_max: 76
    body_min: 26
    body_max: 36
    label_min: 22
    label_max: 28
  weights:
    display: 400
    heading: 400
    body: 400
    body_emphasis: 700
    metadata: 400
  line_heights:
    display_min: 0.96
    display_max: 1.06
    body: 1.38
    metadata: 1.1
spacing:
  content_left_px: 120
  content_width_px: 1680
  group_min_px: 12
  group_max_px: 20
  card_stack_min_px: 14
  card_main_padding_min_px: 28
  card_main_padding_max_px: 40
  card_secondary_padding_min_px: 22
  card_secondary_padding_max_px: 28
  section_min_px: 32
  section_max_px: 60
  element_edge_min_px: 22
  skeleton_bottom_min_px: 22
radius:
  sm_px: 0
  md_px: 0
  lg_px: 0
  xl_px: 0
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 冲击标题
    use_for: 封面、钩子、章节开场、一句话强观点
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 评级对决
    use_for: 两方对比、优劣评级、旧做法与新做法、误区与真相
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 菜单选项
    use_for: 步骤、选择分支、排行清单、按顺序解锁的要点
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 属性雷达
    use_for: 多维能力、综合评分、角色化的系统画像、最终结果
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.25, breathe: 0.55, resolve: 0.20}
  entrance_seconds: {min: 0.16, max: 0.40}
  exit_seconds: {min: 0.12, max: 0.28}
  transition_seconds: {min: 0.12, max: 0.30}
  first_action_delay_seconds: {min: 0.10, max: 0.25}
  entrance_ease: [expo.out, power4.out, back.out(1.6)]
  exit_ease: [power3.in, expo.in]
  verbs: [SLASH, SLAM, CUT_IN, STAMP, SNAP, IMPACT_SHAKE, RANK_UP]
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
  background: "rgba(17, 17, 17, 0.90)"
  text_color: "#F4F1EA"
  padding_px: {vertical: 14, horizontal: 22}
  radius_px: 0
  break_rules:
    - keep_proper_nouns_intact
    - keep_english_phrases_intact
    - keep_numbers_and_units_intact
    - preserve_real_speech_gaps
cover:
  frame_zero_complete: true
  default_lines: 2
  max_lines: 3
  max_fullwidth_chars_per_line: 8
  font_size_px: {min: 88, max: 128}
  line_height: {min: 0.96, max: 1.06}
  max_width_px: 1600
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - copying_commercial_game_ui_logos_or_characters
  - persona_style_red_black_ransom_menu_replica
  - small_dark_text_on_slash_red
  - body_text_in_display_font
  - text_moving_while_being_read
  - continuous_or_long_screen_shake
  - elastic_or_rubber_band_motion
  - rounded_web_cards
  - gradient_text
  - more_than_three_hues_per_scene
  - halftone_behind_body_text
  - full_screen_flash_strobe
  - arbitrary_new_brand_colors
  - labels_or_icons_touching_edges
  - skeleton_or_list_without_bottom_padding
  - sfx_on_every_element
  - music_masking_voice
---

# 二次元锐利拼贴

这是强观点、排行与角色化讲解类横屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择冲击标题、评级对决、菜单选项或属性雷达镜头。
- 渲染工具负责镜头内部的图形隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、直角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景、中景和前景三个层次。
- 拼贴是承载信息的版式，不是噪声。每一块斜切色板、评级字母和雷达轴都必须对应文案里的真实概念。

## 品牌人格

锐利、自信、有态度、节奏干脆。画面像一套被逐项点亮的动画风游戏菜单：观点被斩出、被盖章、被评级、被锁定。只借用这一类游戏界面的共同语汇（斜切、网点、星芒、评级字母、雷达图），**不得复刻任何商业游戏的具体界面、Logo、角色或字形**。

## 色彩与对比

- 三色主轴：墨黑 `ink`/`canvas`、纸白 `paper`、打击黄 `strike_yellow`；斩击红 `slash_red` 只做大面积斜切色块和 46px 以上的白字标题底。
- 同一镜头最多三种色相（黑白不计），不临时引入新色。
- **不在 `slash_red` 上放小号黑字。** 正文与标签一律放在纸白板（配墨黑字）、墨黑板（配纸白字）或 `deep_red` 板（配纸白字）上。
- `strike_yellow` 上只放墨黑字；`halftone_gray` 只做墨黑底上的次级文字或网点。

## 字体角色

- 展示字（封面、镜头标题、评级词）：`ZCOOL QingKe HuangYou`，即 `typography.display_stack`；正文与说明一律用 `typography.primary_stack`（Noto Sans SC）。
- 英文、数字、评级字母、编号：`Bungee`，即 `typography.mono_stack`。
- 正文、说明、字幕（36px 以下的所有中文句子）：只用 `"Noto Sans SC", sans-serif`，400 / 700。展示字不得用于正文。
- 拼贴字块可以把 `ZCOOL QingKe HuangYou`、`Noto Sans SC` 900 与 `Bungee` 混排，但一个词组最多三块，且每块底板都满足对比规则。中文正文不小于 24px。

## 构图

- 外层斜切色带、网点、星芒可进入 `structural` 区；关键内容不得进入该区。
- 普通画面使用 `main_content` 区；标题、数字、评级和结论使用 `critical_text` 区，主动避让播放器底部控制栏与进度条。
- 主斜线角度统一在 -12° 到 -18° 之间（左低右高），同一镜头不混用多个方向的斜切。
- 所有板块都是直角或斜切角，不使用圆角卡片；`pill_px` 只用于圆形评级徽章。

## 动画

遵循 Build / Breathe / Resolve。入场以 SLASH、SLAM、CUT IN、STAMP、SNAP、IMPACT SHAKE、RANK UP 为主要动作词：快速进场、轻微过冲、硬停。退场使用 ease-in 快速切走。

- 文字落定后保持静止直到读完；不做持续漂浮、呼吸缩放或跑马灯。
- IMPACT SHAKE 只用于关键落点，单次不超过 0.20s、位移不超过 12px；不做持续晃动。
- 每镜头最多一种环境动作（例如网点缓慢平移或星芒微转），允许整段稳定停留。
- 禁止全屏闪白、频闪和弹簧/橡皮筋效果。

动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限、墨黑直角底和语义断行规则。

第 0 帧必须已经是完整封面。标题稳定至少 18 帧后才允许转场，不得以黑帧、空白或斩击进行中的中间态开场。

## 声音

人声优先且保持原速。BGM 可以更有律动，但有人声时按 frontmatter 下压。SFX 只标记斩击、盖章、评级揭晓和结论，同时最多两个。

## 验收

1. 核心颜色、字体、直角和间距是否来自 frontmatter。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 有无小号黑字落在 `slash_red` 上；正文是否只用 Noto Sans SC。
4. 关键文字是否避让平台 UI；标签、评级徽章距边缘是否至少 22px。
5. 第 0 帧是否完成、可读且稳定 18 帧。
6. 画面里是否出现任何商业游戏的 Logo、角色、专有界面或可辨认的仿制字形。


## 横屏构图要点

本项目是 16:9 横屏（1920×1080），画面的主方向是左右而不是上下。风格说明书里按竖屏描述的版式骨架，落到横屏时按下面的原则改排：

- 左右分栏优先于上下堆叠：命题型用「左文右图」或「左大字、右主视觉」；对照型直接左右对开；流程型沿水平方向展开，最多 5 步，超过就分两行；系统总览用横向网格。
- 不要把竖屏版式原样居中：中间一条窄栏、两侧大面积空白，是横屏最常见的「没排完」。
- 一行文字不超过 22 个汉字：横屏宽度大，单行过长会让视线来回扫，宁可断行。
- 标题字号以画面高度 1080px 为基准，与竖屏的同级字号保持一致，不因为画面变宽而放大。
- 底部控制栏与进度条是关键避让区：标题、数字、结论和字幕都不得进入 critical_text 下沿以下的区域；字幕带贴在进度条上方。
