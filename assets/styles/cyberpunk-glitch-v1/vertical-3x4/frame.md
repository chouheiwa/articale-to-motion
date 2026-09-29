---
schema_version: 1
style_id: cyberpunk-glitch-v1
style_name: 赛博故障
scope:
  - security_and_hacking
  - contrarian_opinion
  - technology_explainer
  - internet_culture
  - risk_and_failure_analysis
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
  canvas: "#120B1E"
  panel: "#1E1433"
  panel_raised: "#2A1C47"
  structure_line: "#4A3772"
  ink: "#F4EEFF"
  muted_ink: "#B9ABDB"
  hot_magenta: "#FF2E88"
  signal_cyan: "#29E6F0"
  acid_yellow: "#D4FF3A"
  alert_red: "#FF5A4E"
  ink_on_accent: "#120B1E"
typography:
  primary_stack: '"Noto Sans SC", sans-serif'
  mono_stack: '"Chakra Petch", "Noto Sans SC", monospace'
  display_stack: '"Smiley Sans", "Noto Sans SC", sans-serif'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "Smiley Sans", weight: 400, file: "assets/fonts/smiley-sans-oblique.woff2"}
    - {family: "Chakra Petch", weight: 700, file: "assets/fonts/chakra-petch-700.woff2"}
  sizes_px:
    cover_min: 84
    cover_max: 124
    title_min: 50
    title_max: 72
    body_min: 24
    body_max: 34
    label_min: 18
    label_max: 24
  weights:
    display: 400
    heading: 400
    body: 400
    body_emphasis: 700
    metadata: 700
  line_heights:
    display_min: 1.00
    display_max: 1.12
    body: 1.40
    metadata: 1.15
spacing:
  content_left_px: 88
  content_width_px: 904
  group_min_px: 12
  group_max_px: 20
  card_stack_min_px: 18
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
  md_px: 2
  lg_px: 4
  xl_px: 6
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 入侵警报
    use_for: 封面、钩子、反常识判断、章节开场、强结论
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 信号对撞
    use_for: 表象与真相、旧认知与新认知、攻击方与防守方
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 破解链路
    use_for: 攻击链、排查步骤、因果链、状态逐级突破
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 义体配置
    use_for: 能力组合、工具清单、防护模块、最终方案
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.30, breathe: 0.45, resolve: 0.25}
  entrance_seconds: {min: 0.20, max: 0.50}
  exit_seconds: {min: 0.12, max: 0.30}
  transition_seconds: {min: 0.10, max: 0.30}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: [power3.out, expo.out, "steps(4)"]
  exit_ease: [power2.in, power3.in, "steps(3)"]
  verbs: [CUT_IN, SLICE, SPLIT_SETTLE, DECRYPT, SCAN, BREACH, LOCK_ON]
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
  background: "rgba(18, 11, 30, 0.88)"
  text_color: "#F4EEFF"
  padding_px: {vertical: 14, horizontal: 22}
  radius_px: 2
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
  font_size_px: {min: 84, max: 124}
  line_height: {min: 1.00, max: 1.12}
  max_width_px: 812
  contrast_ratio_min: 7.0
  stable_frames: 24
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - glitch_on_frame_zero_title
  - glitch_during_text_read_window
  - random_or_unseeded_glitch
  - flashes_over_three_per_second
  - full_frame_strobe_or_invert
  - rgb_split_on_body_text
  - body_text_in_smiley_sans
  - magenta_text_on_panel_raised
  - ink_text_on_bright_accent_fill
  - neon_colors_outside_tokens
  - acid_yellow_as_decoration
  - neon_glow_blur_on_small_text
  - rounded_web_cards
  - gradient_text
  - elastic_or_bouncy_motion
  - continuous_screen_shake
  - labels_or_icons_touching_edges
  - skeleton_or_list_without_bottom_padding
  - copying_commercial_game_or_film_logos_characters_or_ui
  - imitating_cyberpunk_2077_ui_or_rajdhani_font
  - real_exploit_code_or_real_credentials_on_screen
  - sfx_on_every_element
  - music_masking_voice
---

# 赛博故障

