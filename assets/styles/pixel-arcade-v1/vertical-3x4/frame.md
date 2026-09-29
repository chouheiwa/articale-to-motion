---
schema_version: 1
style_id: pixel-arcade-v1
style_name: 像素街机
scope:
  - retro_game_culture
  - nostalgia_and_history
  - beginner_science_explainer
  - tech_basics_for_newcomers
  - gamified_learning
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
  canvas: "#1B1B2F"
  panel: "#2B2B52"
  shadow: "#0B0B16"
  ink: "#FFF1E8"
  frame_gray: "#C2C3C7"
  muted: "#A6A6C8"
  coin_yellow: "#FFEC27"
  p1_blue: "#29ADFF"
  p2_orange: "#FFA300"
  power_green: "#00E436"
  hp_red: "#FF4D6D"
typography:
  primary_stack: '"Fusion Pixel 12px Prop zh-Hans", "Noto Sans SC", sans-serif'
  mono_stack: '"Press Start 2P", "Silkscreen", "Fusion Pixel 12px Prop zh-Hans", monospace'
  font_files:
    - {family: "Noto Sans SC", weight: 400, file: "assets/fonts/noto-sans-sc-400.woff2"}
    - {family: "Noto Sans SC", weight: 600, file: "assets/fonts/noto-sans-sc-600.woff2"}
    - {family: "Noto Sans SC", weight: 700, file: "assets/fonts/noto-sans-sc-700.woff2"}
    - {family: "Noto Sans SC", weight: 900, file: "assets/fonts/noto-sans-sc-900.woff2"}
    - {family: "Fusion Pixel 12px Prop zh-Hans", weight: 400, file: "assets/fonts/fusion-pixel-12px-zh_hans.woff2"}
    - {family: "Press Start 2P", weight: 400, file: "assets/fonts/press-start-2p-400.woff2"}
    - {family: "Silkscreen", weight: 400, file: "assets/fonts/silkscreen-400.woff2"}
  sizes_px:
    cover_min: 72
    cover_max: 96
    title_min: 48
    title_max: 72
    body_min: 36
    body_max: 48
    body_long_min: 28
    body_long_max: 34
    label_min: 24
    label_max: 32
    pixel_grid_unit: 12
    latin_pixel_grid_unit: 8
  weights:
    display: 400
    heading: 400
    body: 400
    body_emphasis: 400
    body_long: 400
    body_long_emphasis: 700
    metadata: 400
  line_heights:
    display_min: 1.17
    display_max: 1.34
    body: 1.34
    body_long: 1.45
    metadata: 1.5
spacing:
  pixel_unit_px: 6
  content_left_px: 88
  content_width_px: 904
  group_min_px: 12
  group_max_px: 24
  card_stack_min_px: 18
  card_main_padding_min_px: 30
  card_main_padding_max_px: 42
  card_secondary_padding_min_px: 24
  card_secondary_padding_max_px: 30
  section_min_px: 36
  section_max_px: 60
  element_edge_min_px: 24
  skeleton_bottom_min_px: 24
  border_px: 6
  hard_shadow_offset_px: 12
radius:
  sm_px: 0
  md_px: 0
  lg_px: 0
  xl_px: 0
  corner_notch_px: 6
  pill_px: 999
scene_archetypes:
  - id: proposition
    name: 开机标题
    use_for: 封面、钩子、章节开场、强结论（像开机画面或关卡标题卡）
    example_png: assets/style-guide/examples/proposition.png
  - id: comparison
    name: 双人选角
    use_for: 两种方案、旧与新、条件与结果的对照（1P / 2P 选角界面）
    example_png: assets/style-guide/examples/comparison.png
  - id: process
    name: 关卡地图
    use_for: 步骤、阶段、因果链、成长路线（世界地图上逐关解锁）
    example_png: assets/style-guide/examples/process.png
  - id: capability_deck
    name: 道具栏
    use_for: 系统组成、能力清单、工具箱、最终成果（背包格与道具说明框）
    example_png: assets/style-guide/examples/capability_deck.png
motion:
  phases: {build: 0.30, breathe: 0.45, resolve: 0.25}
  entrance_seconds: {min: 0.20, max: 0.50}
  exit_seconds: {min: 0.13, max: 0.33}
  transition_seconds: {min: 0.20, max: 0.40}
  first_action_delay_seconds: {min: 0.10, max: 0.30}
  entrance_ease: ["steps(3)", "steps(4)", "steps(6)", "steps(8)"]
  exit_ease: ["steps(2)", "steps(3)", "steps(4)"]
  typewriter_chars_per_second: {min: 12, max: 24}
  cursor_blink_period_seconds: {min: 0.50, max: 0.80}
  count_up_steps: {min: 6, max: 12}
  verbs: [POP_IN, TYPEWRITE, BLINK, STEP, COUNT_UP, UNLOCK, POWER_UP, SELECT]
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
  font_size_px: {min: 36, max: 36}
  max_width_px: 812
  max_lines: 2
  max_fullwidth_chars_per_line: 16
  line_height: {min: 1.34, max: 1.50}
  background: "rgba(11, 11, 22, 0.88)"
  text_color: "#FFF1E8"
  padding_px: {vertical: 12, horizontal: 24}
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
  max_fullwidth_chars_per_line: 10
  font_size_px: {min: 72, max: 96}
  line_height: {min: 1.17, max: 1.34}
  max_width_px: 812
  contrast_ratio_min: 7.0
  stable_frames: 18
