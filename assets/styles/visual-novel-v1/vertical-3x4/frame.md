---
schema_version: 1
style_id: visual-novel-v1
style_name: 视觉小说对话
scope:
  - story_driven_explainer
  - dialogue_format_explainer
  - emotion_and_relationships
  - character_and_history_stories
  - reflective_personal_growth
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
  canvas: "#2A2238"
  dusk_violet: "#4A3B63"
  horizon_rose: "#E8A0B4"
  dialogue_box: "#1E1830"
  choice_panel: "#3A2E50"
  paper: "#F6F0FA"
  mist_lavender: "#B9A8D6"
  lantern_gold: "#F3C77A"
  ink: "#221A30"
  warning_coral: "#E88A8A"
  memory_mint: "#7FD1C0"
typography:
  primary_stack: '"LXGW WenKai", "Noto Sans SC", sans-serif'
  mono_stack: '"Noto Sans SC", sans-serif'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "LXGW WenKai", weight: 400, file: "assets/fonts/lxgw-wenkai-400.woff2"}
  sizes_px:
    cover_min: 80
    cover_max: 112
    title_min: 48
    title_max: 66
    body_min: 30
    body_max: 38
    label_min: 22
    label_max: 26
  weights:
    display: 400
    heading: 400
    body: 400
    body_emphasis: 400
    metadata: 700
  line_heights:
    display_min: 1.08
    display_max: 1.18
    body: 1.55
    metadata: 1.2
spacing:
  content_left_px: 88
  content_width_px: 904
  group_min_px: 14
  group_max_px: 22
  card_stack_min_px: 18
  card_main_padding_min_px: 32
  card_main_padding_max_px: 40
  card_secondary_padding_min_px: 22
  card_secondary_padding_max_px: 28
  section_min_px: 32
  section_max_px: 60
  element_edge_min_px: 22
  skeleton_bottom_min_px: 22
radius:
  sm_px: 8
  md_px: 14
  lg_px: 20
  xl_px: 28
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 章节标题卡
    use_for: 封面、钩子、章节开场、强结论
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 分支选择
    use_for: 两种做法、两种观点、选择与后果、旧路与新路
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 对话推进
    use_for: 一问一答、步骤讲解、因果推进、状态变化
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 存档卡册
    use_for: 要点回顾、多条经验、阶段成果、全片总结
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.30, breathe: 0.45, resolve: 0.25}
  entrance_seconds: {min: 0.35, max: 0.70}
  exit_seconds: {min: 0.25, max: 0.50}
  transition_seconds: {min: 0.30, max: 0.60}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: [power1.out, power2.out, sine.out]
  exit_ease: [power1.in, power2.in, sine.in]
  verbs: [FADE_IN, TYPE, SPEAK, CHOOSE, ADVANCE, TURN_PAGE, SAVE]
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
  background: "rgba(30, 24, 48, 0.88)"
  text_color: "#F6F0FA"
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
  max_fullwidth_chars_per_line: 10
  font_size_px: {min: 80, max: 112}
  line_height: {min: 1.08, max: 1.18}
  max_width_px: 812
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - copying_commercial_game_ip_characters_or_logos
  - copying_specific_commercial_game_ui
  - character_sprites_or_faces_of_real_or_ip_characters
  - dialogue_box_overlapping_subtitle_band
  - dialogue_text_outside_critical_text_area
  - typewriter_slower_than_narration
  - per_character_sound_blips_over_voice
  - fully_transparent_dialogue_box
  - photographic_or_ai_generated_backgrounds
  - arbitrary_new_brand_colors
  - gradient_text
  - elastic_or_bouncy_motion
  - screen_shake_or_flash_cuts
  - continuous_particle_storms
  - labels_or_icons_touching_edges
  - music_masking_voice
---# 视觉小说对话

