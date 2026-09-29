---
schema_version: 1
style_id: fantasy-quest-v1
style_name: 奇幻羊皮卷
scope:
  - history_and_story
  - learning_roadmap
  - worldbuilding_explainer
  - growth_and_skill_path
  - culture_and_humanities_education
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
  canvas: "#F1E4C6"
  parchment_deep: "#E4D0A4"
  ink: "#2E1F12"
  ink_soft: "#5C4330"
  bronze: "#7A4E17"
  gold: "#B8892E"
  gold_light: "#D9B45A"
  leather: "#3F2B1A"
  rule_line: "#C9AE7C"
  wax_red: "#8E2A22"
  quest_green: "#355A2F"
typography:
  primary_stack: '"Noto Serif SC", serif'
  mono_stack: '"Cinzel", "Noto Serif SC", serif'
  font_files:
    - {family: "Noto Serif SC", weight: 400, file: "assets/fonts/noto-serif-sc-400.woff2"}
    - {family: "Noto Serif SC", weight: 700, file: "assets/fonts/noto-serif-sc-700.woff2"}
    - {family: "Cinzel", weight: 700, file: "assets/fonts/cinzel-700.woff2"}
  sizes_px:
    cover_min: 78
    cover_max: 108
    title_min: 46
    title_max: 62
    body_min: 26
    body_max: 34
    label_min: 22
    label_max: 26
  weights:
    display: 700
    heading: 700
    body: 400
    body_emphasis: 700
    metadata: 700
  line_heights:
    display_min: 1.08
    display_max: 1.18
    body: 1.45
    metadata: 1.2
spacing:
  content_left_px: 96
  content_width_px: 888
  group_min_px: 14
  group_max_px: 22
  card_stack_min_px: 18
  card_main_padding_min_px: 32
  card_main_padding_max_px: 40
  card_secondary_padding_min_px: 24
  card_secondary_padding_max_px: 30
  section_min_px: 36
  section_max_px: 60
  element_edge_min_px: 24
  skeleton_bottom_min_px: 24
radius:
  sm_px: 4
  md_px: 8
  lg_px: 12
  xl_px: 16
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 任务卷轴
    use_for: 封面、钩子、章节开场、核心命题与最终结论
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 装备对比卡
    use_for: 两种方案、两个时代、两条路线的属性对照与取舍
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 旅程地图
    use_for: 步骤、学习路线、历史进程、因果链与阶段变化
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 技能树
    use_for: 能力体系、知识结构、前置依赖与解锁关系
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.30, breathe: 0.45, resolve: 0.25}
  entrance_seconds: {min: 0.45, max: 0.90}
  exit_seconds: {min: 0.25, max: 0.50}
  transition_seconds: {min: 0.30, max: 0.60}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: [power2.out, sine.out, power1.out]
  exit_ease: [power2.in, sine.in]
  verbs: [UNROLL, UNFOLD, INK_DRAW, TRACE_PATH, ILLUMINATE, STAMP_SEAL, FADE_IN]
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
  line_height: {min: 1.35, max: 1.45}
  background: "rgba(46, 31, 18, 0.86)"
  text_color: "#F1E4C6"
  padding_px: {vertical: 14, horizontal: 24}
  radius_px: 8
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
  font_size_px: {min: 78, max: 108}
  line_height: {min: 1.08, max: 1.18}
  max_width_px: 812
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - scroll_still_rolled_at_frame_zero
  - copying_commercial_game_logos_characters_or_ui
  - raster_parchment_or_stock_texture_images
  - unseeded_or_time_varying_noise
  - animated_turbulence_seed_or_frequency
  - gold_text_on_parchment_below_title_size
  - arbitrary_new_brand_colors
  - gradient_text
  - neon_glow_or_saturated_magic_particles
  - elastic_or_bouncy_motion
  - camera_shake_on_seal_stamp
  - burning_or_tearing_paper_transitions
  - blackletter_or_system_fonts
  - ornament_frame_around_every_element
  - labels_or_icons_touching_edges
  - skeleton_or_list_without_bottom_padding
  - sfx_on_every_element
  - music_masking_voice
