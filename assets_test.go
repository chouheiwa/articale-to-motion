package assets

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/cast"
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
	for _, name := range []string{"SKILL.md", "cast.js"} {
		if _, err := fs.Stat(sharedTree(t), castSkillPath(name)); err != nil {
			t.Errorf("character-rig 技能缺少 %s：%v", name, err)
		}
	}
}

// TestCastDriverReadsJSONNotYAML 锁住驱动库的输入格式与动画载体。
//
// 渲染机是干净的无头 Chrome，CSP 下引不进 YAML 解析库，手写 YAML 子集
// 解析器纯属埋雷。character.yaml 是人工真相源，character.json 是每次
// am cast new/add/validate 都重新生成的产物——浏览器端只准读后者。
//
// 后半段禁的是「不受那条 paused timeline 管的动画」：rAF、定时器、自建
// timeline、SMIL <animate>、CSS @keyframes、WAAPI element.animate()。
// 这类动画按墙钟推进，逐帧 seek 出来的成片是错的，而且退出码为 0。
func TestCastDriverReadsJSONNotYAML(t *testing.T) {
	body, err := fs.ReadFile(sharedTree(t), castSkillPath("cast.js"))
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
	for _, forbidden := range []string{
		"requestAnimationFrame", "setTimeout(", "setInterval(", "gsap.timeline(",
		".animate(", "<animate", "@keyframes", "new Animation",
	} {
		if strings.Contains(code, forbidden) {
			t.Errorf("cast.js 不得出现 %s：动画必须挂在调用方传入的 paused timeline 上", forbidden)
		}
	}
}

// TestCastDriverConstantsMatchGo 锁住 Go 侧 internal/cast 与 cast.js 里
// 那四个一式两份的常量。
//
// 前三个（schema、默认视图、关节前缀）漂了会在装载阶段响亮抛错，最后一个
// data-no-mirror 漂了却是静默的：Go 侧 ValidateRig 校验的是这个属性名存在，
// cast.js 按 [data-no-mirror] 选择器做二次翻转，任一处改名都不会有人报错，
// 只是 facing: left 的镜头里举牌、字幕板直接出反字，退出码照样为 0。
func TestCastDriverConstantsMatchGo(t *testing.T) {
	body, err := fs.ReadFile(sharedTree(t), castSkillPath("cast.js"))
	if err != nil {
		t.Fatal(err)
	}
	// 只看代码不看注释：注释里提到常量名不算“真的在用同一个值”。
	//
	// 三个字符串常量连 “= ” 一起锚定，锚的是赋值那一行本身，不是这个值在文件里
	// 出现过。cast.js 的注释里也会顺口写到这些值（例如 mount 的参数说明里那句
	// “初始视图名，默认 'front'”），光找 "'front'" 会被注释撑住：把
	// DEFAULT_VIEW 改成别的值测试照样 PASS，这一条就成了假保证。
	// 不指望 stripJSLineComments 兜住——它不认识正则字面量，cast.js 里
	// /["']/ 之后的注释根本没被剥掉，那是另一个范围的既有缺陷。
	// data-no-mirror 是选择器不是赋值，按 [attr] 的完整写法锚定：紧邻的那条
	// 注释里只有裸的 data-no-mirror，带方括号的只可能是真选择器。
	code := stripJSLineComments(string(body))
	for _, want := range []string{
		"= '" + cast.SchemaVersion + "'",
		"= '" + cast.DefaultView + "'",
		"= '" + cast.JointPrefix + "'",
		"[" + cast.NoMirrorAttr + "]",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("cast.js 里找不到 %s：它与 Go 侧 internal/cast 的常量必须一字不差", want)
		}
	}
}

