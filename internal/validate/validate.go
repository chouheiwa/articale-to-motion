package validate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	assets "github.com/chouheiwa/articale-to-motion"
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
	if err := validateStyleSchema(frame); err != nil {
		return fmt.Errorf("frame.md: %w", err)
	}
	if err := validateStyleSchema(guide); err != nil {
		return fmt.Errorf("风格说明书: %w", err)
	}
	if !reflect.DeepEqual(frame, guide) {
		return fmt.Errorf("frame.md 与人类可读风格说明的 token 不一致")
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
		if width != 1080 || height != 1440 {
			return fmt.Errorf("风格示例必须为 1080x1440：%s", path)
		}
	}
	return nil
}

func validateStyleSchema(tokens map[string]any) error {
	required := []string{"schema_version", "style_id", "style_name", "scope", "canvas", "safe_area", "colors", "typography", "spacing", "radius", "scene_archetypes", "motion", "audio", "subtitles", "cover", "forbidden"}
	if err := requireKeys(tokens, required, ""); err != nil {
		return err
	}
	if tokens["schema_version"] != 1 {
		return fmt.Errorf("schema_version 必须为 1")
	}
	if _, ok := tokens["style_id"].(string); !ok {
		return fmt.Errorf("style_id 必须是字符串")
	}
	if _, ok := tokens["style_name"].(string); !ok {
		return fmt.Errorf("style_name 必须是字符串")
	}
	if scope, ok := tokens["scope"].([]any); !ok || len(scope) == 0 {
		return fmt.Errorf("scope 必须是非空数组")
	}
	canvas, err := requireMap(tokens["canvas"], "canvas")
	if err != nil {
		return err
	}
	if canvas["width_px"] != 1080 || canvas["height_px"] != 1440 || canvas["fps"] != 30 || canvas["orientation"] != "vertical" {
		return fmt.Errorf("canvas 必须为 1080x1440、30fps、vertical")
	}
	safeArea, err := requireMap(tokens["safe_area"], "safe_area")
	if err != nil {
		return err
	}
	for _, name := range []string{"structural", "main_content", "critical_text", "cover_title", "subtitles"} {
		box, err := requireMap(safeArea[name], "safe_area."+name)
		if err != nil {
			return err
		}
		if err := requireKeys(box, []string{"left_px", "right_px", "top_px", "bottom_px"}, "safe_area."+name); err != nil {
			return err
		}
		if err := validateNumericMap(box, "safe_area."+name, 0); err != nil {
			return err
		}
	}
	colors, err := requireMap(tokens["colors"], "colors")
	if err != nil || len(colors) == 0 {
		return fmt.Errorf("colors 必须是非空对象")
	}
	for name, raw := range colors {
		value, ok := raw.(string)
		if !ok || !hexColor.MatchString(value) {
			return fmt.Errorf("colors.%s 必须是六位十六进制颜色", name)
		}
	}
	typography, err := requireMap(tokens["typography"], "typography")
	if err != nil {
		return err
	}
	if err := requireKeys(typography, []string{"primary_stack", "mono_stack", "sizes_px", "weights", "line_heights"}, "typography"); err != nil {
		return err
	}
	for _, metric := range []struct {
		name string
		min  float64
	}{{"sizes_px", 1}, {"weights", 1}, {"line_heights", .5}} {
		mapping, err := requireMap(typography[metric.name], "typography."+metric.name)
		if err != nil {
			return err
		}
		if err := validateNumericMap(mapping, "typography."+metric.name, metric.min); err != nil {
			return err
		}
	}
	for _, name := range []string{"spacing", "radius"} {
		mapping, err := requireMap(tokens[name], name)
		if err != nil {
			return err
		}
		if err := validateNumericMap(mapping, name, 0); err != nil {
			return err
		}
	}
	radius := tokens["radius"].(map[string]any)
	if radius["pill_px"] != 999 {
		return fmt.Errorf("radius.pill_px 必须为 999")
	}
	archetypes, ok := tokens["scene_archetypes"].([]any)
	canonical := []string{"proposition", "comparison", "process", "capability_deck"}
	if !ok || len(archetypes) != len(canonical) {
		return fmt.Errorf("scene_archetypes 必须包含四种标准类型")
	}
	for index, item := range archetypes {
		entry, err := requireMap(item, fmt.Sprintf("scene_archetypes[%d]", index))
		if err != nil {
			return err
		}
		if entry["id"] != canonical[index] {
			return fmt.Errorf("scene_archetypes 必须按标准顺序声明")
		}
		if _, ok := entry["name"].(string); !ok {
			return fmt.Errorf("scene_archetypes[%d].name 必须是字符串", index)
		}
		if _, ok := entry["example_png"].(string); !ok {
			return fmt.Errorf("scene_archetypes[%d].example_png 必须是字符串", index)
		}
	}
	motion, err := requireMap(tokens["motion"], "motion")
	if err != nil {
		return err
	}
	if err := requireKeys(motion, []string{"phases", "entrance_seconds", "exit_seconds", "transition_seconds", "entrance_ease", "exit_ease", "verbs"}, "motion"); err != nil {
		return err
	}
	phases, err := requireMap(motion["phases"], "motion.phases")
	if err != nil {
		return err
	}
	if err := validateNumericMap(phases, "motion.phases", 0); err != nil {
		return err
	}
	total := 0.0
	for _, value := range phases {
		total += number(value)
	}
	if total < .999999 || total > 1.000001 {
		return fmt.Errorf("motion.phases 之和必须为 1")
	}
	audio, err := requireMap(tokens["audio"], "audio")
	if err != nil {
		return err
	}
	if err := requireKeys(audio, []string{"sample_rate_hz", "channels", "final_integrated_lufs", "final_lufs_tolerance", "true_peak_max_dbtp", "bgm_ducking_db", "bgm_fade_seconds", "sfx_max_simultaneous"}, "audio"); err != nil {
		return err
	}
	if audio["sample_rate_hz"] != 48000 || audio["channels"] != 2 {
		return fmt.Errorf("audio 必须为 48000Hz 双声道")
	}
	subtitles, err := requireMap(tokens["subtitles"], "subtitles")
	if err != nil {
		return err
	}
	if err := requireKeys(subtitles, []string{"required", "font_size_px", "max_lines", "max_fullwidth_chars_per_line", "line_height", "background", "text_color", "padding_px", "radius_px", "break_rules"}, "subtitles"); err != nil {
		return err
	}
	cover, err := requireMap(tokens["cover"], "cover")
	if err != nil {
		return err
	}
	if err := requireKeys(cover, []string{"frame_zero_complete", "default_lines", "max_lines", "font_size_px", "max_width_px", "contrast_ratio_min", "stable_frames"}, "cover"); err != nil {
		return err
	}
	stableFrames, stable := cover["stable_frames"].(int)
	if cover["frame_zero_complete"] != true || !stable || stableFrames < 18 {
		return fmt.Errorf("cover 必须从第零帧完整显示并稳定至少 18 帧")
	}
	forbidden, ok := tokens["forbidden"].([]any)
	if !ok || len(forbidden) == 0 {
		return fmt.Errorf("forbidden 必须是非空字符串数组")
	}
	for _, item := range forbidden {
		if _, ok := item.(string); !ok {
			return fmt.Errorf("forbidden 必须是非空字符串数组")
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
	magick, err := exec.LookPath("magick")
	if err != nil {
		return fmt.Errorf("需要 ImageMagick 的 `magick` 命令")
	}
	outputDir := filepath.Join(projectRoot, "assets", "style-guide", "examples")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	names := []string{"proposition", "comparison", "process", "capability_deck"}
	pngPaths := make([]string, 0, len(names))
	for _, name := range names {
		sourcePath := "assets/style-guide/examples/" + name + ".svg"
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
		if commandOutput, err := exec.Command(magick, "-background", "none", svgPath, pngPath).CombinedOutput(); err != nil {
			return fmt.Errorf("ImageMagick 渲染 %s 失败: %w: %s", name, err, strings.TrimSpace(string(commandOutput)))
		}
		fmt.Fprintf(output, "generated %s\ngenerated %s\n", filepath.ToSlash(filepath.Join("assets", "style-guide", "examples", name+".svg")), filepath.ToSlash(filepath.Join("assets", "style-guide", "examples", name+".png")))
		pngPaths = append(pngPaths, pngPath)
	}
	contactPath := filepath.Join(outputDir, "contact-sheet.png")
	args := append([]string{"montage"}, pngPaths...)
	args = append(args, "-thumbnail", "405x540", "-tile", "4x1", "-geometry", "405x540+10+10", "-background", colors["structure_line"].(string), contactPath)
	if commandOutput, err := exec.Command(magick, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ImageMagick 生成 contact sheet 失败: %w: %s", err, strings.TrimSpace(string(commandOutput)))
	}
	fmt.Fprintf(output, "generated %s\n", filepath.ToSlash(filepath.Join("assets", "style-guide", "examples", "contact-sheet.png")))
	return nil
}