---

# 奇幻羊皮卷

这是历史、故事、学习路线与世界观类竖屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择任务卷轴、装备对比卡、旅程地图或技能树镜头。
- 渲染工具负责镜头内部的图形隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、安全区、间距、圆角、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景、中景和前景三个层次。
- 先讲清关系，再加装饰。卷轴、卡牌、地图路径和技能节点必须承载真实语义，不做纯氛围摆件。

## 品牌人格

沉静、古典、有叙事感、可信。画面像一卷被缓缓展开的冒险手札：任务被写下，路线被描出，能力被点亮，结论被火漆封存。它借用奇幻 RPG 的语汇，但本质仍是讲解，不是游戏界面复刻。

## 材质

- 羊皮纸只用矢量渐变与 `feTurbulence` 生成，`seed` 固定写死，`baseFrequency` 与 `seed` 不随时间变化；不得使用位图纸纹或图库素材。
- 纸面纹理透明度保持在 0.04–0.10，只作背景层，不得降低文字对比度。
- 金色 `gold` 只用于边框、角花、符文分隔线和图标描边；正文与小字不得用金色写在羊皮纸上。
- 深色皮革 `leather` 是最高权重面板，只承载结论、能力核心或最终奖励。

## 构图

- 外层角花、边框和地图装饰可进入 `structural` 区；关键内容不得进入该区。
- 普通画面使用 `main_content` 区。
- 标题、数字、结论和核心术语使用 `critical_text` 区，主动避让抖音右侧互动栏和底部 UI。
- 装饰框只包住一个主容器；不要给每个元素都套华丽边框。

## 字体

- 中文标题与正文使用 `Noto Serif SC`：标题 700，正文 400，强调 700。
- 拉丁铭文、章节编号、罗马数字与英文小标题使用 `Cinzel` 700，大写并加 0.08–0.16em 字距。
- 中文正文不小于 24px；拉丁铭文不小于 22px。

## 动画

遵循 Build / Breathe / Resolve。入场以 UNROLL、UNFOLD、INK_DRAW、TRACE_PATH、ILLUMINATE、STAMP_SEAL、FADE_IN 为主要动作词。入场使用 ease-out，退场使用 ease-in；动作平缓，不弹跳、不回弹、不震屏。按信息优先级而非 DOM 顺序编排。每镜头最多一种环境动作（如烛光亮度的缓慢呼吸），关键信息登场后允许完全静止地停留。

火漆盖章只用缩放 1.08→1.00 与透明度完成，不得使用 back / elastic 缓动。动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若平台会另加字幕，可以关闭；若内嵌字幕，严格使用 `safe_area.subtitles`、两行上限和语义断行规则。

第 0 帧必须已经是完整封面：卷轴已完全展开、标题已写完、火漆已落定。标题稳定至少 18 帧后才允许转场，不得以黑帧、空白、卷起状态或淡入中间态开场。

## 声音

人声优先且保持原速。BGM 只建立克制的古典或民谣氛围；有人声时按 frontmatter 下压。SFX 只标记展开卷轴、盖章、解锁和关键转场，同时最多两个。

## 可变项

题材、图形隐喻、图标、节点数量、地图地形、局部构图和内容素材可以变化。不得复刻任何商业游戏的 Logo、角色、界面或专有图标。

## 验收

制作完成后检查：

1. 核心颜色、字体、圆角和间距是否来自 frontmatter。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 关键文字是否避让平台 UI。
4. 标签、印章和图标距边缘是否至少 24px。
5. 列表、路径和进度条底部是否至少留 24px。
6. 纸纹 `feTurbulence` 是否固定 seed 且不随时间变化。
7. 第 0 帧是否完成、可读且稳定 18 帧。
8. 动画是否可寻址、确定性、无同质化入场，成片声音是否达到 loudness 和 true-peak 目标。