这是故事化、对话体竖屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择章节标题卡、分支选择、对话推进或存档卡册镜头。
- 渲染工具负责镜头内部的场景道具、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、圆角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景（氛围场景）、中景（道具与剪影）和前景（对话框与界面）三个层次。
- 把讲解写成对话：一个发问的人、一个回答的人，或一个旁白。对话框承载口播里最关键的那一句，不承载整段稿子。

## 品牌人格

温柔、安静、有代入感、节奏从容。画面像一部视觉小说正在播放：夜色或黄昏的场景先铺好，名牌亮起，一句台词被打出来，然后停下来等观众读完。

## 场景层

- 背景只用矢量渐变与简单几何：天空分层、远山、窗框、路灯、书桌、月亮、云带。不画角色立绘和人脸；需要人物时只用无五官的抽象剪影。
- 不使用照片、AI 生成图或任何商业游戏的角色、Logo、特定界面截图作为品牌层。
- 背景亮度要压低：主文字永远落在对话框或实色面板上，不直接压在渐变天空上。

## 对话框

- 对话框底色 `dialogue_box`，以约 0.86 不透明度叠在场景上；对比度一律按它叠在最亮背景上的等效实色计算，正文 `paper`、辅助 `mist_lavender`、强调 `lantern_gold` 都须 ≥ 4.5:1。
- 对话框必须整体位于 `safe_area.subtitles.top_px` 之上，且与字幕带至少相距 24px；对话框内文字只在 `critical_text` 区内换行。对话框不得侵入字幕带，字幕也不得盖在对话框上。
- 名牌是 `horizon_rose` 胶囊，压在对话框左上边缘，文字用 `ink`。旁白没有名牌，只用 `mist_lavender` 小标签「旁白」。
- 对话框高度至少 230px，最多三行正文；右下角放推进指示（小三角），只在整句打完后出现。

## 字体

- 对话、标题、章节名：`LXGW WenKai` 400。它只有一个字重，不要用 `font-weight: 700` 伪加粗；强调靠 `lantern_gold` 颜色和字号。
- 界面标签、按钮编号、存档时间、页码：`Noto Sans SC` 600/700。
- 中文正文不小于 30px，标签不小于 22px。

## 动画

遵循 Build / Breathe / Resolve。主要动作词为 FADE IN、TYPE、SPEAK、CHOOSE、ADVANCE、TURN PAGE、SAVE。入场使用 ease-out，退场使用 ease-in，全部柔和；不弹跳、不震屏、不闪白。

- 打字机速度 8–14 字/秒，并且必须领先于口播：台词在配音说完这一句之前已完整显示。标点处可停 0.12–0.25s。
- 选择按钮的高亮只做一次：移入、描边变金、停住。
- 每镜头最多一种环境动作（云带缓移、灯光呼吸或推进三角轻闪，三选一）；读完后允许画面完全静止地停留。

动画必须确定性、可寻址，并可在任意帧重建。打字机用按时间计算的字符数实现，不得使用运行时随机数、墙钟时间或逐帧累加状态。

## 字幕与封面

字幕是可选层。若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则；字幕与对话框内容不要逐字重复同一句。

第 0 帧必须已经是完整封面（章节标题卡的完成态），不得以黑帧、空白、打字机未打完或淡入中间态开场。标题稳定至少 18 帧后才允许转场。

## 声音

人声优先且保持原速。BGM 用安静的钢琴、弦乐或环境垫底；有人声时按 frontmatter 下压。不要给打字机配逐字音效；SFX 只用于翻页、选择确认、存档和章节切换，同时最多两个。

## 验收

1. 核心颜色、字体、圆角和间距是否来自 frontmatter。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 对话框是否在字幕带之上并留出 24px，文字是否在 `critical_text` 区内。
4. 对话框上的文字对比度是否按等效实色达到 4.5:1。
5. 打字机是否领先口播、打完后是否留出阅读停顿。
6. 第 0 帧是否完成、可读且稳定 18 帧。
7. 画面里有没有任何商业作品的角色、Logo 或特定界面。
8. 成片声音是否达到 loudness 和 true-peak 目标。
