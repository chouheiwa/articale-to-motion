package validate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	assets "github.com/chouheiwa/articale-to-motion"
	"github.com/chouheiwa/articale-to-motion/internal/preset"
	"github.com/chouheiwa/articale-to-motion/internal/styleimage"
	"gopkg.in/yaml.v3"
)

var secretValue = regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_-]{12,}|api[_-]?key\s*[:=]\s*[^\s"']+|password\s*[:=]\s*[^\s"']+)`)

func frontmatter(path string) (map[string]any, string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	parts := bytes.SplitN(body, []byte("---"), 3)
	if len(parts) != 3 || len(bytes.TrimSpace(parts[0])) != 0 {
		return nil, "", fmt.Errorf("%s 缺少 YAML frontmatter", path)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(parts[1], &node); err != nil {
		return nil, "", fmt.Errorf("yaml 解析失败：%w", err)
	}
	if err := validateTags(&node); err != nil {
		return nil, "", err
	}
	var result map[string]any
	if err := node.Decode(&result); err != nil {
		return nil, "", err
	}
	return result, string(parts[2]), nil
}

func validateTags(node *yaml.Node) error {
	allowed := map[string]bool{"": true, "!!map": true, "!!seq": true, "!!str": true, "!!int": true, "!!float": true, "!!bool": true, "!!null": true, "!!timestamp": true}
	if !allowed[node.Tag] {
		return fmt.Errorf("不安全的 YAML tag：%s", node.Tag)
	}
	for _, child := range node.Content {
		if err := validateTags(child); err != nil {
			return err
		}
	}
	return nil
}

func safeRelative(root string, value any) error {
	path, ok := value.(string)
	if !ok || path == "" {
		return nil
	}
	if filepath.IsAbs(path) || strings.Contains(path, "://") {
		return fmt.Errorf("路径必须位于项目内：%s", path)
	}
	root, _ = filepath.Abs(root)
	target, _ := filepath.Abs(filepath.Join(root, path))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	resolvedTarget, resolveErr := filepath.EvalSymlinks(target)
	if resolveErr != nil {
		resolvedParent, parentErr := filepath.EvalSymlinks(filepath.Dir(target))
		if parentErr == nil {
			resolvedTarget = filepath.Join(resolvedParent, filepath.Base(target))
		} else {
			resolvedTarget = target
		}
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("路径逃出项目目录：%s", path)
	}
	return nil
}

func Publish(path, projectRoot string, templateMode bool) (map[string]any, error) {
	data, body, err := frontmatter(path)
	if err != nil {
		return nil, err
	}
	required := []string{"schema_version", "workflow", "publish_status", "platform", "generated_at", "video", "cover", "copy", "credits", "manual_checks", "evidence"}
	for _, key := range required {
		if _, ok := data[key]; !ok {
			return nil, fmt.Errorf("缺少必填字段：%s", key)
		}
	}
	if data["schema_version"] != 1 {
		return nil, fmt.Errorf("schema_version 必须为 1")
	}
	statuses := map[string]bool{"draft": true, "pending_manual_checks": true, "blocked": true, "ready": true}
	status, _ := data["publish_status"].(string)
	if !statuses[status] {
		return nil, fmt.Errorf("无效的 publish_status：%v", data["publish_status"])
	}
	if secretValue.MatchString(string(mustRead(path))) {
		return nil, fmt.Errorf("publish.md 不得包含密钥")
	}
	video, ok := data["video"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("video 必须是对象")
	}
	if err := safeRelative(projectRoot, video["path"]); err != nil {
		return nil, err
	}
	if evidence, ok := data["evidence"].(map[string]any); ok {
		for _, value := range evidence {
			if values, ok := value.([]any); ok {
				for _, item := range values {
					if err := safeRelative(projectRoot, item); err != nil {
						return nil, err
					}
				}
			} else if err := safeRelative(projectRoot, value); err != nil {
				return nil, err
			}
		}
	}
	for _, heading := range []string{"## 发布状态", "## 发布平台", "## 主标题", "## 发布介绍", "## 话题标签", "## 封面文字", "## 素材署名", "## 成片规格与 SHA-256", "## 证据索引", "## 发布前人工检查", "## 未完成事项"} {
		if !strings.Contains(body, heading) {
			return nil, fmt.Errorf("markdown 缺少章节：%s", heading)
		}
	}
	_ = templateMode
	return data, nil
}

func mustRead(path string) []byte {
	body, _ := os.ReadFile(path)
	return body
}

func Style(projectRoot string) error {
	frame, _, err := frontmatter(filepath.Join(projectRoot, "frame.md"))
	if err != nil {
		return err
	}
	guide, _, err := frontmatter(filepath.Join(projectRoot, "docs", "清晰系统蓝图-视频风格说明书.md"))
	if err != nil {
		return err
	}
	active, err := validateStyleSchema(frame)
	if err != nil {
		return fmt.Errorf("frame.md: %w", err)
	}
	if _, err := validateStyleSchema(guide); err != nil {
		return fmt.Errorf("风格说明书: %w", err)
	}
	if !reflect.DeepEqual(frame, guide) {
		return fmt.Errorf("frame.md 与人类可读风格说明的 token 不一致")
	}
	for index, item := range frame["typography"].(map[string]any)["font_files"].([]any) {
		path := item.(map[string]any)["file"].(string)
		if err := safeRelative(projectRoot, path); err != nil {
			return fmt.Errorf("typography.font_files[%d].file: %w", index, err)
		}
		info, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(path)))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("typography.font_files[%d] 声明的字体文件不存在：%s", index, path)
		}
	}
	for _, item := range frame["scene_archetypes"].([]any) {
		entry := item.(map[string]any)
		path := entry["example_png"].(string)
		if err := safeRelative(projectRoot, path); err != nil {
			return fmt.Errorf("example_png: %w", err)
		}
		width, height, err := pngDimensions(filepath.Join(projectRoot, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		if int(width) != active.Canvas.WidthPx || int(height) != active.Canvas.HeightPx {
			return fmt.Errorf("风格示例必须为 %dx%d，实际 %dx%d：%s",
				active.Canvas.WidthPx, active.Canvas.HeightPx, width, height, path)
		}
	}
	return nil
}

// validateStyleSchema 校验风格 token，并返回该 frame.md 对应的画幅预设，
// 供调用方继续按画幅校验示例图尺寸。
func validateStyleSchema(tokens map[string]any) (preset.Preset, error) {
	required := []string{"schema_version", "style_id", "style_name", "scope", "canvas", "safe_area", "colors", "typography", "spacing", "radius", "scene_archetypes", "motion", "audio", "subtitles", "cover", "forbidden"}
	if err := requireKeys(tokens, required, ""); err != nil {
		return preset.Preset{}, err
	}
	if tokens["schema_version"] != 1 {
		return preset.Preset{}, fmt.Errorf("schema_version 必须为 1")
	}
	if _, ok := tokens["style_id"].(string); !ok {
		return preset.Preset{}, fmt.Errorf("style_id 必须是字符串")
	}
	if _, ok := tokens["style_name"].(string); !ok {
		return preset.Preset{}, fmt.Errorf("style_name 必须是字符串")
	}
	if scope, ok := tokens["scope"].([]any); !ok || len(scope) == 0 {
		return preset.Preset{}, fmt.Errorf("scope 必须是非空数组")
	}
	canvas, err := requireMap(tokens["canvas"], "canvas")
	if err != nil {
		return preset.Preset{}, err
	}
	width, widthOK := canvas["width_px"].(int)
	height, heightOK := canvas["height_px"].(int)
	fps, fpsOK := canvas["fps"].(int)
	orientation, orientationOK := canvas["orientation"].(string)
	if !widthOK || !heightOK || !fpsOK || !orientationOK {
		return preset.Preset{}, fmt.Errorf("canvas 必须包含整数 width_px、height_px、fps 与字符串 orientation")
	}
	active, ok := preset.ByCanvas(width, height, fps, orientation)
	if !ok {
		return preset.Preset{}, fmt.Errorf("canvas %dx%d、%dfps、%s 不是内置画幅，可选：%s",
			width, height, fps, orientation, strings.Join(preset.IDs(), " "))
	}
	safeArea, err := requireMap(tokens["safe_area"], "safe_area")
	if err != nil {
		return preset.Preset{}, err
	}
	// 逐值比对 anchor 推导结果，而不只是检查字段齐全非负：手改一个 top_px
	// 在旧校验下完全查不出来，成片才会发现字幕压线。
	expected, err := active.ResolveSafeArea()
	if err != nil {
		return preset.Preset{}, err
	}
	if len(safeArea) != len(expected) {
		return preset.Preset{}, fmt.Errorf("safe_area 必须恰好包含 %d 个安全区，实际 %d 个", len(expected), len(safeArea))
	}
	for _, item := range expected {
		box, err := requireMap(safeArea[item.Name], "safe_area."+item.Name)
		if err != nil {
			return preset.Preset{}, err
		}
		if err := requireKeys(box, []string{"left_px", "right_px", "top_px", "bottom_px"}, "safe_area."+item.Name); err != nil {
			return preset.Preset{}, err
		}
		for _, field := range []struct {
			name string
			want int
		}{
			{"left_px", item.Box.LeftPx}, {"right_px", item.Box.RightPx},
			{"top_px", item.Box.TopPx}, {"bottom_px", item.Box.BottomPx},
		} {
			got, ok := box[field.name].(int)
			if !ok {
				return preset.Preset{}, fmt.Errorf("safe_area.%s.%s 必须是整数", item.Name, field.name)
			}
			if got != field.want {
				return preset.Preset{}, fmt.Errorf("safe_area.%s.%s = %d，按 %s 画幅应为 %d",
					item.Name, field.name, got, active.ID, field.want)
			}
		}
	}
	colors, err := requireMap(tokens["colors"], "colors")
	if err != nil || len(colors) == 0 {
		return preset.Preset{}, fmt.Errorf("colors 必须是非空对象")
	}
	for name, raw := range colors {
		value, ok := raw.(string)
		if !ok || !hexColor.MatchString(value) {
			return preset.Preset{}, fmt.Errorf("colors.%s 必须是六位十六进制颜色", name)
		}
	}
	typography, err := requireMap(tokens["typography"], "typography")
	if err != nil {
		return preset.Preset{}, err
	}
	if err := requireKeys(typography, []string{"primary_stack", "mono_stack", "sizes_px", "weights", "line_heights", "font_files"}, "typography"); err != nil {
		return preset.Preset{}, err
	}
	if err := validateFontStacks(typography); err != nil {
		return preset.Preset{}, err
	}
	for _, metric := range []struct {
		name string
		min  float64
	}{{"sizes_px", 1}, {"weights", 1}, {"line_heights", .5}} {
		mapping, err := requireMap(typography[metric.name], "typography."+metric.name)
		if err != nil {
			return preset.Preset{}, err
		}
		if err := validateNumericMap(mapping, "typography."+metric.name, metric.min); err != nil {
			return preset.Preset{}, err
		}
	}
	for _, name := range []string{"spacing", "radius"} {
		mapping, err := requireMap(tokens[name], name)
		if err != nil {
			return preset.Preset{}, err
		}
		if err := validateNumericMap(mapping, name, 0); err != nil {
			return preset.Preset{}, err
		}
	}
	radius := tokens["radius"].(map[string]any)
	if radius["pill_px"] != 999 {
		return preset.Preset{}, fmt.Errorf("radius.pill_px 必须为 999")
	}
	archetypes, ok := tokens["scene_archetypes"].([]any)
	canonical := []string{"proposition", "comparison", "process", "capability_deck"}
	if !ok || len(archetypes) != len(canonical) {
		return preset.Preset{}, fmt.Errorf("scene_archetypes 必须包含四种标准类型")
	}
	for index, item := range archetypes {
		entry, err := requireMap(item, fmt.Sprintf("scene_archetypes[%d]", index))
		if err != nil {
			return preset.Preset{}, err
		}
		if entry["id"] != canonical[index] {
			return preset.Preset{}, fmt.Errorf("scene_archetypes 必须按标准顺序声明")
		}
		if _, ok := entry["name"].(string); !ok {
			return preset.Preset{}, fmt.Errorf("scene_archetypes[%d].name 必须是字符串", index)
		}
		if _, ok := entry["example_png"].(string); !ok {
			return preset.Preset{}, fmt.Errorf("scene_archetypes[%d].example_png 必须是字符串", index)
		}
	}
	motion, err := requireMap(tokens["motion"], "motion")
	if err != nil {
		return preset.Preset{}, err
	}
	if err := requireKeys(motion, []string{"phases", "entrance_seconds", "exit_seconds", "transition_seconds", "entrance_ease", "exit_ease", "verbs"}, "motion"); err != nil {
		return preset.Preset{}, err
	}
	phases, err := requireMap(motion["phases"], "motion.phases")
	if err != nil {
		return preset.Preset{}, err
	}
	if err := validateNumericMap(phases, "motion.phases", 0); err != nil {
		return preset.Preset{}, err
	}
	total := 0.0
	for _, value := range phases {
		total += number(value)
	}
	if total < .999999 || total > 1.000001 {
		return preset.Preset{}, fmt.Errorf("motion.phases 之和必须为 1")
	}
	audio, err := requireMap(tokens["audio"], "audio")
	if err != nil {
		return preset.Preset{}, err
	}
	if err := requireKeys(audio, []string{"sample_rate_hz", "channels", "final_integrated_lufs", "final_lufs_tolerance", "true_peak_max_dbtp", "bgm_ducking_db", "bgm_fade_seconds", "sfx_max_simultaneous"}, "audio"); err != nil {
		return preset.Preset{}, err
	}
	if audio["sample_rate_hz"] != 48000 || audio["channels"] != 2 {
		return preset.Preset{}, fmt.Errorf("audio 必须为 48000Hz 双声道")
	}
	subtitles, err := requireMap(tokens["subtitles"], "subtitles")
	if err != nil {
		return preset.Preset{}, err
	}
	if err := requireKeys(subtitles, []string{"required", "font_size_px", "max_lines", "max_fullwidth_chars_per_line", "line_height", "background", "text_color", "padding_px", "radius_px", "break_rules"}, "subtitles"); err != nil {
		return preset.Preset{}, err
	}
	cover, err := requireMap(tokens["cover"], "cover")
	if err != nil {
		return preset.Preset{}, err
	}
	if err := requireKeys(cover, []string{"frame_zero_complete", "default_lines", "max_lines", "font_size_px", "max_width_px", "contrast_ratio_min", "stable_frames"}, "cover"); err != nil {
		return preset.Preset{}, err
	}
	stableFrames, stable := cover["stable_frames"].(int)
	if cover["frame_zero_complete"] != true || !stable || stableFrames < 18 {
		return preset.Preset{}, fmt.Errorf("cover 必须从第零帧完整显示并稳定至少 18 帧")
	}
	forbidden, ok := tokens["forbidden"].([]any)
	if !ok || len(forbidden) == 0 {
		return preset.Preset{}, fmt.Errorf("forbidden 必须是非空字符串数组")
	}
	for _, item := range forbidden {
		if _, ok := item.(string); !ok {
			return preset.Preset{}, fmt.Errorf("forbidden 必须是非空字符串数组")
		}
	}
	return active, nil
}

// autoEmbeddedFonts 是 HyperFrames 0.7.94 会自动下载并内联的字体族
// （CANONICAL_FONTS，背后是 @fontsource/* 包）。清单里没有任何简体中文字体：
// 唯一的 CJK 项 Noto Sans JP 缺失大量简体字，不能当替代品。
var autoEmbeddedFonts = map[string]bool{
	"archivo black": true, "eb garamond": true, "ibm plex mono": true, "inter": true,
	"jetbrains mono": true, "lato": true, "league gothic": true, "montserrat": true,
	"noto sans jp": true, "nunito": true, "open sans": true, "oswald": true,
	"outfit": true, "playfair display": true, "poppins": true, "roboto": true,
	"source code pro": true, "space mono": true,
}

// genericFontFamilies 是 CSS 内建的兜底关键字，不需要字体文件。
var genericFontFamilies = map[string]bool{
	"sans-serif": true, "serif": true, "monospace": true, "cursive": true,
	"fantasy": true, "system-ui": true, "ui-sans-serif": true, "ui-serif": true,
	"ui-monospace": true, "emoji": true, "math": true, "fangsong": true,
}

func fontStackFamilies(stack string) []string {
	parts := strings.Split(stack, ",")
	families := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.ToLower(strings.Trim(strings.TrimSpace(part), `"'`))
		if name != "" {
			families = append(families, name)
		}
	}
	return families
}

