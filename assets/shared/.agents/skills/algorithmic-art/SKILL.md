---
name: algorithmic-art
description: 在 HyperFrames 组合中创建算法艺术/生成式视觉 — 确定性种子驱动、canvas/SVG/WebGL 渲染、完美循环动画、音频响应式视觉。当用户提到生成式艺术、算法艺术、流场、粒子系统、吸引子、tiling、分形、art loop、reel 时使用。
---

# Algorithmic Art for HyperFrames

在 HyperFrames 组合中制作生成式艺术。所有随机性来自一个种子，同一种子 + 同一组参数 = 像素级一致的输出。

## 工作流

**1. 设计意图 — 三句话，然后停。**
写出来：结构/主题、情绪/配色方向、驱动作品的那一个算法想法。加一行说明它为什么不像默认生成式艺术。不要写宣言。

**2. 选渲染方式。** 根据需求选择：

| 需求 | 方式 | 参考 |
|---|---|---|
| 2D 静态或交互动画 | HTML canvas，作为 HyperFrames scene | `references/techniques.md` |
| 线条艺术、激光切割、CNC | SVG 路径，真实单位 | `references/export.md` |
| 实时 3D、shader、bloom/glow | three.js / raw WebGL | `references/webgl.md` |
| 循环动画、视频内容 | canvas 帧 → HyperFrames 渲染 | `references/animation.md` |
| 音频响应式 | 离线特征提取 → 逐帧参数 | `references/animation.md` |

**3. 构建。** 以 `templates/viewer.html` 为起点构建 scene — 它包含完整引擎（种子 PRNG、噪声、自动参数 UI、种子导航、高清导出、帧捕获钩子）。你只需替换 `ART` 对象。遵守下面的不变量。

**4. 看输出。必须。**
渲染作品并实际查看 — 截图、导出 PNG 并读取。按清单审查，修改，再看。交付前至少两轮审查循环。永远不要交付没见过的作品。

**5. 交付。** 作为 HyperFrames scene 交付，报告最佳变体的种子。

## 不变量 — 每件作品、每种媒介

- **确定性。** 所有随机性从一个种子流出。同一种子 + 同一组参数 = 像素级一致的输出。永远不要在艺术路径中调用未播种的 `Math.random()` 或读取时钟。
- **一个 params 对象。** 所有可调参数在一个 spec 中，带 min/max/step/default。模板自动从中构建 UI。参数是*系统的属性*（密度、湍流、对比度），不是"图案类型"开关。
- **反默认规则。** 选择算法前先读 `references/techniques.md`。反射性套路 — Perlin 流场、粒子轨迹、圆填充、递归树、普通 Voronoi — 作为唯一想法被禁止，除非明确要求。要么使用该列表之外的技术，要么组合两个正交技术。
- **色彩是设计的，不是采样的。** 读 `references/palettes.md`。永远不要原始 RGB 插值。先建立明度结构（明暗层次），再定色相。有限调色板胜过彩虹。
- **干净的输出。** 不在作品本身烘焙文字、签名或水印。UI 可以有标题；画布/导出永远没有。
- **高性能。** 交互作品保持 60fps 或使用 `noLoop` 风格的静态渲染。限制元素数量；优先 O(n) 遍历和空间哈希。

## 审查清单（第 4 步）

对渲染图诚实评分：

- **明度结构** — 眯眼看（或心理上缩小到 64px）：有清晰的明暗构图，还是均匀的糊状？
- **焦点层次** — 眼睛落在某处，然后移动？还是兴趣均匀涂抹？
- **密度变化** — 有休息区和密集区？还是从头到尾噪声汤/大部分空白？
- **边缘行为** — 作品有意识地结束（渐晕、出血、边距、构图感知边框）还是直接裁切？
- **色彩纪律** — 有限调色板、和谐邻色、一个重音在工作？还是随机色相？
- **AI 感** — 它看起来像默认 AI 生成艺术吗（均匀的 Perlin 漩涡、等距粒子、无张力的居中径向对称）？如果是，换算法，不要只改参数。
- **种子鲁棒性** — 检查 3+ 个种子。好的系统产生*不同的好*图像，不是一个好种子加垃圾兄弟。

先修最差的失败项，重新渲染，重新评分。

## 与 HyperFrames 的集成

在 HyperFrames scene 中使用生成式艺术：

1. **作为 scene 目录中的 HTML 文件** — 在 scene 目录中创建 `composition.html`，包含完整的 viewer 引擎和你的 ART 对象
2. **通过 `__art.renderFrame` 钩子** — HyperFrames 可以调用 `window.__art.renderFrame(n, total)` 来逐帧渲染
3. **配色与风格规范一致** — 生成式作品的调色板必须来自项目的 `frame.md` 风格规范
4. **画布尺寸匹配** — 使用项目预设的画布尺寸（如 1080×1440），不要自由选择

## 文件

- `templates/viewer.html` — 自包含 viewer/引擎。暗色 chrome，零依赖，无 CDN（离线和严格 CSP 下工作）。只替换 `ART` 对象。
- `references/techniques.md` — 算法广度目录
- `references/palettes.md` — 色彩规则、OKLab 插值、起始调色板
- `references/animation.md` — 完美循环、帧捕获、ffmpeg 配方、音频响应式
- `references/export.md` — 打印分辨率 PNG、SVG（plotter/laser/CNC）
- `references/webgl.md` — three.js 和 shader 指导
