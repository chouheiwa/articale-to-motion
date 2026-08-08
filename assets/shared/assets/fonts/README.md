# 项目自带字体

HyperFrames 只会自动嵌入它内置清单里的字体族。**清单中没有任何简体中文字体**，
唯一的 CJK 项 `Noto Sans JP` 缺失大量简体字（实测本系列文案 56 个汉字中缺 19 个），
不能用作替代。

系统字体（`PingFang SC`、`Hiragino Sans GB` 等）同样不能写进字体栈：渲染机是一台干净的
无头 Chrome，上面没有装这些字体，文字会静默回退成通用字体，MP4 里的排版就错了。
在装有这些字体的 macOS 上本地渲染看着正常，是回退掩盖了问题，不是它可用。

因此中文字形必须由本目录的字体文件提供，并在每个镜头的 composition 里用 `@font-face`
指向真实文件。

## 文件

| 文件 | 字重 | 对应 `frame.md` 的 `typography.weights` |
| --- | ---: | --- |
| `noto-sans-sc-400.woff2` | 400 | `body` |
| `noto-sans-sc-600.woff2` | 600 | `body_emphasis` |
| `noto-sans-sc-700.woff2` | 700 | `heading`、`metadata` |
| `noto-sans-sc-900.woff2` | 900 | `display` |

来源：`@fontsource/noto-sans-sc` 5.3.0 的 `chinese-simplified` 子集。
许可证：SIL Open Font License 1.1，见 `LICENSE-Noto-Sans-SC.txt`，允许随项目分发。

拉丁字形由 `Inter` 提供，它在 HyperFrames 的内置清单里，会被自动嵌入，不需要自带文件。

## 在 composition 里怎么用

把本目录复制进镜头目录后，在 composition 的 `<style>` 里声明：

```css
@font-face {
  font-family: "Noto Sans SC";
  src: url("assets/fonts/noto-sans-sc-400.woff2") format("woff2");
  font-weight: 400;
  font-display: block;
}
/* 用到的每个字重都要单独声明一条，src 指向对应文件 */
```

`font-display: block` 是刻意选的：渲染器并行抽帧，`swap` 会让部分帧抓到回退字体。

## 更新字体

```bash
curl -sL -o /tmp/nssc.tgz "$(npm view @fontsource/noto-sans-sc dist.tarball)"
tar xzf /tmp/nssc.tgz -C /tmp package/files/noto-sans-sc-chinese-simplified-{400,600,700,900}-normal.woff2
for w in 400 600 700 900; do
  cp /tmp/package/files/noto-sans-sc-chinese-simplified-$w-normal.woff2 assets/fonts/noto-sans-sc-$w.woff2
done
```

换字体或增删字重后必须同步改 `frame.md` 与 `docs/清晰系统蓝图-视频风格说明书.md` 的
`typography.font_files`，两份文件的 token 必须逐字节一致，`am validate style` 会校验。
