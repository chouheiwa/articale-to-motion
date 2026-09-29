# 风格字体池

这里放各风格独有的字体。它**不是**源树：`am init` 只按所选风格在
`internal/preset/style.go` 里声明的 `Fonts`，把用到的字体和对应许可证拷进项目的
`assets/fonts/`。所有风格共用的正文字体 Noto Sans SC 在 `assets/shared/assets/fonts/`。

字体表（文件名、族名、字重、许可证）的唯一真相源是 `internal/preset/fonts.go`。
新增字体要同时改那里和本目录；`fontpool_test.go` 会拦下没人用的字体、缺许可证的
字体，以及 `frame.md` 的 `font_files` 与风格表对不上的情况。

## 许可

全部为 SIL Open Font License 1.1，许可证以 `LICENSE-<名称>.txt` 随字体一起下发。

带保留字体名（RFN）的字体只做无损的 WOFF2 封装，不子集化、不改名：OFL 下子集化
属于修改，修改后不能再用原名。

| 文件 | 族名 | 来源 | 处理 |
|---|---|---|---|
| `noto-serif-sc-400/700.woff2` | Noto Serif SC | `@fontsource/noto-serif-sc` 5.3.0 `chinese-simplified` | 族名表修正（见下） |
| `lxgw-wenkai-400.woff2` | LXGW WenKai | [lxgw/LxgwWenKai](https://github.com/lxgw/LxgwWenKai) v1.522 `LXGWWenKai-Regular.ttf` | GB2312 子集。RFN 附加许可明确允许为网页分发子集化并保留原名 |
| `xiaolai-400.woff2` | Xiaolai | [lxgw/kose-font](https://github.com/lxgw/kose-font) v3.126 `Xiaolai-Regular.ttf` | GB2312 子集（无 RFN） |
| `smiley-sans-oblique.woff2` | Smiley Sans | [atelier-anchor/smiley-sans](https://github.com/atelier-anchor/smiley-sans) v2.0.1 `SmileySans-Oblique.ttf.woff2` | 原样（RFN：Smiley、得意黑） |
| `zcool-kuaile-400.woff2` | ZCOOL KuaiLe | `@fontsource/zcool-kuaile` 5.3.0 `chinese-simplified` | 原样 |
| `zcool-qingke-huangyou-400.woff2` | ZCOOL QingKe HuangYou | `@fontsource/zcool-qingke-huangyou` 5.3.0 `chinese-simplified` | 原样 |
| `fusion-pixel-12px-zh_hans.woff2` | Fusion Pixel 12px Prop zh-Hans | [TakWolf/fusion-pixel-font](https://github.com/TakWolf/fusion-pixel-font) 2026.09.25 `12px-proportional-zh_hans.otf.woff2` | 原样。字体本身为 OFL，仓库里的 MIT 只覆盖构建程序 |
| `press-start-2p-400.woff2` | Press Start 2P | google/fonts `ofl/pressstart2p` | TTF 无损转 WOFF2（RFN） |
| `orbitron-variable.woff2` | Orbitron | google/fonts `ofl/orbitron` 可变字体 | TTF 无损转 WOFF2（RFN），含全部字重 |
| `chakra-petch-700.woff2` 等拉丁字体 | 见 `fonts.go` | `@fontsource/<名称>` 5.3.0 `latin` | 原样；Fredoka、Oxanium、Space Grotesk 族名表修正 |

**族名表修正**：fontsource 从可变字体实例化出的静态字体，name 表里的族名会带上
实例名（`Noto Serif SC ExtraLight`、`Fredoka Light`、`Oxanium ExtraLight`、
`Space Grotesk Light`）。这几款都没有 RFN，已把 name 表的 1/2/4/6/16/17 项改成
正确的族名与子族名，示例 SVG 经 fontconfig 才能按正常族名找到它们。

**GB2312 子集**：`pyftsubset <ttf> --text-file=<GB2312 全部 7445 字 + ASCII + 全角标点>
--flavor=woff2 --layout-features='*'`。

## 已排除

- **Zpix 最像素**：商业使用需付费授权，并禁止修改与格式转换。
- **方舟像素字体 12px**（单独使用）：缺 172 个一级常用字；缝合像素字体以它为基础补齐。
- **站酷高端黑 / 酷黑 / 文艺体等**：上游没有 OFL 文本可核实，只收经 Google Fonts
  发布、带 OFL.txt 的快乐体与庆科黄油体。

## 使用注意

- 像素字体只能按整数倍使用：12px 字形用 36 / 48 / 72 / 96px，并关闭字体平滑。
  缝合像素字体缺 143 个 GB2312 二级字，渲染前要逐字检查覆盖，不得静默回退。
- `font-display: block`：渲染器并行抽帧，`swap` 会让部分帧抓到回退字体。
