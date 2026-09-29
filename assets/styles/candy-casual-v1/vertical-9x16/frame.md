---
schema_version: 1
style_id: candy-casual-v1
style_name: 糖果休闲
scope:
  - light_science_explainer
  - life_tips_and_how_to
  - habit_and_growth_motivation
  - beginner_friendly_product_intro
  - family_and_youth_education
canvas:
  width_px: 1080
  height_px: 1920
  fps: 30
  orientation: vertical
safe_area:
  structural: {left_px: 40, right_px: 40, top_px: 96, bottom_px: 60}
  main_content: {left_px: 88, right_px: 88, top_px: 120, bottom_px: 100}
  critical_text: {left_px: 88, right_px: 180, top_px: 120, bottom_px: 260}
  cover_title: {left_px: 88, right_px: 180, top_px: 260, bottom_px: 940}
  subtitles: {left_px: 88, right_px: 180, top_px: 1470, bottom_px: 260}
colors:
  canvas: "#FFF4E8"
  ink: "#3B1E54"
  white: "#FFFFFF"
  candy_pink: "#FF5C8A"
  berry: "#B8285A"
  grape: "#7A4BD1"
  lemon: "#FFC83D"
  mint: "#25C08A"
  sky: "#45B8F5"
  bubble_tint: "#FFE3EE"
  support_text: "#6E4E86"
typography:
  primary_stack: '"Noto Sans SC", sans-serif'
  mono_stack: '"Fredoka", "Noto Sans SC", sans-serif'
  display_stack: '"ZCOOL KuaiLe", "Noto Sans SC", sans-serif'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "ZCOOL KuaiLe", weight: 400, file: "assets/fonts/zcool-kuaile-400.woff2"}
    - {family: "Fredoka", weight: 600, file: "assets/fonts/fredoka-600.woff2"}
    - {family: "Fredoka", weight: 700, file: "assets/fonts/fredoka-700.woff2"}
  sizes_px:
    cover_min: 84
    cover_max: 120
    title_min: 50
    title_max: 68
    body_min: 26
    body_max: 36
    label_min: 24
    label_max: 30
  weights:
    display: 400
    heading: 400
    body: 600
    body_emphasis: 700
    metadata: 700
  line_heights:
    display_min: 1.02
    display_max: 1.12
    body: 1.38
    metadata: 1.15
spacing:
  content_left_px: 88
  content_width_px: 904
  group_min_px: 14
  group_max_px: 24
  card_stack_min_px: 20
  card_main_padding_min_px: 32
  card_main_padding_max_px: 44
  card_secondary_padding_min_px: 22
  card_secondary_padding_max_px: 30
  section_min_px: 36
  section_max_px: 60
  element_edge_min_px: 24
  skeleton_bottom_min_px: 24
radius:
  sm_px: 16
  md_px: 24
  lg_px: 36
  xl_px: 48
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 奖励弹窗
    use_for: 封面、钩子、章节开场、强结论
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 关卡评星
    use_for: 做法对比、前后对照、一星与三星的差距
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 签到日历
    use_for: 步骤、每日习惯、进度累积、阶段解锁
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 商店货架
    use_for: 清单、工具箱、选项总览、收获盘点
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.30, breathe: 0.45, resolve: 0.25}
  entrance_seconds: {min: 0.28, max: 0.55}
  exit_seconds: {min: 0.16, max: 0.35}
  transition_seconds: {min: 0.20, max: 0.40}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: [back.out(1.6), back.out(2.2), power3.out, elastic.out(1, 0.6)]
  exit_ease: [back.in(1.4), power2.in]
  verbs: [POP, DROP_IN, STAR_BURST, COUNT_UP, SQUASH, UNLOCK, STAMP]
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
  font_size_px: {min: 34, max: 40}
  max_width_px: 812
  max_lines: 2
  max_fullwidth_chars_per_line: 15
  line_height: {min: 1.30, max: 1.40}
  background: "rgba(59, 30, 84, 0.88)"
  text_color: "#FFFFFF"
  padding_px: {vertical: 14, horizontal: 24}
  radius_px: 24
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
  font_size_px: {min: 84, max: 120}
  line_height: {min: 1.02, max: 1.12}
  max_width_px: 812
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - copying_commercial_game_logos_characters_or_ui
  - named_real_candy_or_mascot_characters
  - arbitrary_new_brand_colors
  - thin_hairline_outlines
  - white_text_on_pink_mint_sky_or_lemon_without_ink_plate
  - gradient_text
  - glassmorphism_or_neon_glow
  - text_still_bouncing_after_settle
  - continuous_wobble_or_idle_bounce_on_text
  - confetti_or_particle_rain_covering_text
  - loot_box_gambling_or_real_money_prices
  - more_than_one_popup_panel_at_once
  - labels_or_icons_touching_edges
  - skeleton_or_list_without_bottom_padding
  - sfx_on_every_element
  - music_masking_voice
