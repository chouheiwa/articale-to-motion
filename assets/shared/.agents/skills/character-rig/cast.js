// character-rig 驱动库：把角色包（character pack）挂进 HyperFrames 镜头。
//
// ============================ 四条硬约束 ============================
//
// 1) 本库自己不建 timeline、不碰 requestAnimationFrame、不用定时器。
//    所有 tween 都挂在调用方传入的那条 paused timeline 上。HyperFrames 是
//    单条 paused timeline + 逐帧 seek 出帧：任何不受那条 timeline 管的动画，
//    本地播放看着正常、成片是错的，而且退出码为 0、不报错。
//
// 2) 每个 onUpdate 只是「当前进度 -> 画面」的纯函数。不翻标志位、不做累积、
//    不读上一帧。渲染机按任意帧 seek，不保证顺序、不保证只走一遍——靠
//    「播过去了所以状态变了」的实现会给出乱掉的帧，而本地预览完全正常。
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
  const HEAD_JOINTS = /head|neck/i;
  const TAIL_JOINTS = /tail/i;

  const clamp = (value, lo, hi) => Math.min(hi, Math.max(lo, value));
  const round3 = (value) => Math.round(value * 1000) / 1000;

  // requireTimeline 把「忘了传 timeline」变成一个响亮的错误。
  // 这类错误如果放过去，动画要么完全不动、要么用别的机制跑起来，而成片错、
  // 退出码 0 是这个项目最贵的失败模式。
  function requireTimeline(tl, method) {
    if (!tl || typeof tl.to !== 'function') {
      throw new Error(
        `cast.${method}() 的第一个参数必须是镜头那条 paused GSAP timeline：` +
          '角色动画必须挂在它上面，否则逐帧 seek 出的成片是错的且不报错'
      );
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
      if (!node.attributes) continue;
      for (const attr of Array.from(node.attributes)) {
        if (!attr.value.includes('#')) continue;
        let next = attr.value;
        for (const [from, to] of renamed) {
          for (const quote of ['', '"', "'"]) {
            next = next.split(`url(${quote}#${from}${quote})`).join(`url(${quote}#${to}${quote})`);
          }
          if (next === `#${from}`) next = `#${to}`;
        }
        if (next !== attr.value) node.setAttribute(attr.name, next);
      }
    }
  }

  // mount 把角色挂上舞台。返回的 handle 是之后所有动作的入口。
  //
  // opts:
  //   pack        角色包目录（相对镜头 HTML），例如 'cast/heiwa'
  //   x           水平位置，画面宽度的归一化比例，锚在角色左右中线
  //   ground      地平线，画面高度的归一化比例，角色脚底（baselineY）对齐它
  //   facing      'right'（默认）| 'left'，左右镜像，与 view 正交
  //   view        初始视图名，默认 'front'
  //   pose        初始姿势名，默认 'idle'
  //   heightRatio 覆盖角色高度占比，默认取包内 scale.heightRatio 的中值
  async function mount(selector, opts = {}) {
    const stage = typeof selector === 'string' ? document.querySelector(selector) : selector;
    if (!stage) throw new Error(`cast.mount 找不到舞台 ${selector}`);

    const packDir = String(opts.pack || '').replace(/\/+$/, '');
    if (!packDir) throw new Error('cast.mount 缺少 pack：角色包目录（相对镜头 HTML）');

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
    // 之后的建 DOM、置位、建 tween 全是同步的。
    const svgTexts = await Promise.all(viewNames.map((name) => fetchText(`${packDir}/${rigOf(name).file}`)));

    const stageW = stage.clientWidth;
    const stageH = stage.clientHeight;
    if (!stageW || !stageH) {
      throw new Error('舞台尺寸为 0：cast.mount 必须在舞台已经有布局尺寸之后调用');
    }
    // 舞台必须是定位上下文，否则角色会跑到页面左上角去。
    if (getComputedStyle(stage).position === 'static') stage.style.position = 'relative';

    const [ratioLo, ratioHi] = pack.scale.heightRatio;
    const heightRatio = opts.heightRatio == null ? (ratioLo + ratioHi) / 2 : opts.heightRatio;
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
    holder.style.left = `${round3(stageW * clamp(opts.x == null ? 0.5 : opts.x, 0, 1))}px`;
    holder.style.top = `${round3(stageH * clamp(opts.ground == null ? 0.8 : opts.ground, 0, 1))}px`;

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
      namespaceIds(svg, prefix);

      const [vbX, vbY, vbW, vbH] = rig.viewBox;
      const scale = drawnHeight / vbH;
      svg.setAttribute('width', round3(vbW * scale));
      svg.setAttribute('height', round3(vbH * scale));
      svg.style.display = 'block';
      svg.style.overflow = 'visible';

      const wrapper = document.createElement('div');
      wrapper.className = 'cast-view';
      wrapper.dataset.castView = name;
      wrapper.style.position = 'absolute';
      wrapper.style.left = `${round3(-(vbW * scale) / 2)}px`;
      wrapper.style.top = `${round3(-(rig.baselineY - vbY) * scale)}px`;
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
        // angle 是姿势，breath 是呼吸叠加量，分开存、合成后写一次 transform：
        // 两条 tween 各写各的字段，谁先谁后渲染结果都一样。
        const state = { group, pivot: joint.pivot, range: joint.rotate, angle: 0, breath: 0 };
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

    // ---------------- 渲染：全部由这两个纯函数收口 ----------------

    // stageState 的每个字段都由某条 tween 写成「自己那条进度的纯函数」，
    // 没有任何字段是累积出来的；renderStage 只是把它画出来。
    const stageState = {
      view: initialView,
      scaleX: opts.facing === 'left' ? -1 : 1,
      bobY: 0,
    };

    function renderStage() {
      for (const name of viewNames) {
        views[name].wrapper.style.visibility = name === stageState.view ? 'visible' : 'hidden';
      }
      body.style.transform = `translateY(${round3(stageState.bobY)}px) scaleX(${round3(stageState.scaleX)})`;
      const mirrored = stageState.scaleX < 0;
      for (const name of viewNames) {
        for (const node of views[name].noMirror) {
          node.style.transform = mirrored ? 'scaleX(-1)' : 'none';
        }
      }
    }

    // 用 rotate(角度 支点x 支点y) 三参数写法，与 Go 侧 cast.PoseSVG 注入的
    // 完全一致——验收用的姿势预览图和成片里的角色必须是同一个姿势。
    // 也因此不依赖 CSS transform-origin，rig.svg 里禁止写 transform-origin
    // 这条校验才站得住：支点的唯一真相在 character.yaml。
    function applyJoint(state) {
      const angle = clamp(state.angle + state.breath, state.range[0], state.range[1]);
      state.group.setAttribute('transform', `rotate(${round3(angle)} ${state.pivot[0]} ${state.pivot[1]})`);
    }

    function eachJoint(fn) {
      for (const name of viewNames) {
        for (const [jointName, state] of Object.entries(views[name].joints)) fn(state, jointName, name);
      }
    }

    function poseAngles(name) {
      const target = (pack.poses || {})[name];
      if (!target) {
        throw new Error(`角色 ${pack.id} 没有姿势 ${name}，已声明的是 ${Object.keys(pack.poses || {}).join('、')}`);
      }
      return target;
    }

    // 呼吸挑哪些关节：有头用头，有尾巴用尾巴，都没有就退回第一个关节做最小
    // 幅度的起伏。刻意不做口型音素同步——这个画风是面无表情的简笔画，加嘴型
    // 会毁掉它；「在说话」由头部起伏、呼吸和尾巴表达，speak 与 listen 的差别
    // 只是幅度。
    const breathTargets = (() => {
      const names = Object.keys(jointsByName).sort();
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
    })();

    // breathe 把一段区间内的呼吸写成进度的纯函数：sin(相位) × 幅度 × 包络。
    // 包络在区间两端归零，所以 seek 到区间之外时呼吸叠加量恰好是 0，
    // 不会有残留角度粘在身上。
    function breathe(tl, { from = 0, to = 0, intensity = 1 } = {}) {
      const seconds = to - from;
      if (!(seconds > 0)) {
        throw new Error(`cast speak/listen 的区间必须为正：from=${from} to=${to}`);
      }
      const proxy = { t: 0 };
      tl.to(
        proxy,
        {
          t: 1,
          duration: seconds,
          ease: 'none',
          onUpdate: () => {
            const t = proxy.t;
            const envelope = clamp(Math.min(t, 1 - t) / BREATH_RAMP, 0, 1);
            const elapsed = t * seconds;
            for (const target of breathTargets) {
              const wave = Math.sin(2 * Math.PI * (elapsed / target.period + target.phase));
              const amount = wave * target.degrees * intensity * envelope;
              for (const state of jointsByName[target.name]) {
                state.breath = amount;
                applyJoint(state);
              }
            }
            const bob = Math.sin(2 * Math.PI * (elapsed / BREATH_PERIOD + 0.5));
            stageState.bobY = bob * drawnHeight * BREATH_BOB_RATIO * intensity * envelope;
            renderStage();
          },
        },
        from
      );
    }

    // current 只在「搭时间线的那一刻」使用，用来算下一次转身的起点。
    // 它不参与任何一帧的取值——渲染取值一律走 stageState。
    const current = { view: initialView, sign: stageState.scaleX };

    const handle = {
      pack,
      id: pack.id,
      root: holder,
      body,
      views,
      viewNames,
      get svg() {
        return views[current.view].svg;
      },

      // pose 是同步置位，且对所有视图同时置位。
      // 同步：第 0 帧必须已经是终态，不留空台或入场中间态。
      // 所有视图：姿势跨视图共享，只置当前视图的话，转身之后姿势会掉回 idle。
      pose(name) {
        const target = poseAngles(name);
        eachJoint((state, jointName) => {
          state.angle = target[jointName] || 0;
          applyJoint(state);
        });
        renderStage();
        return handle;
      },

      // to 把姿势 tween 到目标，同样覆盖所有视图。
      to(tl, name, { at = 0, dur = 0.4, ease = 'power2.out' } = {}) {
        requireTimeline(tl, 'to');
        const target = poseAngles(name);
        eachJoint((state, jointName) => {
          tl.to(
            state,
            { angle: target[jointName] || 0, duration: dur, ease, onUpdate: () => applyJoint(state) },
            at
          );
        });
        return handle;
      },

      // flip 是左右镜像翻转：scaleX 从 +1 连续走到 -1，过零那一瞬间最窄。
      // 这是「朝向」维度，与「视图」维度正交。
      flip(tl, { at = 0, dur = DEFAULT_TURN_MS / 1000 } = {}) {
        requireTimeline(tl, 'flip');
        const startSign = current.sign;
        const startView = current.view;
        const proxy = { t: 0 };
        tl.to(
          proxy,
          {
            t: 1,
            duration: dur,
            ease: 'none',
            onUpdate: () => {
              stageState.view = startView;
              stageState.scaleX = startSign * Math.cos(Math.PI * proxy.t);
              renderStage();
            },
          },
          at
        );
        current.sign = -startSign;
        return handle;
      },

      // turn 是转身，不是把镜像 tween 过去——镜像动画在宽度过零时会翻成
      // 反面，读起来是卡片翻面而不是转身。做法是 scaleX 压到 0 再回到 1，
      // 在过零那一帧换视图，中间按 turns[key].via 经过四分之三侧。
      //
      // 只声明了默认视图的角色包，turn() 退化成镜像翻转（flip）：画三视图的
      // 准入成本是三倍，只有真有转身戏份的主角才值得画全。
      turn(tl, toView, { at = 0, dur } = {}) {
        requireTimeline(tl, 'turn');
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
        const seq = [current.view, ...(spec.via || []), toView];
        const segments = seq.length - 1;
        const startSign = current.sign;
        const proxy = { t: 0 };
        tl.to(
          proxy,
          {
            t: 1,
            duration: dur == null ? spec.durationMs / 1000 : dur,
            ease: 'none',
            // 视图与 scaleX 都是 proxy.t 的纯函数，每次求值只依赖当前进度。
            // 不得在这里翻标志位或做累积——渲染机按任意帧 seek，不保证顺序、
            // 不保证只走一遍，靠「播过去了所以状态变了」的实现会给出乱掉的帧，
            // 而本地预览完全正常。
            //
            // phase 走 [0, segments]，压扁量取 |cos(π·phase)|，零点落在
            // phase 的半整数处；视图索引取 round(phase)，切换点正好是同一批
            // 半整数。两者必须对齐：错开一点点，就会在角色还有宽度的时候换
            // 视图，肉眼看是"闪一下换了张图"。
            onUpdate: () => {
              const phase = proxy.t * segments;
              stageState.view = seq[clamp(Math.round(phase), 0, segments)];
              stageState.scaleX = startSign * Math.abs(Math.cos(Math.PI * phase));
              renderStage();
            },
          },
          at
        );
        current.view = toView;
        return handle;
      },

      // speak / listen 的区别只有幅度，都不做口型。
      speak(tl, { from, to } = {}) {
        requireTimeline(tl, 'speak');
        breathe(tl, { from, to, intensity: 1 });
        return handle;
      },
      listen(tl, { from, to } = {}) {
        requireTimeline(tl, 'listen');
        breathe(tl, { from, to, intensity: 0.35 });
        return handle;
      },
    };

    handle.pose(opts.pose || 'idle');
    return handle;
  }

  return { mount, DEFAULT_VIEW, SCHEMA };
})();

// 以 <script src="…/cast.js"> 引入时，上面的 const 已经是全局绑定；
// 这一行让 window.cast 在模块作用域里引入时也能拿到。
if (typeof window !== 'undefined') window.cast = cast;