forbidden:
  - black_or_blank_frame_zero
  - cover_fade_in_intermediate_state
  - copying_commercial_game_logos_characters_or_ui
  - trademarked_console_or_controller_likeness
  - pixel_font_at_non_integer_multiple_size
  - antialiased_blurry_pixel_edges
  - scaled_pixel_art_with_smoothing
  - rounded_corners_on_pixel_frames
  - soft_blur_shadows_or_glow
  - gradient_fills_or_gradient_text
  - smooth_elastic_or_bouncy_easing
  - motion_blur_or_opacity_crossfade_as_main_motion
  - heavy_crt_curvature_or_chromatic_aberration
  - scanlines_over_critical_text_above_10_percent
  - arbitrary_new_palette_colors
  - long_body_text_in_pixel_font
  - red_text_on_panel
  - more_than_one_ambient_motion
  - sfx_on_every_element
  - music_masking_voice
---# 像素街机

这是复古游戏、怀旧与入门科普类竖屏 MG 视频的品牌层规范。YAML frontmatter 是唯一规范性 token；正文解释如何使用。

## 使用原则

- 品牌规范不是固定布局。根据文案语义选择开机标题、双人选角、关卡地图或道具栏镜头。
- 渲染工具负责镜头内部的图形隐喻、局部构图和动画编排，但不得改写核心颜色、字体角色、像素网格、安全区、间距、动作语法和声音目标。
- 同一时刻只有一个主焦点；整镜最多两个焦点，按节拍依次登场，并包含背景、中景和前景三个层次。
- 游戏界面是讲解的容器，不是装饰。对话框承载一句话，选项菜单承载选择，分数与血条承载真实数字，关卡承载真实步骤。

## 品牌人格

好玩、直接、有节奏、怀旧但不油腻。画面像一台刚通电的街机：标题逐格出现，光标闪烁等待选择，关卡一格一格被点亮，道具被收进背包。

## 像素规则

- 所有图形按 6px 像素单位绘制：边框 6px，硬投影向右下偏移 12px，形状只用直角与 6px 切角，`radius` 除 `pill_px` 外一律为 0。
- 中文像素字 `Fusion Pixel 12px Prop zh-Hans` 只能用 12 的整数倍字号：36、48、72、96px；拉丁像素字 `Press Start 2P` 与 `Silkscreen` 只能用 8 的整数倍：24、32、48px。非整数倍会让像素糊边，属于禁用项。
- 像素字只放短文本（标题、选项、标签、一句对白）。超过两行的说明文字改用 `Noto Sans SC` 28–34px。像素中文字体缺少部分二级汉字，出现缺字时整句换成 `Noto Sans SC`，不要混排单字。
- CSS 中对像素图形与像素字使用 `image-rendering: pixelated` 与 `-webkit-font-smoothing: none`，SVG 使用 `shape-rendering="crispEdges"`。
- 禁止模糊阴影、发光、渐变与圆角；深度只靠硬投影和面板叠层表达。

## 构图

- 背景层：深色画布上的像素星点、地平线网格或弱扫描线（透明度不超过 10%，不得压在关键文字上）。
- 中景层：对话框、选项菜单、地图路径、背包格等界面框。
- 前景层：光标、分数、血条、道具图标和被选中的高亮。
- 标题、数字、结论和核心术语使用 `critical_text` 区，主动避让抖音右侧互动栏和底部 UI；底部字幕带保持干净。
- 金币黄只标记当前焦点（光标、分数、选中项）；红色只表示风险与失败，且不作为面板上的文字色。

## 动画

遵循 Build / Breathe / Resolve。入场以 POP_IN、TYPEWRITE、BLINK、STEP、COUNT_UP、UNLOCK、POWER_UP、SELECT 为主要动作词。缓动只用 `steps(n)` 阶梯函数，位移按像素单位跳格，不使用平滑、弹性或回弹缓动。打字机每秒 12–24 字；计数跳 6–12 格落定。

每镜头最多一种环境动作（光标闪烁、星点闪烁或扫描线二选一），信息出齐后允许画面完全静止、稳定可读。

动画必须确定性、可寻址，并可在任意帧重建。不得使用运行时随机数、墙钟时间或依赖播放顺序的状态。

## 字幕与封面

字幕是可选层。若内嵌字幕，使用 `safe_area.subtitles`、36px 像素字、深色直角底框、两行上限和语义断行规则。

第 0 帧必须已经是完整封面：标题、面板和光标都已就位。标题稳定至少 18 帧后才允许转场，不得以黑帧、「INSERT COIN」空屏或逐字打出的中间态开场。

## 声音

人声优先且保持原速。BGM 可用芯片音乐质感但要克制；有人声时按 frontmatter 下压。SFX（投币、确认、升级、计分）只标记钩子、选择、解锁和结论，同时最多两个。

## 可变项

题材、像素图标、关卡数量、背包格数量、局部构图和内容素材可以变化。不得复刻任何商业游戏的 Logo、角色、道具造型或专有界面；需要游戏引用时只用文字说明。

## 验收

1. 核心颜色、字体、字号倍数和间距是否来自 frontmatter。
2. `typography.font_files` 里用到的每个字重是否都有对应的 `@font-face`，`src` 是否指向镜头目录内真实存在的文件，是否都是 `font-display: block`。渲染机不装系统字体，漏一条就会静默回退成通用字体，本地却看不出来。
3. 像素字是否全部使用整数倍字号且边缘锐利，无模糊、无圆角、无渐变。
4. 关键文字是否避让平台 UI，标签与图标距边缘至少 24px。
5. 第 0 帧是否完成、可读且稳定 18 帧。
6. 动画是否只用阶梯缓动、可寻址、确定性，环境动作不超过一种。
7. 成片声音是否达到 loudness 和 true-peak 目标。
