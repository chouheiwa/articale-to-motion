// character-rig 驱动库：把角色包（character pack）挂进 HyperFrames 镜头。
//
// ============================ 四条硬约束 ============================
//
// 1) 本库自己不建 timeline、不碰 requestAnimationFrame、不用定时器。
//    所有 tween 都挂在调用方传入的那条 paused timeline 上。HyperFrames 是
//    单条 paused timeline + 逐帧 seek 出帧：任何不受那条 timeline 管的动画，
//    本地播放看着正常、成片是错的，而且退出码为 0、不报错。
//
// 2) 视图、朝向、呼吸都是「时间 -> 画面」的纯函数：不写共享可变状态，而是在
//    装配期登记进 schedule，每帧由 renderFrame(时间) 从 schedule 现算。渲染机
//    按任意帧 seek，不保证顺序、不保证只走一遍——见下面「舞台状态」一段。
//    姿势是唯一的例外：关节角由时间线上的 GSAP tween 拥有（数值插值本来就该
//    它管），所以出帧只能靠推时间线，renderFrame 单独调用不算「画出第 t 帧」。
//
// 3) mount() 把所有已声明视图的 SVG 一次性挂进 DOM 并预置隐藏，转身只切换
//    可见性。不能在转身那一刻现取现挂：逐帧 seek 下异步加载时机不可控，会
//    抓到还没挂上的空帧。
//
// 4) 角色包从 character.json 读，不在浏览器里解析 YAML。渲染机是干净的无头
//    Chrome，CSP 下引不进 YAML 库，手写 YAML 子集解析器纯属埋雷。
//    character.yaml 仍是人工编辑的唯一真相源，character.json 由
//    am cast new/add/validate 每次重新生成。
//
// ====================================================================
const cast = (() => {
  'use strict';

  // 与 Go 侧 internal/cast 的常量一一对应，改一边必须改另一边。
  const SCHEMA = 'cast/v1';
  const DEFAULT_VIEW = 'front'; // rig 字段代表的视图，不出现在 views 里
  const JOINT_PREFIX = 'j-'; // rig.svg 里可动件分组的 id 前缀

  const DEFAULT_TURN_MS = 320; // 只有默认视图、退化成镜像翻转时的时长
  const BREATH_PERIOD = 1.05; // 秒；呼吸基础周期
  const BREATH_TAIL_PERIOD_FACTOR = 1.37; // 尾巴比头慢一点，避免整只狗同频抖
  const BREATH_RAMP = 0.12; // 进出场淡入淡出占比，保证区间两端幅度归零
  const BREATH_DEGREES = { head: 1.8, tail: 6.0, fallback: 1.0 };
  const BREATH_BOB_RATIO = 0.006; // 身体上下起伏，占角色绘制高度的比例
  // 呼吸挑关节靠名字：角色包若把头关节叫 noggin 一类，会走 fallback 兜底
  // （幅度更小的第一个关节）。SKILL.md 里写明了对关节命名的期望。
  const HEAD_JOINTS = /head|neck/i;
  const TAIL_JOINTS = /tail/i;

  const clamp = (value, lo, hi) => Math.min(hi, Math.max(lo, value));
  const round3 = (value) => Math.round(value * 1000) / 1000;

  // ======================= 纯函数区 =======================
  //
  // 这几个函数刻意提到模块作用域并通过 cast.pure 导出：它们是本库全部时间
  // 相关取值的定义，而这个项目没有前端测试基建，静态审查与 node -e 断言是
  // 唯一防线。放在 mount() 闭包里就没人能从外面对它们求值。

  // pinchScaleAtPhase：转身的压扁量。phase 走 [0, segments]，零点落在半整数处。
  const pinchScaleAtPhase = (phase) => Math.abs(Math.cos(Math.PI * phase));

  // viewIndexAtPhase：转身走到 phase 时该显示第几个视图。
  //
  // round 让切换点正好落在 pinchScaleAtPhase 的零点上。两者必须共用同一个
  // phase：错开一点点就会在角色还有可见宽度时换图，肉眼是「闪了一下换了张
  // 图」而不是转身。
  const viewIndexAtPhase = (phase, viewCount) => clamp(Math.round(phase), 0, viewCount - 1);

  // mirrorScaleAtProgress：镜像翻转的 scaleX 系数，连续从 +1 走到 -1。
  const mirrorScaleAtProgress = (local) => Math.cos(Math.PI * local);

  // breathEnvelope：呼吸的进出场包络，区间两端为 0，中间为 1。
  // 两端归零是为了 seek 出区间时呼吸干净地消失，不留残留角度。
  const breathEnvelope = (local, ramp = BREATH_RAMP) => clamp(Math.min(local, 1 - local) / ramp, 0, 1);

  // actorLayout：角色在舞台上的像素几何。
  // 高度取 heightRatio × 舞台高度，左右中线对齐 x，脚底 baselineY 对齐 ground。
  function actorLayout({ stageW, stageH, x, ground, heightRatio, viewBox, baselineY }) {
    const [, vbY, vbW, vbH] = viewBox;
    const drawnHeight = stageH * heightRatio;
    const scale = drawnHeight / vbH;
    return {
      scale,
      anchorLeft: stageW * x,
      anchorTop: stageH * ground,
      width: vbW * scale,
      height: drawnHeight,
      viewLeft: -(vbW * scale) / 2,
      viewTop: -(baselineY - vbY) * scale,
    };
  }

  // stageStateAt：某一时刻角色的视图与 scaleX，是 schedule 的纯函数。
  //
  // schedule 的每一条形如 { start, end, seq, sign, mirror }，按时间首尾相接：
  // 每条的起始态等于上一条的结束态（装配期保证，见 assertForward）。所以只要
  // 取「最后一条已经开始的」就够，不需要从头累积。
  function stageStateAt(schedule, initial, time) {
    let entry = null;
    for (const candidate of schedule) {
      if (candidate.start <= time && (!entry || candidate.start >= entry.start)) entry = candidate;
    }
    if (!entry) return initial;
    const endSign = entry.mirror ? -entry.sign : entry.sign;
    if (time >= entry.end) return { view: entry.seq[entry.seq.length - 1], sign: endSign };
    const local = (time - entry.start) / (entry.end - entry.start);
    if (entry.mirror) {
      return { view: entry.seq[0], sign: entry.sign * mirrorScaleAtProgress(local) };
    }
    // 一次视图切换配一次压扁，N 个视图的路径压 N-1 次。这不是可调的节奏参数，
    // 而是这套做法的定义：via 声明的中间视图必须在两次压扁之间真的被看见，
    // 否则声明它就没有意义。想让每次压扁更从容，加长 durationMs。
    const segments = Math.max(1, entry.seq.length - 1);
    const phase = local * segments;
    return {
      view: entry.seq[viewIndexAtPhase(phase, entry.seq.length)],
      sign: entry.sign * pinchScaleAtPhase(phase),
    };
  }

  // breathStateAt：某一时刻的呼吸叠加量（每个关节的角度增量 + 身体起伏像素）。
  // 区间之外一律返回 0。
  function breathStateAt(schedule, targets, time, bobAmplitude) {
    const value = { bobY: 0, joints: {} };
    let entry = null;
    for (const candidate of schedule) {
      if (candidate.start <= time && time < candidate.end && (!entry || candidate.start >= entry.start)) {
        entry = candidate;
      }
    }
    if (!entry) return value;
    const local = (time - entry.start) / (entry.end - entry.start);
    const envelope = breathEnvelope(local);
    const elapsed = time - entry.start;
    for (const target of targets) {
      value.joints[target.name] =
        Math.sin(2 * Math.PI * (elapsed / target.period + target.phase)) *
        target.degrees * entry.intensity * envelope;
    }
    value.bobY = Math.sin(2 * Math.PI * (elapsed / BREATH_PERIOD + 0.5)) * bobAmplitude * entry.intensity * envelope;
    return value;
  }

  // breathTargetsFor：呼吸挑哪些关节、各自多大幅度。
  //
  // 刻意不做口型音素同步——这个画风是面无表情的简笔画，加嘴型会毁掉它；
  // 「在说话」由头部起伏、呼吸和尾巴表达，speak 与 listen 的差别只是幅度。
  function breathTargetsFor(jointNames) {
    const names = [...jointNames].sort();
    const targets = [];
    names.filter((n) => HEAD_JOINTS.test(n)).forEach((name, i) => {
      targets.push({ name, degrees: BREATH_DEGREES.head, period: BREATH_PERIOD, phase: i * 0.13 });
    });
    names.filter((n) => TAIL_JOINTS.test(n)).forEach((name, i) => {
      targets.push({
        name,
        degrees: BREATH_DEGREES.tail,
        period: BREATH_PERIOD * BREATH_TAIL_PERIOD_FACTOR,
        phase: 0.25 + i * 0.13,
      });
    });
    if (targets.length === 0 && names.length > 0) {
      targets.push({ name: names[0], degrees: BREATH_DEGREES.fallback, period: BREATH_PERIOD, phase: 0 });
    }
    return targets;
  }

  // ======================= 渲染机的 seek 也要能出帧 =======================
  //
  // 实测 GSAP 3.15：tl.seek(t) 的 suppressEvents 默认是 true，onUpdate 完全
  // 不触发；tl.time(t) / tl.progress(p) 才触发。而 HyperFrames 的采样脚本
  // （hyperframes-animation/scripts/animation-map-sampling.mjs）走的正是
  // window.__hf.seek(time) / timeline.seek(time) 的默认参数这条路。
  //
  // 只把渲染挂在 onUpdate 上，遇到这条路径角色会从头到尾定格在 mount 姿势，
  // 退出码 0、不报错——本项目最贵的那种失败。
  //
  // 解法照抄上游官方示例（hyperframes-animation/examples/
  // messaging-multi-phrase.html）：一条覆盖全片的 tween 负责常规出帧，另外把
  // window.__hf / window.__player 的 seek 和 timeline 的 seek/pause 各包一层，
  // 让它们也把时间转发给 renderFrame。这些入口都是在宿主已经把时间推到 t
  // 之后才被调用的，所以姿势已经由时间线回算好，补渲染拿到的是完整的一帧。
  // actors 是模块级注册表：每项记着「怎么出帧」和「挂在哪条 timeline 上」。
  const actors = [];

  // renderActors 按宿主区分作用域。
  //
  // owner 为 null 表示宿主级入口（window.__hf / window.__player 的 seek）：
  // 它推的是整页的时间，所有角色都该跟着走。
  // owner 是某条 timeline 时，只能渲染绑在那条 timeline 上的角色——
  // HyperFrames 的 window.__timelines 本身就是 map、支持子 composition，
  // 同页出现第二条时间线时，推 A 把 B 也一起渲染就是串台：B 的时间根本没动，
  // 却按 A 的时间画了一帧。不报错，只是另一个角色的帧全是错的。
  //
  // 出帧的时间**取角色自己那条 timeline 的 time()，不用 seek 的入参**。
  // 入参是宿主的单位与坐标系，本库无从假设：宿主若按毫秒 seek
  // （animejs 适配器就是毫秒口径），入参会比时间线的真实时间大三个数量级。
  // 而这个包装跑在宿主自己的 seek 之后，是这一帧的最后一个写入者——用错了
  // 单位就不是「偶尔错一帧」，而是每一帧都被覆盖成错的，角色全程冻在终态、
  // 不报错。timeline 的 time() 是本库唯一认得的坐标系，schedule 里的时间也
  // 都在这个坐标系里。
  function renderActors(time, owner) {
    for (const actor of actors) {
      if (owner && actor.timeline() !== owner) continue;
      const timeline = actor.timeline();
      actor.render(timeline ? timeline.time() : time);
    }
  }

  // scoped 为 true 表示 owner 是一条 timeline，只渲染绑在它上面的角色。
  function wrapSeekOwner(owner, scoped) {
    if (!owner || owner.__castSeekWrapped || typeof owner.seek !== 'function') return;
    const inner = owner.seek.bind(owner);
    owner.seek = function (time, ...rest) {
      const result = inner(time, ...rest);
      if (typeof time === 'number') renderActors(time, scoped ? owner : null);
      return result;
    };
    owner.__castSeekWrapped = true;
  }

  // installWindowSeekHook：宿主对象可能在 cast.js 之后才被赋值，所以用
  // defineProperty 等它出现。装不上（属性不可配置）就静默放过——覆盖全片的
  // 那条 tween 仍然工作，这里只是额外一层保险。
  function installWindowSeekHook(key) {
    if (typeof window === 'undefined' || window[`__castHooked_${key}`]) return;
    try {
      window[`__castHooked_${key}`] = true;
      if (window[key]) {
        wrapSeekOwner(window[key], false);
        return;
      }
      let pending;
      Object.defineProperty(window, key, {
        configurable: true,
        get: () => pending,
        set: (value) => {
          pending = value;
          wrapSeekOwner(value, false);
        },
      });
    } catch (_) {
      /* 宿主不让改就算了，覆盖全片的 tween 才是主路径 */
    }
  }

  // ======================= I/O 与 SVG 预处理 =======================

  function requireTimeline(tl, method) {
    if (!tl || typeof tl.to !== 'function') {
      throw new Error(
        `cast.${method}() 的第一个参数必须是镜头那条 paused GSAP timeline：` +
          '角色动画必须挂在它上面，否则逐帧 seek 出的成片是错的且不报错'
      );
    }
    // time() 是硬要求，不做兜底：舞台状态按「timeline 当前时间」现算，拿不到
    // 准确时间就只能从每条 tween 自己的进度反推，而停在进度 0 的那条反推出的
    // 是它自己的起点、不是 timeline 真正所在的时刻，于是又退回成「取值依赖
    // 渲染顺序」。GSAP 的 timeline 一定有 time()。
    if (typeof tl.time !== 'function') {
      throw new Error(`cast.${method}() 收到的 timeline 没有 time() 方法：本库按 timeline 当前时间现算每一帧`);
    }
    return tl;
  }

  async function fetchText(url) {
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`cast 取 ${url} 失败：HTTP ${response.status}`);
    }
    return response.text();
  }

  function parseSVG(text, label) {
    const doc = new DOMParser().parseFromString(text, 'image/svg+xml');
    const failure = doc.querySelector('parsererror');
    if (failure) {
      throw new Error(`${label} 不是合法 SVG：${failure.textContent.trim()}`);
    }
    const root = doc.documentElement;
    if (!root || root.nodeName.toLowerCase() !== 'svg') {
      throw new Error(`${label} 的根元素不是 <svg>`);
    }
    return document.importNode(root, true);
  }

  // rewriteRefs 把一段文本里对旧 id 的引用改写成新 id。
  // 覆盖 url(#x)（含引号变体）与整串就是 "#x" 的 href。
  function rewriteRefs(text, renamed) {
    let next = text;
    for (const [from, to] of renamed) {
      for (const quote of ['', '"', "'"]) {
        next = next.split(`url(${quote}#${from}${quote})`).join(`url(${quote}#${to}${quote})`);
      }
      if (next === `#${from}`) next = `#${to}`;
    }
    return next;
  }

  // namespaceIds 给一个视图里的所有 id 加前缀，并同步改写内部引用。
  //
  // 三个视图的 rig 往往是同一张图改出来的，渐变、遮罩、滤镜的 id 大概率重名。
  // 三份 SVG 同时挂在一个文档里时，url(#grad) 一律解析到文档里第一个匹配的
  // 元素——侧面视图会静默用上正面视图的渐变。不报错，只是颜色不对。
  function namespaceIds(svg, prefix) {
    const nodes = [svg, ...svg.querySelectorAll('*')];
    const renamed = new Map();
    for (const node of nodes) {
      const id = node.getAttribute && node.getAttribute('id');
      if (id) {
        renamed.set(id, prefix + id);
        node.setAttribute('id', prefix + id);
      }
    }
    if (renamed.size === 0) return;
    for (const node of nodes) {
      if (node.attributes) {
        for (const attr of Array.from(node.attributes)) {
          if (!attr.value.includes('#')) continue;
          const next = rewriteRefs(attr.value, renamed);
          if (next !== attr.value) node.setAttribute(attr.name, next);
        }
      }
      // 内嵌 <style> 里的 url(#…) 同样要改写，否则它照样指到别的视图去。
      if (node.nodeName && node.nodeName.toLowerCase() === 'style' && node.textContent) {
        const next = rewriteRefs(node.textContent, renamed);
        if (next !== node.textContent) node.textContent = next;
      }
    }
  }

  // scopeStyleRules 把一份内嵌 <style> 里的每条规则限定在这个视图内。
  //
  // inline SVG 的 <style> 在 HTML 文档里是全局作用域。三份同源改出来的 rig
  // 共用 .c 或 circle 这类选择器几乎是必然，最后挂上的那份会盖住全部三个
  // 视图——与 id 撞车是同一种静默失败：不报错，只是颜色和描边不对。
  //
  // 只给「选择器 {」这种普通规则加前缀。@media 一类 at-rule 的头部不动
  // （正则里排除了 @），它内部的规则仍会被逐条加上前缀。
  //
  // 这是按「选择器 {」切分的正则改写，不是 CSS 解析器，有两类输入会被它悄悄
  // 改坏——都不是臆想出来的极端写法，而是随手就能写出的东西：
  //
  //   [title="a,b"]{…}     选择器按逗号拆分时，把引号里的逗号也当成了分隔符，
  //                        产出 [title="a, [scope] b"]，规则从此不再命中；
  //   /* { */ .a{fill:red} 注释里的花括号被当成规则边界，整段 CSS 报废。
  //
  // 与其改坏了不吭声，不如在这里拦住：rig 的样式本就该极简，改用 class、
  // 删掉注释都是几秒钟的事，而静默失效的样式要到看成片时才发现。
  function scopeStyleRules(svg, scopeSelector, label) {
    for (const style of svg.querySelectorAll('style')) {
      const css = style.textContent;
      if (!css || !css.includes('{')) continue;
      if (css.includes('/*')) {
        throw new Error(
          `${label} 的 <style> 里有 CSS 注释：视图作用域改写按「选择器 {」切分，` +
            '注释里的花括号会把规则切错、整段样式失效。请删掉 rig 样式里的注释。'
        );
      }
      style.textContent = css.replace(/(^|\}|\{)([^{}@]+)\{/g, (match, lead, selectors) => {
        if (/["']/.test(selectors)) {
          throw new Error(
            `${label} 的 <style> 选择器 ${selectors.trim()} 里有引号：视图作用域改写会按逗号拆分选择器，` +
              '引号里的内容会被一起改掉。请改用 class 选择器。'
          );
        }
        const scoped = selectors
          .split(',')
          .map((one) => one.trim())
          .filter(Boolean)
          .map((one) => `${scopeSelector} ${one}`)
          .join(', ');
        return `${lead}${scoped}{`;
      });
    }
  }

  // ======================= mount =======================
  //
  // opts:
  //   pack        角色包目录（相对镜头 HTML），例如 'cast/heiwa'
  //   x           水平位置，画面宽度的归一化比例（0–1），锚在角色左右中线
  //   ground      地平线，画面高度的归一化比例（0–1），脚底 baselineY 对齐它
  //   facing      'right'（默认）| 'left'，左右镜像，与 view 正交
  //   view        初始视图名，默认 'front'
  //   pose        初始姿势名，默认 'idle'
  //   heightRatio 覆盖角色高度占比，默认取包内 scale.heightRatio 的中值
  //   duration    镜头总时长（秒）。给了它，驱动 tween 从第 0 秒覆盖到片尾，
  //               每一帧都会重算角色；不给则只覆盖到最后一个角色事件结束。
  async function mount(selector, opts = {}) {
    const stage = typeof selector === 'string' ? document.querySelector(selector) : selector;
    if (!stage) throw new Error(`cast.mount 找不到舞台 ${selector}`);

    const packDir = String(opts.pack || '').replace(/\/+$/, '');
    if (!packDir) throw new Error('cast.mount 缺少 pack：角色包目录（相对镜头 HTML）');

    // 参数一律校验后再用，不做静默 clamp：位置写错了要立刻响，而不是把角色
    // 悄悄贴到画面边缘——后者要等到看成片才发现。
    const x = opts.x == null ? 0.5 : opts.x;
    const ground = opts.ground == null ? 0.8 : opts.ground;
    for (const [name, value] of [['x', x], ['ground', ground]]) {
      if (typeof value !== 'number' || !isFinite(value) || value < 0 || value > 1) {
        throw new Error(`cast.mount 的 ${name} 必须是 0–1 之间的归一化比例，收到 ${value}`);
      }
    }
    const facing = opts.facing == null ? 'right' : opts.facing;
    if (facing !== 'left' && facing !== 'right') {
      throw new Error(`cast.mount 的 facing 只能是 'left' 或 'right'，收到 ${JSON.stringify(opts.facing)}`);
    }

    // 硬约束 4：读 JSON，不读 YAML。
    const pack = JSON.parse(await fetchText(`${packDir}/character.json`));
    if (pack.schema !== SCHEMA) {
      throw new Error(`角色包 ${packDir} 的 schema 必须是 ${SCHEMA}，收到 ${pack.schema}`);
    }

    const rigOf = (name) => (name === DEFAULT_VIEW ? pack.rig : (pack.views || {})[name]);
    const viewNames = [DEFAULT_VIEW, ...Object.keys(pack.views || {}).filter((n) => n !== DEFAULT_VIEW)];

    const initialView = opts.view || DEFAULT_VIEW;
    if (!rigOf(initialView)) {
      throw new Error(`角色 ${pack.id} 没有视图 ${initialView}，已声明的是 ${viewNames.join('、')}`);
    }

    // 硬约束 3：所有视图一次性取回、一次性挂上。这里是全流程唯一的 await 点，
    // 之后建 DOM、置位、建 tween 全是同步的。没有任何 lazy fetch 分支：转身那
    // 一刻才去取 SVG 的话，逐帧 seek 下加载时机不可控，首批帧会抓到空视图。
    const svgTexts = await Promise.all(viewNames.map((name) => fetchText(`${packDir}/${rigOf(name).file}`)));

    const stageW = stage.clientWidth;
    const stageH = stage.clientHeight;
    if (!stageW || !stageH) {
      throw new Error('舞台尺寸为 0：cast.mount 必须在舞台已经有布局尺寸之后调用');
    }
    if (getComputedStyle(stage).position === 'static') stage.style.position = 'relative';

    const [ratioLo, ratioHi] = pack.scale.heightRatio;
    const heightRatio = opts.heightRatio == null ? (ratioLo + ratioHi) / 2 : opts.heightRatio;
    // 与 x / ground / facing 同一口径：越界就抛错，不静默接受。
    // heightRatio 为负会算出负的 scale 与负的高度，角色上下翻转还缩到画外，
    // 而且一路不报错。
    if (typeof heightRatio !== 'number' || !isFinite(heightRatio) || heightRatio <= 0 || heightRatio >= 1) {
      throw new Error(`cast.mount 的 heightRatio 必须是 0–1 之间的归一化比例，收到 ${opts.heightRatio}`);
    }
    const drawnHeight = stageH * heightRatio;

    // holder 是一个 0×0 的锚点，落在 (x, ground) 上——也就是角色两脚之间。
    // 尺寸为 0 意味着默认的 transform-origin（50% 50%）正好是这个锚点，
    // 镜像和转身压扁都绕它发生，角色不会横向漂移。
    const holder = document.createElement('div');
    holder.className = 'cast-actor';
    holder.dataset.castId = pack.id;
    holder.style.position = 'absolute';
    holder.style.width = '0';
    holder.style.height = '0';
    // 锚点只跟 x/ground 有关，与视图无关：所有视图共用同一个落脚点。
    holder.style.left = `${round3(stageW * x)}px`;
    holder.style.top = `${round3(stageH * ground)}px`;

    // body 承担 scaleX（朝向镜像 + 转身压扁）与呼吸的上下起伏，
    // 与 holder 的定位分开，两者互不覆盖对方的 transform。
    const body = document.createElement('div');
    body.className = 'cast-body';
    body.style.position = 'absolute';
    body.style.width = '0';
    body.style.height = '0';
    holder.appendChild(body);

    const views = {};
    const jointsByName = {};
    viewNames.forEach((name, index) => {
      const rig = rigOf(name);
      const svg = parseSVG(svgTexts[index], `${packDir}/${rig.file}`);
      const prefix = `cast-${pack.id}-${name}-`;
      const scope = `${pack.id}-${name}`;
      namespaceIds(svg, prefix);
      scopeStyleRules(svg, `[data-cast-scope="${scope}"]`, `${packDir}/${rig.file}`);

      const layout = actorLayout({
        stageW, stageH, x, ground, heightRatio, viewBox: rig.viewBox, baselineY: rig.baselineY,
      });
      svg.setAttribute('width', round3(layout.width));
      svg.setAttribute('height', round3(layout.height));
      svg.style.display = 'block';
      svg.style.overflow = 'visible';

      const wrapper = document.createElement('div');
      wrapper.className = 'cast-view';
      wrapper.dataset.castView = name;
      // 作用域挂在 wrapper（HTML 元素）上而不是 svg 根上：这样 `[scope] svg`
      // 这类选择器连根元素自己也能命中。
      wrapper.dataset.castScope = scope;
      wrapper.style.position = 'absolute';
      wrapper.style.left = `${round3(layout.viewLeft)}px`;
      wrapper.style.top = `${round3(layout.viewTop)}px`;
      // 预置隐藏，转身时只切 visibility：用 visibility 而不是 display，
      // 是为了不触发布局，切换在任意帧都是同一个开销。
      wrapper.style.visibility = 'hidden';
      wrapper.appendChild(svg);
      body.appendChild(wrapper);

      const joints = {};
      for (const [jointName, joint] of Object.entries(rig.joints)) {
        // 用属性选择器而不是 #id，省掉转义：关节名来自 yaml，可能是小驼峰。
        const group = svg.querySelector(`[id="${prefix}${JOINT_PREFIX}${jointName}"]`);
        if (!group) {
          throw new Error(`角色 ${pack.id} 的视图 ${name} 里找不到 <g id="${JOINT_PREFIX}${jointName}">`);
        }
        // angle 由 GSAP 的数值 tween 拥有；呼吸不写这里，它是时间的纯函数，
        // 每帧现算后与 angle 合成（见 applyJoint）。
        const state = { group, pivot: joint.pivot, range: joint.rotate, angle: 0 };
        joints[jointName] = state;
        (jointsByName[jointName] = jointsByName[jointName] || []).push(state);
      }

      // data-no-mirror 的元素在镜像时要二次翻转，否则举牌镜头会出反字。
      // transform-box 必须显式写成 fill-box：SVG 元素的默认参照框是整个
      // viewBox，只设 transform-origin: center 会绕画面中心翻，元素直接飞到
      // 另一侧去。
      const noMirror = Array.from(svg.querySelectorAll('[data-no-mirror]'));
      for (const node of noMirror) {
        node.style.transformBox = 'fill-box';
        node.style.transformOrigin = 'center';
      }

      views[name] = { wrapper, svg, rig, joints, noMirror };
    });

    stage.appendChild(holder);

    // ======================= 舞台状态 =======================
    //
    // 转身/翻转/呼吸只在装配期往 schedule 里追加一条，每一帧由 renderFrame 从
    // schedule 现算。为什么不能用「onUpdate 里写共享状态」这条捷径：
    //
    //   onUpdate 只在自己那段区间内触发，区间之外的帧要靠别的东西兜底。如果
    //   每条 tween 各自往共享状态里写自己的起止态，那么某一帧最终显示什么，
    //   取决于「最后一次写是谁写的」，也就是 timeline 向后 seek 时以什么顺序
    //   渲染子 tween。实测 GSAP 3.15 是倒序渲染，结果碰巧对——但那是内部实现
    //   细节，换个 timeline 实现、GSAP 改版、或把 tween 拆进嵌套 timeline 就
    //   出错帧，而顺序播放和本地预览永远看不出来。
    //
    //   现算则与渲染顺序完全无关：哪条 tween 的 onUpdate 触发都行，一条都没
    //   触发（第 0 帧、或宿主用 seek() 抑制了事件）也照样正确。
    const stageSchedule = []; // { start, end, seq, sign, mirror }
    const breathSchedule = []; // { start, end, intensity }
    const initialStage = { view: initialView, sign: facing === 'left' ? -1 : 1 };
    const breathTargets = breathTargetsFor(Object.keys(jointsByName));

    // breathStateAt 每帧要被每个关节问一次，按 time 做一格缓存。
    const breathCache = { time: NaN, value: null };
    function breathAt(time) {
      if (breathCache.time !== time) {
        breathCache.time = time;
        breathCache.value = breathStateAt(breathSchedule, breathTargets, time, drawnHeight * BREATH_BOB_RATIO);
      }
      return breathCache.value;
    }

    // 用 rotate(角度 支点x 支点y) 三参数写法，与 Go 侧 cast.PoseSVG 注入的
    // 完全一致——验收用的姿势预览图和成片里的角色必须是同一个姿势。
    // 也因此不依赖 CSS transform-origin，rig.svg 里禁止写 transform-origin
    // 这条校验才站得住：支点的唯一真相在 character.yaml。
    function applyJoint(state, jointName, time) {
      const breath = breathAt(time).joints[jointName] || 0;
      const angle = clamp(state.angle + breath, state.range[0], state.range[1]);
      state.group.setAttribute('transform', `rotate(${round3(angle)} ${state.pivot[0]} ${state.pivot[1]})`);
    }

    function eachJoint(fn) {
      for (const name of viewNames) {
        for (const [jointName, state] of Object.entries(views[name].joints)) fn(state, jointName, name);
      }
    }

    // lastTime 只是「上一次渲染到哪一帧」的回声，供 pose() 这种不带时间的同步
    // 调用复用。它不参与任何取值判断。
    let lastTime = 0;

    // renderFrame 是本库唯一的写 DOM 入口。
    //
    // 它重算的是**视图、朝向与呼吸**——这三样是时间的纯函数，同一个 t 永远
    // 给出同样的结果。**姿势不在其中**：关节角 state.angle 由时间线上的
    // GSAP tween 拥有，renderFrame 只是把「当前的」角度和呼吸合成后写下去。
    //
    // 所以 renderFrame(t) 单独调用不构成「把第 t 帧画出来」。要出某一帧，
    // 必须先把时间线推到 t（tl.time(t)），让姿势 tween 也回算。对外暴露的
    // handle.renderAt 会检查这一点并抛错，理由见那里的注释。
    function renderFrame(time) {
      lastTime = time;
      const shape = stageStateAt(stageSchedule, initialStage, time);
      if (!views[shape.view]) {
        // 走到这里说明 turns 里写了一个没声明的视图。不抛错的话所有 wrapper
        // 都会被设成 hidden，角色凭空消失且不报错。
        throw new Error(`角色 ${pack.id} 在 t=${time} 需要视图 ${shape.view}，但它没有被声明`);
      }
      for (const name of viewNames) {
        views[name].wrapper.style.visibility = name === shape.view ? 'visible' : 'hidden';
      }
      body.style.transform = `translateY(${round3(breathAt(time).bobY)}px) scaleX(${round3(shape.sign)})`;
      const mirrored = shape.sign < 0;
      for (const name of viewNames) {
        for (const node of views[name].noMirror) {
          node.style.transform = mirrored ? 'scaleX(-1)' : 'none';
        }
      }
      eachJoint((state, jointName) => applyJoint(state, jointName, time));
    }

    function poseAngles(name) {
      const target = (pack.poses || {})[name];
      if (!target) {
        throw new Error(`角色 ${pack.id} 没有姿势 ${name}，已声明的是 ${Object.keys(pack.poses || {}).join('、')}`);
      }
      return target;
    }

    // ======================= 装配期状态 =======================

    // current 只在「搭时间线的那一刻」使用，用来算下一次转身的起点。
    // 它不参与任何一帧的取值——渲染取值一律走 stageStateAt。
    const current = { view: initialView, sign: initialStage.sign, until: -Infinity };

    // assertForward：转身与翻转必须按时间先后声明，且区间不得重叠。
    //
    // current 是「声明顺序」游标：turn() 要用它算 turns 的键、flip() 要用它
    // 算起始朝向。乱序声明（先写 at:5 的 flip 再写 at:2 的 turn）会让后声明
    // 的那条拿到未来的状态当起点，两条都错，而且不报错。
    //
    // 游标记的是上一条的**结束时刻**而不是起点，这样重叠与「at 相同」一并挡住：
    // stageStateAt 取的是「最后一条已经开始的」条目，两条区间重叠时，后一条
    // 从它的 start 那一刻起就整个盖住前一条——前一次转身的后半段被静默丢弃，
    // 而且新条目在自己的进度 0 处是满宽的，于是画面上就是「角色还有可见宽度
    // 时硬切了视图」，正是硬规则 2/6 要防的那件事。
    //
    // 姿势 tween 与 speak/listen 不动游标，可任意顺序穿插、也可与转身重叠。
    function assertForward(at, end, method) {
      if (at < current.until - 1e-9) {
        throw new Error(
          `角色 ${pack.id} 的 cast.${method}(at=${at}) 与上一次转身/翻转的区间重叠` +
            `（那一次到 ${round3(current.until)} 才结束）：转身与翻转必须按时间先后、互不重叠地声明，` +
            '否则前一次的后半段会被丢弃，并在角色还有可见宽度时硬切视图' +
            '（pose、to、speak、listen 不受此限）'
        );
      }
      current.until = end;
    }

    // 覆盖全片的驱动 tween：区间之外的帧也要重算，不能只在事件窗口里出帧。
    // 它自己不携带任何状态，proxy 的值根本不被读——渲染取值全从 schedule 现算。
    let driver = null;
    let driverCreated = false; // 与 driver 分开：拿不到 tween 引用也只准建一条
    let coverage = opts.duration || 0;
    function ensureCoverage(tl, until) {
      coverage = Math.max(coverage, until);
      if (!driverCreated) {
        driverCreated = true;
        tl.to({ t: 0 }, { t: 1, duration: coverage, ease: 'none', onUpdate: () => renderFrame(tl.time()) }, 0);
        // recent() 是 GSAP 的接口。拿不到引用就没法再延长时长，此时驱动只覆盖
        // 到第一次绑定时的那个 until——所以 SKILL.md 建议显式传 mount({duration})。
        driver = typeof tl.recent === 'function' ? tl.recent() : null;
        return;
      }
      if (driver && typeof driver.duration === 'function') driver.duration(coverage);
    }

    // 所有 tween 必须挂在同一条 timeline 上：混用两条会让 schedule 里的时间
    // 分属不同坐标系，算出来的帧全是错的，而且不报错。
    let boundTimeline = null;
    function bind(tl, method, until) {
      requireTimeline(tl, method);
      if (boundTimeline && boundTimeline !== tl) {
        throw new Error(`角色 ${pack.id} 的动画必须全部挂在同一条 timeline 上，cast.${method}() 收到了另一条`);
      }
      if (!boundTimeline) {
        boundTimeline = tl;
        // timeline 自己的 seek 也要转发出帧，但只渲染绑在它上面的角色。
        wrapSeekOwner(tl, true);
      }
      ensureCoverage(tl, until);
      return tl;
    }

    const handle = {
      pack,
      id: pack.id,
      root: holder,
      body,
      views,
      viewNames,
      // renderAt 重算视图/朝向/呼吸，**不重算姿势**——姿势由时间线拥有。
      //
      // 因此它只在「时间线已经在 t 上」时才有意义：宿主 seek 之后的补渲染
      // （本库内部的 seek 包装走的就是这条），或者纯粹刷新一次当前帧。
      // 不同步时直接抛错，而不是画出一张姿势停在别处的帧——
      // 这个接口被 SKILL.md 用作验收基准，让它给出错误结论比它不存在更糟。
      renderAt(time) {
        if (boundTimeline && Math.abs(boundTimeline.time() - time) > 1e-6) {
          // 最常见的成因不是「调用者搞错了时间」，而是时间线根本到不了 t：
          // GSAP 会把 time(t) 钳到 duration()，于是 t 超出时间线长度时，
          // 位置永远对不上。这种情况下只报「位置不一致」会把人引向错误的方向，
          // 所以先点名真实原因。
          const beyond = time > boundTimeline.duration() + 1e-6;
          throw new Error(
            `cast.renderAt(${time}) 与时间线当前位置 ${round3(boundTimeline.time())} 不一致：` +
              (beyond
                ? `时间线总长只有 ${round3(boundTimeline.duration())} 秒，time(${time}) 被钳住了。` +
                  '给 cast.mount({ duration: 镜头时长 }) 让驱动 tween 覆盖到片尾，或检查镜头内容是否真有这么长。'
                : 'renderAt 只重算视图/朝向/呼吸，姿势由时间线上的 tween 拥有。' +
                  '要出某一帧请先 tl.time(t)，出帧会自动完成。')
          );
        }
        renderFrame(time);
        return handle;
      },
      // stageAt 是纯取值，不碰 DOM，任何时刻问任何 t 都安全。
      stageAt: (time) => stageStateAt(stageSchedule, initialStage, time),
      get svg() {
        return views[stageStateAt(stageSchedule, initialStage, lastTime).view].svg;
      },

      // pose 是同步置位，且对所有视图同时置位。
      // 同步：第 0 帧必须已经是终态，不留空台或入场中间态。
      // 所有视图：姿势跨视图共享，只置当前视图的话，转身之后姿势会掉回 idle。
      pose(name) {
        const target = poseAngles(name);
        eachJoint((state, jointName) => {
          state.angle = target[jointName] || 0;
        });
        renderFrame(lastTime);
        return handle;
      },

      // to 把姿势 tween 到目标，同样覆盖所有视图。
      // 数值插值交给 GSAP：链式 to() 在向后 seek 时由 GSAP 自己倒序渲染回
      // 正确的起始值，这是它的既定语义（scrub 的基础），不需要我们复刻。
      to(tl, name, { at = 0, dur = 0.4, ease = 'power2.out' } = {}) {
        bind(tl, 'to', at + dur);
        const target = poseAngles(name);
        eachJoint((state, jointName) => {
          tl.to(
            state,
            {
              angle: target[jointName] || 0,
              duration: dur,
              ease,
              onUpdate: () => applyJoint(state, jointName, tl.time()),
            },
            at
          );
        });
        return handle;
      },

      // flip 是左右镜像翻转：scaleX 从 +1 连续走到 -1，过零那一瞬间最窄。
      // 这是「朝向」维度，与「视图」维度正交。
      flip(tl, { at = 0, dur = DEFAULT_TURN_MS / 1000 } = {}) {
        bind(tl, 'flip', at + dur);
        assertForward(at, at + dur, 'flip');
        stageSchedule.push({ start: at, end: at + dur, seq: [current.view], sign: current.sign, mirror: true });
        current.sign = -current.sign;
        return handle;
      },

      // turn 是转身，不是把镜像 tween 过去——镜像动画在宽度过零时会翻成反面，
      // 读起来是卡片翻面而不是转身。做法是 scaleX 压到 0 再回到 1，在过零那
      // 一帧换视图，中间按 turns[key].via 经过四分之三侧。
      //
      // 只声明了默认视图的角色包，turn() 退化成镜像翻转（flip）：画三视图的
      // 准入成本是三倍，只有真有转身戏份的主角才值得画全。
      turn(tl, toView, { at = 0, dur } = {}) {
        const key = `${current.view}->${toView}`;
        const spec = (pack.turns || {})[key];
        if (viewNames.length === 1) {
          const seconds = dur == null ? (spec ? spec.durationMs : DEFAULT_TURN_MS) / 1000 : dur;
          return handle.flip(tl, { at, dur: seconds });
        }
        if (!spec) {
          throw new Error(
            `角色 ${pack.id} 未声明转身 ${key}：请在 character.yaml 的 turns 里显式写出这条路径（不做自动寻路，哪条路径好看是创作判断）`
          );
        }
        const seconds = dur == null ? spec.durationMs / 1000 : dur;
        bind(tl, 'turn', at + seconds);
        assertForward(at, at + seconds, 'turn');
        stageSchedule.push({
          start: at,
          end: at + seconds,
          seq: [current.view, ...(spec.via || []), toView],
          sign: current.sign,
          mirror: false,
        });
        current.view = toView;
        return handle;
      },

      // speak / listen 的区别只有幅度，都不做口型。
      speak(tl, options = {}) {
        return handle.breathe(tl, { ...options, intensity: 1 });
      },
      listen(tl, options = {}) {
        return handle.breathe(tl, { ...options, intensity: 0.35 });
      },
      breathe(tl, { from = 0, to = 0, intensity = 1 } = {}) {
        if (!(to - from > 0)) {
          throw new Error(`cast speak/listen 的区间必须为正：from=${from} to=${to}`);
        }
        bind(tl, 'speak/listen', to);
        breathSchedule.push({ start: from, end: to, intensity });
        return handle;
      },
    };

    // 注册进模块级注册表时带上「挂在哪条 timeline 上」，让宿主 seek 的转发
    // 能按时间线隔离，不会推 A 把 B 也渲染了。
    actors.push({ render: renderFrame, timeline: () => boundTimeline });
    installWindowSeekHook('__hf');
    installWindowSeekHook('__player');
    handle.pose(opts.pose || 'idle');
    return handle;
  }

  return {
    mount,
    DEFAULT_VIEW,
    SCHEMA,
    // pure 里全是无副作用的取值函数，导出供 assets_test.go 的 node 断言与
    // 交付前自检直接求值。改这里等于改动画的定义，改完必须同步那些断言。
    pure: {
      pinchScaleAtPhase,
      viewIndexAtPhase,
      mirrorScaleAtProgress,
      breathEnvelope,
      actorLayout,
      stageStateAt,
      breathStateAt,
      breathTargetsFor,
    },
  };
})();

// 以 <script src="…/cast.js"> 引入时，上面的 const 已经是全局绑定；
// 这一行让 window.cast 在模块作用域里引入时也能拿到。
if (typeof window !== 'undefined') window.cast = cast;
