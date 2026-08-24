package assets

import (
	"io/fs"
	"strings"
	"testing"
)

// TestBuiltinSkillsIncludesCharacterRig 守住角色驱动库的下发。
//
// 镜头提示词的角色契约段（internal/scene 的 castSection）写死了“必须使用
// character-rig 技能的 cast.js 驱动”。技能没随二进制下发时，渲染 agent
// 找不到这份契约提到的东西，会自己发明一套动画写法——那套写法多半不挂在
// 那条 paused timeline 上，本地播放正常、成片是错的，而且退出码为 0。
func TestBuiltinSkillsIncludesCharacterRig(t *testing.T) {
	skills, err := BuiltinSkills()
	if err != nil {
		t.Fatal(err)
	}
	if !skills["character-rig"] {
		t.Errorf("内置技能缺 character-rig：%v", skills)
	}
}

// TestCharacterRigShipsDriverAndManifest 校验技能目录里真的有那两个文件。
//
// BuiltinSkills 只看目录名，空目录也算数；而提示词里点名的是 cast.js 这个
// 具体文件，缺了它同样是静默降级。
func TestCharacterRigShipsDriverAndManifest(t *testing.T) {
	shared, err := Shared()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "cast.js"} {
		path := ".agents/skills/character-rig/" + name
		if _, err := fs.Stat(shared, path); err != nil {
			t.Errorf("character-rig 技能缺少 %s：%v", name, err)
		}
	}
}

// TestCastDriverReadsJSONNotYAML 锁住驱动库的输入格式。
//
// 渲染机是干净的无头 Chrome，CSP 下引不进 YAML 解析库，手写 YAML 子集
// 解析器纯属埋雷。character.yaml 是人工真相源，character.json 是每次
// am cast new/add/validate 都重新生成的产物——浏览器端只准读后者。
func TestCastDriverReadsJSONNotYAML(t *testing.T) {
	shared, err := Shared()
	if err != nil {
		t.Fatal(err)
	}
	body, err := fs.ReadFile(shared, ".agents/skills/character-rig/cast.js")
	if err != nil {
		t.Fatal(err)
	}
	// 只看代码，不看注释：注释里正需要写清楚为什么不读 YAML、为什么不用 rAF。
	code := stripJSLineComments(string(body))
	if !strings.Contains(code, "/character.json") {
		t.Error("cast.js 没有读取 character.json")
	}
	// 拉取的必须是 .json；错误信息里提到 character.yaml 是对的（人要去那儿改），
	// 所以只禁「取 .yaml 文件」和「引 YAML 解析器」这两件事。
	if strings.Contains(code, "/character.yaml") {
		t.Error("cast.js 不得在浏览器里拉取 character.yaml：CSP 下引不进 YAML 库")
	}
	for _, forbidden := range []string{"jsyaml", "yaml.load", "parseYaml", "YAML.parse"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("cast.js 不得出现 YAML 解析器 %s：真相源是 character.json", forbidden)
		}
	}
	// 自建 timeline / rAF / 定时器驱动的动画不受那条 paused timeline 管，
	// 逐帧 seek 出来的成片是错的且不报错。
	for _, forbidden := range []string{"requestAnimationFrame", "setTimeout(", "setInterval(", "gsap.timeline("} {
		if strings.Contains(code, forbidden) {
			t.Errorf("cast.js 不得出现 %s：动画必须挂在调用方传入的 paused timeline 上", forbidden)
		}
	}
}

// stripJSLineComments 去掉 // 行注释，只留下可执行代码。
// cast.js 里没有块注释，也没有含 "//" 的字符串字面量，够用且不会误伤。
func stripJSLineComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if at := strings.Index(line, "//"); at >= 0 {
			lines[i] = line[:at]
		}
	}
	return strings.Join(lines, "\n")
}
