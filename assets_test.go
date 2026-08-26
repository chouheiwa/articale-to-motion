package assets

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/cast"
	"github.com/chouheiwa/articale-to-motion/internal/hyperframes"
	"github.com/chouheiwa/articale-to-motion/internal/scene"
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

// TestCharacterRigSkillMentionsCastRequiredHeading 覆盖遗留缺口⑥：
// 「角色（强制）」是 internal/scene.castSection 产出、internal/scene/skills.go
// 的角色驱动门控消费、character-rig/SKILL.md:115 引用的三处共同契约锚点。
// 前两处已经收拢成 scene.CastRequiredHeading 这一个 Go 常量（改一处、两处
// 同步），但 SKILL.md 是纯文本，改错这里不会编译失败，只能靠这条断言把它
// 钉在同一个字面量上——三处但凡有一处漂移，"下方出现「角色（强制）」段"
// 这句话对读它的人（无论是渲染 agent 还是维护者）就变成假话。
func TestCharacterRigSkillMentionsCastRequiredHeading(t *testing.T) {
	body, err := fs.ReadFile(sharedTree(t), castSkillPath("SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), scene.CastRequiredHeading) {
		t.Errorf("character-rig/SKILL.md 应引用契约标题 %q，与 scene.CastRequiredHeading 保持一致", scene.CastRequiredHeading)
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
	code := stripJSComments(string(body))
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
	// 锚在赋值上而不是靠 stripJSComments 兜底：这两道防线各挡一半。剥注释挡的是
	// 「注释里顺口写到这个值」，锚 “= ” 挡的是「值出现在别处（选择器、错误文案）」。
	// data-no-mirror 是选择器不是赋值，按 [attr] 的完整写法锚定：紧邻的那条
	// 注释里只有裸的 data-no-mirror，带方括号的只可能是真选择器。
	code := stripJSComments(string(body))
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

// TestStripJSCommentsHandlesRegexLiterals 守住上面那些静态断言的地基。
//
// stripJSComments 只要在任何一处认错上下文，扫描就会整体错位，而错位的方向决定
// 危害：把代码当字符串吞掉的话，本文件里全部禁令与常量断言都会静默失效（PASS
// 但什么都没查）。cast.js 早先就吃过这个亏——一个 /["']/ 让文件后六成的注释都
// 没被剥掉。所以这里喂一段把四种上下文全凑齐的样本，逐字比对剥离结果。
func TestStripJSCommentsHandlesRegexLiterals(t *testing.T) {
	const src = `const cls = /["']/;      // 正则里的引号不是引号：这条注释必须被剥掉
const url = 'https://a.b//c'; // 字符串里的 // 不是注释开头
const raw = "他说 /* 别删我 */";  // 字符串里的块注释记号也不是注释
/* 跨行块注释
   第二行 */
const div = total / count;  // 这个斜杠是除号，不能当成正则开头
const tpl = ` + "`" + `第 ${idx / 2} 段 ${label.replace(/\s+/g, '')}` + "`" + `; // 模板串里两种都有
const esc = /a\/b/g.test(url); // 正则里转义过的斜杠不算结束
setTimeout(fn, 0); // 这行注释里的 setTimeout( 必须被剥掉，代码里的那个不许被剥掉
`
	const want = `const cls = /["']/;
const url = 'https://a.b//c';
const raw = "他说 /* 别删我 */";


const div = total / count;
const tpl = ` + "`" + `第 ${idx / 2} 段 ${label.replace(/\s+/g, '')}` + "`" + `;
const esc = /a\/b/g.test(url);
setTimeout(fn, 0);
`
	got := stripJSComments(src)
	// 行尾空白是「注释被换成换行」留下的，不影响任何 strings.Contains 断言，
	// 逐行右裁后再比，这样期望值写起来才是可读的。
	trimLines := func(s string) string {
		lines := strings.Split(s, "\n")
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], " \t")
		}
		return strings.Join(lines, "\n")
	}
	if trimLines(got) != trimLines(want) {
		t.Errorf("剥离结果不对：\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// 再单独钉两条最要命的：注释里的禁令词必须消失，代码里的必须留下。
	if strings.Contains(got, "这条注释必须被剥掉") || strings.Contains(got, "这行注释里的") {
		t.Error("正则/字符串之后的行注释没有被剥掉：假红回来了")
	}
	if !strings.Contains(got, "setTimeout(fn, 0)") {
		t.Error("代码里的 setTimeout( 被吃掉了：假绿——禁令断言从此形同虚设")
	}
}

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

// regexPrefixKeywords 是「后面跟的 / 一定是正则字面量开头，不是除号」的关键字。
//
// JS 的 / 有歧义：`a / b` 是除法，`return /x/` 是正则。判别方式是看前一个有意义
// 的 token 能不能作为一个值的结尾——能（标识符、数字、字符串、) ] }）就是除号，
// 不能（运算符、逗号、左括号，以及这里这些关键字）就是正则。
var regexPrefixKeywords = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true,
	"new": true, "delete": true, "void": true, "throw": true, "case": true,
	"do": true, "else": true, "yield": true, "await": true,
}

func isJSIdentRune(c rune) bool {
	return c == '_' || c == '$' || (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// regexCanFollow 判断刚扫到的 / 是正则开头还是除号，依据是已产出代码的末尾。
//
// 已知边界：`if (a) /re/.test(b)` 这种「) 之后直接跟正则」会被判成除号。JS 不看
// 语法上下文就无法区分它和 `f(a) / 2`，而真正的解析器不许引入。cast.js 里的正则
// 都跟在 = 或 ( 之后，判别不到这条边界；真写出这种写法，效果是把它当除号扫，
// 正则里的引号会被当成字符串开头——也就是退回本函数存在之前的老毛病，
// 而 TestStripJSCommentsHandlesRegexLiterals 会在样本里守住常见形态。
func regexCanFollow(out []rune) bool {
	i := len(out) - 1
	for i >= 0 && (out[i] == ' ' || out[i] == '\t' || out[i] == '\n' || out[i] == '\r') {
		i--
	}
	if i < 0 {
		return true
	}
	switch c := out[i]; {
	case c == ')' || c == ']' || c == '}':
		return false
	case c == '\'' || c == '"' || c == '`':
		return false
	case isJSIdentRune(c):
		j := i
		for j >= 0 && isJSIdentRune(out[j]) {
			j--
		}
		return regexPrefixKeywords[string(out[j+1:i+1])]
	default:
		return true
	}
}

// stripJSComments 去掉 // 行注释与 /* */ 块注释，只留下可执行代码。
//
// 逐字符扫描并跟踪全部四种「不是代码」的上下文：字符串、模板串（含 ${} 里嵌回
// 代码那一层）、正则字面量、注释。任何一种漏认都会让后面的扫描整体错位，而错位
// 的方向决定了危害：
//
//   - 把注释当代码（漏剥）只是假红——注释里写到 setTimeout 之类的禁令词会误报，
//     看一眼就知道；
//   - 把代码当注释或当字符串（多剥、或吞掉一大段）是假绿——上面那些禁令与常量
//     断言全部悄悄失效，测试照样 PASS。
//
// 这个函数此前只认引号，不认正则字面量：cast.js 里 `if (/["']/.test(selectors))`
// 的那个字符组被当成开引号，从那一行到文件末尾（约六成篇幅）的注释一句都没被剥
// 掉。危害当时是假红，但只要有人在它前面再写一个带引号的正则，假绿就回来了。
//
// 不引第三方 JS 解析器：这是测试里的一段静态检查，依赖一个 JS parser 会把
// go test 的可运行性绑到一堆与本仓库无关的东西上。
func stripJSComments(src string) string {
	runes := []rune(src)
	out := make([]rune, 0, len(runes))
	// 模板串栈：进入 `…` 压一层，进入其中的 ${…} 再压一层代码帧。
	// depths 记每个代码帧里未闭合的 { 数，用来分辨 ${} 的收尾 } 与普通块的 }。
	inTemplate := []bool{false}
	depths := []int{0}
	top := func() int { return len(inTemplate) - 1 }

	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if inTemplate[top()] {
			switch {
			case c == '\\' && i+1 < len(runes):
				out = append(out, c, runes[i+1])
				i++
			case c == '`':
				out = append(out, c)
				inTemplate = inTemplate[:top()]
				depths = depths[:len(depths)-1]
			case c == '$' && i+1 < len(runes) && runes[i+1] == '{':
				out = append(out, c, '{')
				i++
				inTemplate = append(inTemplate, false)
				depths = append(depths, 0)
			default:
				out = append(out, c)
			}
			continue
		}

		// 行注释：整行吃掉，只留换行，保住后续行号。
		if c == '/' && i+1 < len(runes) && runes[i+1] == '/' {
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			out = append(out, '\n')
			continue
		}
		// 块注释：整段吃掉，内部每个换行照写一个，同样是为了保住行号。
		if c == '/' && i+1 < len(runes) && runes[i+1] == '*' {
			i += 2
			for i < len(runes) && !(runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/') {
				if runes[i] == '\n' {
					out = append(out, '\n')
				}
				i++
			}
			i++ // 停在 '/' 上，交给外层 i++ 跨过去
			continue
		}
		// 正则字面量：连同字符组与标志一起原样抄下来，里面的引号不算引号。
		if c == '/' && regexCanFollow(out) {
			out = append(out, c)
			inClass := false
			for i++; i < len(runes); i++ {
				r := runes[i]
				out = append(out, r)
				if r == '\\' && i+1 < len(runes) {
					i++
					out = append(out, runes[i])
					continue
				}
				if r == '\n' { // 未闭合的正则不存在，按行止损
					break
				}
				if r == '[' {
					inClass = true
				} else if r == ']' {
					inClass = false
				} else if r == '/' && !inClass {
					break
				}
			}
			for i+1 < len(runes) && isJSIdentRune(runes[i+1]) { // 标志位 g/i/m/s/u/y
				i++
				out = append(out, runes[i])
			}
			continue
		}
		if c == '\'' || c == '"' {
			out = append(out, c)
			for i++; i < len(runes); i++ {
				r := runes[i]
				out = append(out, r)
				if r == '\\' && i+1 < len(runes) {
					i++
					out = append(out, runes[i])
					continue
				}
				if r == c || r == '\n' {
					break
				}
			}
			continue
		}
		if c == '`' {
			out = append(out, c)
			inTemplate = append(inTemplate, true)
			depths = append(depths, 0)
			continue
		}
		if c == '{' {
			depths[len(depths)-1]++
		} else if c == '}' {
			if depths[len(depths)-1] == 0 && top() > 0 {
				// ${…} 收尾：回到外层模板串。
				out = append(out, c)
				inTemplate = inTemplate[:top()]
				depths = depths[:len(depths)-1]
				continue
			}
			if depths[len(depths)-1] > 0 {
				depths[len(depths)-1]--
			}
		}
		out = append(out, c)
	}
	return string(out)
}

// TestPresetsPinSameHyperFramesVersionAsBinary 守住下发提示词与二进制固定
// 版本之间的一致性。
//
// PROMPT-PRODUCTION.md 第八阶段要求渲染 agent 建一个「渲染器锁定文件」，
// 把 HyperFrames 版本钉死。这个版本号原先是在模板里硬写的字面量，与
// hyperframes.PinnedVersion 之间没有任何绑定——改了常量忘了模板，am init
// 装的是新版技能、而下发给渲染 agent 的锁定指令仍指向旧版，两边悄悄错开。
//
// 断言的是「每份产物里出现的 hyperframes@ 版本全部等于 PinnedVersion」，
// 而不是「包含 PinnedVersion」：后者在模板同时留着新旧两个版本号时也会通过。
func TestPresetsPinSameHyperFramesVersionAsBinary(t *testing.T) {
	// 走 embed.FS 而不是磁盘路径：真正发到用户手上的是嵌进二进制的这一份，
	// 磁盘上的生成物与它一致由 go generate 后的 git diff 门禁另行保证。
	prompts, err := fs.Glob(Files, "assets/presets/*/PROMPT-PRODUCTION.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) == 0 {
		t.Fatal("嵌入树里没有任何 PROMPT-PRODUCTION.md，检查 //go:embed 模式")
	}
	want := "hyperframes@" + hyperframes.PinnedVersion
	pattern := regexp.MustCompile(`hyperframes@[0-9][0-9A-Za-z.\-]*`)
	for _, path := range prompts {
		body, err := fs.ReadFile(Files, path)
		if err != nil {
			t.Fatal(err)
		}
		found := pattern.FindAllString(string(body), -1)
		if len(found) == 0 {
			t.Errorf("%s 没有固定 HyperFrames 版本", path)
			continue
		}
		for _, got := range found {
			if got != want {
				t.Errorf("%s 固定的是 %s，与二进制的 %s 不一致", path, got, want)
			}
		}
	}
}