这是安全攻防、黑客叙事与反差观点类竖屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择入侵警报、信号对撞、破解链路或义体配置镜头。
- 渲染工具负责镜头内部的图形隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、切角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景、中景和前景三个层次。
- 故障是标点，不是底色。画面的大部分时间必须是干净、静止、可读的；故障只出现在切入、转折和锁定的瞬间。

## 品牌人格

锐利、紧张、反叛、清醒。画面像一台被接管的终端：信号被截获、切片、错位，再被重新对齐成一条清晰结论。

## 构图

- 背景层：近黑紫画布 `canvas`，可叠极弱扫描线或透视网格；中景层：`panel` 切角面板、终端窗、警示条；前景层：标题、数字、结论与锁定框。
- 外层角标、坐标、警示斜条可进入 `structural` 区；关键内容不得进入该区。
- 普通画面使用 `main_content` 区。标题、数字、结论和核心术语使用 `critical_text` 区，主动避让抖音右侧互动栏和底部 UI。
- 面板用 45° 切角（切角边长 16–32px）和 0–6px 圆角，不做圆润网页卡片。
- 霓虹强调色按语义分工：品红 `hot_magenta` 负责冲突与警报，青 `signal_cyan` 负责信号与数据，二者是本风格的主对子；酸黄 `acid_yellow` 只留给最终结论或锁定状态，整镜至多一处；`alert_red` 只表示真实风险或失败。不另加其他霓虹色。

## 字体

- `Smiley Sans`（得意黑斜体，仅 400）只用于封面、镜头标题、短口号等 ≥50px 的展示字。
- `Chakra Petch` 700 只用于英文标签、编号、代码、坐标和数字；它不含中文，中文自动回退到 `Noto Sans SC`。
- 正文、说明与字幕一律显式使用 `Noto Sans SC`（400/700），字号不小于 24px。

## 动画

遵循 Build / Breathe / Resolve。入场以 CUT IN、SLICE、SPLIT SETTLE、DECRYPT、SCAN、BREACH、LOCK ON 为主要动作词；允许 `steps()` 阶跃缓动表达数字信号，退场使用 ease-in 或硬切。

故障可读性边界：

- 一次故障爆发（RGB 分离、切片位移、扫描撕裂）不超过 0.25s，且在口播念到该文字之前已完全归位；文字归位后至少静止 18 帧才允许下一次扰动。
- 每秒闪烁或反相不超过 3 次，不做全屏频闪、全屏反色和持续抖屏。
- 正文不做 RGB 分离；RGB 分离只作用于展示字和图形，偏移不超过 8px。
- 每镜头最多一种环境动作（例如扫描线缓慢下移或光标闪烁），稳定可读的静止保持完全合法。

动画必须确定性、可寻址，并可在任意帧重建。故障的切片位置、偏移量和时刻由固定种子或手写表生成，不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则；字幕不做任何故障效果。

第 0 帧必须已经是完整、干净、无故障的封面。标题稳定至少 24 帧后才允许第一次故障扰动或转场，不得以黑帧、空白、噪点屏或解码中间态开场。

## 声音

人声优先且保持原速。BGM 可用低频合成器与轻颗粒噪声建立紧张感；有人声时按 frontmatter 下压。故障 SFX 只标记钩子、切入、锁定和关键转场，同时最多两个，不给每个元素配音效。

## 可变项

题材、图形隐喻、终端内容、节点数量、局部构图、内容素材和第三方 Logo 可以变化。不得复制任何商业游戏、电影或动漫的 Logo、角色、界面和标志性配色，不模仿《赛博朋克 2077》的界面与 Rajdhani 字体；终端里不出现可执行的真实攻击代码或真实凭据。

## 验收

制作完成后检查：

1. 核心颜色、字体、切角和间距是否来自 frontmatter。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 关键文字是否避让平台 UI。
4. 标签、状态和图标距边缘是否至少 22px。
5. 每次故障是否在口播到达前归位，归位后是否静止至少 18 帧，每秒闪烁是否不超过 3 次。
6. 第 0 帧是否完成、干净、可读且稳定 24 帧。
7. 动画是否可寻址、确定性、无同质化入场。
8. 成片声音是否达到 loudness 和 true-peak 目标。