// validateFontStacks 保证字体栈里的每个字体族要么被 HyperFrames 自动内联，
// 要么由项目自带文件。渲染机是干净的无头 Chrome：写一个只存在于本机的系统字体
// （PingFang SC、Hiragino Sans GB 等），本地渲染会因为回退而"看着正常"，
// 云端和 CI 上排版却是错的，而且没有任何报错。
func validateFontStacks(typography map[string]any) error {
	declared := map[string]bool{}
	entries, ok := typography["font_files"].([]any)
	if !ok || len(entries) == 0 {
		return fmt.Errorf("typography.font_files 必须是非空数组")
	}
	for index, item := range entries {
		entry, err := requireMap(item, fmt.Sprintf("typography.font_files[%d]", index))
		if err != nil {
			return err
		}
		family, familyOK := entry["family"].(string)
		file, fileOK := entry["file"].(string)
		weight := number(entry["weight"])
		if !familyOK || family == "" || !fileOK || file == "" || weight < 1 {
			return fmt.Errorf("typography.font_files[%d] 必须声明 family、weight 和 file", index)
		}
		declared[strings.ToLower(family)] = true
	}
	used := map[string]bool{}
	for _, key := range []string{"primary_stack", "mono_stack"} {
		stack, ok := typography[key].(string)
		if !ok || stack == "" {
			return fmt.Errorf("typography.%s 必须是非空字符串", key)
		}
		for _, family := range fontStackFamilies(stack) {
			if genericFontFamilies[family] {
				continue
			}
			used[family] = true
			if autoEmbeddedFonts[family] || declared[family] {
				continue
			}
			return fmt.Errorf("typography.%s 使用了既不会被 HyperFrames 自动内联、"+
				"也没有在 typography.font_files 中自带文件的字体：%s（渲染机没有本机系统字体，"+
				"排版会静默回退）", key, family)
		}
	}
	for family := range declared {
		if !used[family] {
			return fmt.Errorf("typography.font_files 声明了字体 %s，但两个字体栈都没有使用它", family)
		}
	}
	return nil
}