---

# 糖果休闲

这是轻松科普、生活技巧与成长激励类竖屏 MG 视频的品牌层规范。画面借用休闲手游的界面语言：糖果色、圆胖形状、粗深色描边、底部“3D 唇边”阴影、奖励弹窗与星星金币。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择奖励弹窗、关卡评星、签到日历或商店货架镜头。
- 渲染工具负责镜头内部的图形隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、圆角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景、中景和前景三个层次。
- 游戏界面只是隐喻。星星、金币、关卡、签到必须对应文案里的真实含义（进度、评价、收益、步骤），不得为了热闹堆奖励图标。

## 品牌人格

轻松、友好、有成就感、明快、不幼稚。画面像一局刚刚通关的小游戏：任务被领取、进度被点亮、星星被点亮、奖励被收下。

## 形状语言

- 所有主体都是圆胖的圆角形状，使用 `ink` 粗描边：主面板与按钮 6px，次级元素 4px，最细不低于 3px。
- 按钮、主面板和格子带“3D 唇边”：同形状向下偏移 8–12px，填充 `ink`，放在主体之下。唇边只向下，不向其他方向。
- 光泽高光只用 `white`（不透明度 0.35–0.55）的圆角条，放在形状上沿内侧，不做渐变文字。
- 圆角只用 frontmatter 的 16 / 24 / 36 / 48 / 999 与 50%。

## 色彩与文字对比

- `ink` 文字可放在 `canvas`、`white`、`bubble_tint`、`lemon`、`mint`、`sky`、`candy_pink` 上（均 ≥ 4.5:1）。
- `white` 文字只放在 `grape`、`berry`、`ink` 底上。粉、薄荷、天蓝、柠檬底上不得直接放白字；必须放白字时，文字加 ≥ 6px 的 `ink` 描边（`paint-order: stroke`），且字号 ≥ 46px。
- `support_text` 只用于 `canvas` / `white` / `bubble_tint` 底上的辅助说明。
- `berry` 表示失败、风险、扣分；`mint` 表示完成、通过、领取。二者是语义色，不作装饰轮换。

## 字体角色

- `ZCOOL KuaiLe`（仅 400）：封面、镜头标题、按钮字、面板标题。不得设置 `font-weight: 700`，否则会合成伪粗体。
- `Noto Sans SC`：正文、说明、超过一行的中文段落，字重 600 为主。中文正文不小于 24px。
- `Fredoka` 600/700：数字、倍率、分数、金币数、`LV`/`x3` 这类拉丁短标签。

## 构图

- 外层装饰（糖果条纹、圆点、散落星星）可进入 `structural` 区；关键内容不得进入该区。
- 普通画面使用 `main_content` 区。
- 标题、数字、结论和按钮文字使用 `critical_text` 区，主动避让抖音右侧互动栏和底部 UI。
- 一镜只允许一个弹窗式主面板；面板外至多一枚浮动奖励图形作为第二焦点。

## 动画

遵循 Build / Breathe / Resolve。动作词为 POP、DROP_IN、STAR_BURST、COUNT_UP、SQUASH、UNLOCK、STAMP。允许 `back.out` 与轻度 `elastic.out`，但只用于图形和面板；文字在 0.35s 内落定，落定后不再弹动。退场使用 ease-in，快速干净。每镜头最多一种环境动作（例如背景星星的缓慢闪烁）；信息稳定后允许完全静止的停留。

动画必须确定性、可寻址，并可在任意帧重建。星爆、撒币等粒子必须用固定种子或手写坐标，不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若平台会另加字幕，可以关闭；若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则，葡萄紫深底、白字、24px 圆角。

第 0 帧必须已经是完整封面：面板、标题、星星全部就位。标题稳定至少 18 帧后才允许转场，不得以黑帧、空白、缩放到 0 的中间态开场。

## 声音

人声优先且保持原速。BGM 可以轻快，但有人声时按 frontmatter 下压。SFX 只标记钩子、星星点亮、奖励领取、解锁和结论，同时最多两个；不给每颗星、每枚金币都配音。

## 可变项

题材、图形隐喻、图标、格子数量、局部构图和内容素材可以变化。不得复制任何商业游戏的 Logo、角色、糖果造型、吉祥物或界面布局；第三方品牌只能作为内容素材出现。

## 验收

制作完成后检查：

1. 核心颜色、字体、圆角和间距是否来自 frontmatter。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 亮色底上的文字是否使用 `ink`，或有 `ink` 实底/粗描边保证对比度。
4. 关键文字是否避让平台 UI；标签、星星、金币距边缘是否至少 24px。
5. 列表、日历格和进度条底部是否至少留 24px。
6. 第 0 帧是否完成、可读且稳定 18 帧。
7. 文字是否在落定后保持静止；弹性动作是否只作用于图形。
8. 成片声音是否达到 loudness 和 true-peak 目标。
