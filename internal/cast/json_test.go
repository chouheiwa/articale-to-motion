package cast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// character.json 是驱动库 cast.js 在无头 Chrome 里读取角色包的唯一格式：
// 浏览器端 CSP 下引不进 YAML 解析库。它每次校验都从 character.yaml 重新生成，
// 键名必须和 yaml 的小驼峰一致，Go 字段名（尤其是本机绝对路径 Dir）不得泄漏。
func TestWriteJSONProducesLowerCamelKeysWithoutDir(t *testing.T) {
	dir := writePack(t, goodYAML)
	if err := os.WriteFile(filepath.Join(dir, "rig.svg"), []byte(goodSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	pack, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := pack.WriteJSON(dir); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "character.json"))
	if err != nil {
		t.Fatal(err)
	}

	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		t.Fatalf("character.json 不是合法 JSON：%v", err)
	}
	for _, want := range []string{"schema", "id", "name", "summary", "voice", "rig", "scale", "poses"} {
		if _, ok := generic[want]; !ok {
			t.Errorf("character.json 缺少键 %q，实际键：%v", want, generic)
		}
	}
	if _, ok := generic["Dir"]; ok {
		t.Error("character.json 不应包含本机绝对路径 Dir 字段")
	}
	if strings.Contains(string(body), "Dir") {
		t.Errorf("character.json 不应出现 Go 字段名 Dir：%s", body)
	}

	rig, ok := generic["rig"].(map[string]any)
	if !ok {
		t.Fatalf("rig 字段类型不对：%v", generic["rig"])
	}
	for _, want := range []string{"file", "viewBox", "baselineY", "joints"} {
		if _, ok := rig[want]; !ok {
			t.Errorf("character.json 的 rig 缺少键 %q", want)
		}
	}

	voice, ok := generic["voice"].(map[string]any)
	if !ok {
		t.Fatalf("voice 字段类型不对：%v", generic["voice"])
	}
	minimax, ok := voice["minimax"].(map[string]any)
	if !ok {
		t.Fatalf("voice.minimax 类型不对：%v", voice["minimax"])
	}
	if _, ok := minimax["voiceId"]; !ok {
		t.Error("character.json 的 voice.minimax 缺少 voiceId")
	}

	scale, ok := generic["scale"].(map[string]any)
	if !ok {
		t.Fatalf("scale 字段类型不对：%v", generic["scale"])
	}
	if _, ok := scale["heightRatio"]; !ok {
		t.Error("character.json 的 scale 缺少 heightRatio")
	}
}