// TestCastDriverPureFunctions 用 node 对 cast.js 导出的纯函数直接求值。
//
// 这个项目没有前端测试基建，JS 侧唯一的防线是静态审查。把「转身在哪一帧换
// 视图」这类定义性行为锁进可执行断言，比在 SKILL.md 里写一句「必须对齐」
// 有用得多：换视图的时刻一旦和压扁的零点错开，角色就会在还有可见宽度时
// 闪一下换成另一张图，而顺序播放的预览完全看不出来。
//
// 没装 node 的机器上跳过——CI 有 node，本地缺了不该把整个测试套件卡住。
func TestCastDriverPureFunctions(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("本机没有 node，跳过 cast.js 的纯函数断言")
	}
	body, err := fs.ReadFile(sharedTree(t), castSkillPath("cast.js"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	driver := filepath.Join(dir, "cast.js")
	if err := os.WriteFile(driver, body, 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "assert.mjs")
	if err := os.WriteFile(script, []byte(castPureAssertions), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, script, driver).CombinedOutput()
	if err != nil {
		t.Fatalf("cast.js 纯函数断言未通过：%v\n%s", err, out)
	}
	t.Logf("cast.js 纯函数断言：%s", strings.TrimSpace(string(out)))
}

// castPureAssertions 断言 cast.pure 里那几个取值函数的定义性行为。
//
// cast.js 是浏览器端的经典脚本，不是 ES 模块：用 new Function 取出它的
// 顶层 const，比给它加一套模块导出（那会改变它在 <script src> 下的行为）
// 更保险。纯函数不碰 DOM，所以 node 里不需要任何打桩。
const castPureAssertions = `
import fs from 'node:fs';
const src = fs.readFileSync(process.argv[2], 'utf8');
const cast = new Function(src + '; return cast;')();
const p = cast.pure;
const fail = (msg) => { console.error('断言失败：' + msg); process.exit(1); };
const near = (a, b) => Math.abs(a - b) < 1e-9;

// 1) 换视图的时刻必须正好是压扁的零点。
//    seq 有 N 个视图就压 N-1 次，每次切换都落在角色宽度为 0 的那一瞬。
for (const count of [2, 3, 4]) {
  const segments = count - 1;
  let switches = 0;
  let prev = p.viewIndexAtPhase(0, count);
  for (let i = 1; i <= 200000; i++) {
    const phase = (i / 200000) * segments;
    const index = p.viewIndexAtPhase(phase, count);
    if (index === prev) continue;
    switches++;
    prev = index;
    const pinch = p.pinchScaleAtPhase(phase);
    if (pinch > 1e-4) fail(count + ' 个视图：在 phase=' + phase + ' 换视图时压扁量是 ' + pinch + '，应为 0');
  }
  if (switches !== segments) fail(count + ' 个视图应当切换 ' + segments + ' 次，实际 ' + switches);
  if (!near(p.pinchScaleAtPhase(0), 1)) fail('转身起点必须是满宽');
  if (!near(p.pinchScaleAtPhase(segments), 1)) fail('转身终点必须是满宽');
}

// 2) 镜像翻转连续走 +1 -> 0 -> -1，中点恰好过零。
if (!near(p.mirrorScaleAtProgress(0), 1)) fail('镜像起点应为 +1');
if (!near(p.mirrorScaleAtProgress(0.5), 0)) fail('镜像中点应过零');
if (!near(p.mirrorScaleAtProgress(1), -1)) fail('镜像终点应为 -1');

// 3) 呼吸包络两端归零：seek 出区间时不留残留角度。
if (p.breathEnvelope(0) !== 0 || p.breathEnvelope(1) !== 0) fail('呼吸包络两端必须为 0');
if (p.breathEnvelope(0.5) !== 1) fail('呼吸包络中段必须为 1');

// 4) 几何：脚底压在 ground 上，高度等于 heightRatio × 舞台高度。
const layout = p.actorLayout({
  stageW: 1080, stageH: 1440, x: 0.34, ground: 0.78, heightRatio: 0.27,
  viewBox: [0, 0, 400, 520], baselineY: 512,
});
if (!near(layout.anchorLeft, 1080 * 0.34)) fail('锚点横坐标错');
if (!near(layout.anchorTop, 1440 * 0.78)) fail('锚点纵坐标错');
if (!near(layout.height, 1440 * 0.27)) fail('绘制高度错');
if (!near(layout.viewTop + 512 * layout.scale, 0)) fail('脚底没有压在 ground 上');
if (!near(layout.viewLeft * -2, layout.width)) fail('角色左右中线没有对齐 x');

// 5) 舞台状态是时间的纯函数：区间前取初始态、区间后取终态，与求值顺序无关。
const initial = { view: 'front', sign: 1 };
const schedule = [
  { start: 2, end: 2.2, seq: ['front', 'three-quarter', 'side'], sign: 1, mirror: false },
  { start: 5, end: 5.2, seq: ['side', 'three-quarter', 'front'], sign: 1, mirror: false },
];
const at = (t) => p.stageStateAt(schedule, initial, t);
if (at(0).view !== 'front' || at(1.9).view !== 'front') fail('转身之前应显示 front');
if (at(3).view !== 'side' || at(4.9).view !== 'side') fail('两次转身之间应显示 side');
if (at(6).view !== 'front') fail('第二次转身之后应显示 front');
if (at(2.1).view !== 'three-quarter') fail('转身中点应显示 via 视图');
if (!near(at(2.05).sign, 0)) fail('第一次换视图那一帧宽度应为 0');
for (const t of [0, 1.9, 2.05, 2.1, 3, 4.9, 5.1, 6]) {
  const a = JSON.stringify(at(t));
  for (let k = 0; k < 5; k++) { at(Math.random() * 8); }
  if (JSON.stringify(at(t)) !== a) fail('t=' + t + ' 的取值受了求值顺序影响');
}
console.log('压扁零点/换视图对齐、镜像曲线、呼吸包络、几何、乱序求值一致性 —— 全部通过');
`

func sharedTree(t *testing.T) fs.FS {
	t.Helper()
	shared, err := Shared()
	if err != nil {
		t.Fatal(err)
	}
	return shared
}

func castSkillPath(name string) string {
	return ".agents/skills/character-rig/" + name
}

// stripJSLineComments 去掉 // 行注释，只留下可执行代码。
//
// 逐字符扫描并跟踪引号状态：直接按 strings.Index 找 "//" 会把字符串字面量里
// 的 https:// 之类当成注释开头，把整行后半截连同真正的代码一起吃掉——那会让
// 上面几条禁令悄悄失效（被吃掉的代码再违规也检查不到）。cast.js 里没有块注释，
// 所以只处理行注释。
func stripJSLineComments(src string) string {
	var out strings.Builder
	var quote rune
	escaped := false
	runes := []rune(src)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if quote != 0 {
			out.WriteRune(c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == quote:
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			quote = c
			out.WriteRune(c)
			continue
		}
		if c == '/' && i+1 < len(runes) && runes[i+1] == '/' {
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			out.WriteRune('\n')
			continue
		}
		out.WriteRune(c)
	}
	return out.String()
}