func requireMap(value any, name string) (map[string]any, error) {
	mapping, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s 必须是对象", name)
	}
	return mapping, nil
}

func requireKeys(mapping map[string]any, keys []string, name string) error {
	for _, key := range keys {
		if _, ok := mapping[key]; !ok {
			if name == "" {
				return fmt.Errorf("缺少必填字段：%s", key)
			}
			return fmt.Errorf("缺少必填字段：%s.%s", name, key)
		}
	}
	return nil
}

func number(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case float64:
		return typed
	default:
		return -1
	}
}

func validateNumericMap(mapping map[string]any, name string, minimum float64) error {
	for key, value := range mapping {
		numeric := number(value)
		if math.IsNaN(numeric) || math.IsInf(numeric, 0) || numeric < minimum {
			return fmt.Errorf("%s.%s 必须是大于等于 %g 的数字", name, key, minimum)
		}
	}
	return nil
}

func pngDimensions(path string) (uint32, uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	header := make([]byte, 24)
	if _, err := io.ReadFull(file, header); err != nil {
		return 0, 0, fmt.Errorf("读取 PNG %s: %w", path, err)
	}
	if !bytes.Equal(header[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return 0, 0, fmt.Errorf("不是 PNG：%s", path)
	}
	return binary.BigEndian.Uint32(header[16:20]), binary.BigEndian.Uint32(header[20:24]), nil
}

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// RegenerateExamples recreates the generic style SVGs and renders their PNG
// counterparts with ImageMagick. The embedded SVGs are immutable templates,
// so regeneration never depends on files left by an earlier run.
func RegenerateExamples(projectRoot string, output io.Writer) error {
	frame, _, err := frontmatter(filepath.Join(projectRoot, "frame.md"))
	if err != nil {
		return err
	}
	colors, ok := frame["colors"].(map[string]any)
	if !ok {
		return fmt.Errorf("frame.md 的 colors 必须是对象")
	}
	baseline := map[string]string{
		"canvas": "#F5F7FB", "ink": "#0E2340", "engineering_blue": "#1857C4",
		"capability_deck": "#12294A", "blue_tint": "#EAF1FD", "support_gray": "#4E6076",
		"structure_line": "#D5DEEB", "structure_line_strong": "#C3D2E6", "white": "#FFFFFF",
	}
	for name := range baseline {
		value, ok := colors[name].(string)
		if !ok || !hexColor.MatchString(value) {
			return fmt.Errorf("frame.md 的 colors.%s 必须是六位十六进制颜色", name)
		}
	}
	// 示例图模板按画幅分目录，必须从项目自己的 canvas 反查，不能取默认预设：
	// 拿 3:4 的模板铺进 9:16 项目，Style 会立刻拒绝，但用户看到的是尺寸报错而
	// 不是「模板取错了」。
	canvas, ok := frame["canvas"].(map[string]any)
	if !ok {
		return fmt.Errorf("frame.md 的 canvas 必须是对象")
	}
	width, _ := canvas["width_px"].(int)
	height, _ := canvas["height_px"].(int)
	fps, _ := canvas["fps"].(int)
	orientation, _ := canvas["orientation"].(string)
	active, ok := preset.ByCanvas(width, height, fps, orientation)
	if !ok {
		return fmt.Errorf("frame.md 的画幅 %dx%d 不是内置画幅，无法定位示例模板", width, height)
	}
	outputDir := filepath.Join(projectRoot, "assets", "style-guide", "examples")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	names := []string{"proposition", "comparison", "process", "capability_deck"}
	pngPaths := make([]string, 0, len(names))
	for _, name := range names {
		sourcePath := "assets/presets/" + active.ID + "/assets/style-guide/examples/" + name + ".svg"
		source, readErr := assets.Files.ReadFile(sourcePath)
		if readErr != nil {
			return fmt.Errorf("读取内置示例 %s: %w", name, readErr)
		}
		svg := string(source)
		for colorName, original := range baseline {
			svg = strings.ReplaceAll(svg, original, colors[colorName].(string))
		}
		svgPath := filepath.Join(outputDir, name+".svg")
		pngPath := filepath.Join(outputDir, name+".png")
		if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
			return err
		}
		if err := styleimage.RenderSVG(svgPath, pngPath); err != nil {
			return err
		}
		fmt.Fprintf(output, "generated %s\ngenerated %s\n", filepath.ToSlash(filepath.Join("assets", "style-guide", "examples", name+".svg")), filepath.ToSlash(filepath.Join("assets", "style-guide", "examples", name+".png")))
		pngPaths = append(pngPaths, pngPath)
	}
	contactPath := filepath.Join(outputDir, "contact-sheet.png")
	const thumbnailWidthPx = 405
	thumb := fmt.Sprintf("%dx%d", thumbnailWidthPx, thumbnailWidthPx*active.Canvas.HeightPx/active.Canvas.WidthPx)
	if err := styleimage.ContactSheet(pngPaths, contactPath, thumb, colors["structure_line"].(string)); err != nil {
		return err
	}
	fmt.Fprintf(output, "generated %s\n", filepath.ToSlash(filepath.Join("assets", "style-guide", "examples", "contact-sheet.png")))
	return nil
}
