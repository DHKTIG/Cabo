import { state, snd, sleep, clamp, reduceMotion } from './core.mjs';
import { $, tone, cardFace } from './dom.mjs';

export function comboFlare(text, color, tier) {
  const chip = document.querySelector('.combo-chip');
  const base = chip ? chip.getBoundingClientRect() : $('meBar').getBoundingClientRect();
  const el = document.createElement('div');
  el.className = 'combo-flare';
  el.textContent = text;
  if (color) el.style.color = color;
  el.style.top = base.top - 6 + 'px';
  el.dataset.tier = tier;
  $('fx').appendChild(el);
  // 连击标识贴在名条右端时，爆发字条可能被视口切掉，夹一下
  const w = el.offsetWidth;
  el.style.left = clamp(base.left + base.width / 2, w / 2 + 8, innerWidth - w / 2 - 8) + 'px';
  setTimeout(() => el.remove(), 1100);
}

export function toast(msg) {
  snd('error');
  const d = document.createElement('div');
  d.className = 'pill';
  d.style.color = '#f0a5a5';
  d.style.borderColor = 'rgba(224,82,82,.5)';
  d.style.left = '50%';
  d.style.top = '86%';
  d.textContent = msg;
  $('fx').appendChild(d);
  setTimeout(() => d.remove(), 1800);
}

/* ---------------- 几何 ---------------- */

function rectOf(sel) {
  const el = document.querySelector(sel);
  return el ? el.getBoundingClientRect() : null;
}

function deckRect() { return rectOf('.deck-stack'); }
function discardRect() { return rectOf('.discard-holder'); }

function seatHandRect(pid) {
  const seat = state.seatEls.get(pid);
  if (!seat) return null;
  const h = seat.querySelector('.seat-hand');
  return (h || seat).getBoundingClientRect();
}

function cardRect(pid, slot) {
  const el = document.querySelector(`.card[data-ref="${pid}:${slot}"]`);
  return el ? el.getBoundingClientRect() : seatHandRect(pid);
}

/* ---------------- 转场与「完成」演出 ---------------- */

// 圆形扩散转场：从指落点铺满整屏，再原路收回去露出新画面。
// 用独立圆形合成层做 scale，避免全屏 clip-path 每帧重绘。
export function circleWipe(x, y, atCover) {
  if (reduceMotion() || document.visibilityState === 'hidden') {
    atCover();
    return Promise.resolve();
  }
  const el = document.createElement('div');
  el.className = 'wipe';
  el.style.setProperty('--wx', x + 'px');
  el.style.setProperty('--wy', y + 'px');
  $('fx').appendChild(el);
  const far = (cx, cy) => Math.hypot(cx - x, cy - y);
  const r = Math.max(
    far(0, 0),
    far(innerWidth, 0),
    far(0, innerHeight),
    far(innerWidth, innerHeight)
  ) + 40;
  el.style.width = r * 2 + 'px';
  el.style.height = r * 2 + 'px';
  el.style.margin = -r + 'px';
  const grow = [{ transform: 'scale(0)' }, { transform: 'scale(1)' }];
  const shrink = [grow[1], grow[0]];
  const run = (kf, dur, delay) =>
    el.animate(kf, { duration: dur, delay, easing: 'cubic-bezier(.22,.61,.36,1)', fill: 'forwards' }).finished;
  // 窗口最小化或切后台时 WAAPI 不推进，finished 会永远挂着 —— 那等于「点开始游戏没反应」。
  // 所以整条链兜一个超时：动画放不出来也必须把画面换过去，但只能换一次。
  let swapped = false;
  const cover = () => {
    if (swapped) return;
    swapped = true;
    atCover();
  };
  const guard = sleep(1500).then(cover);
  let chain;
  try {
    chain = run(grow, 420, 0).then(() => {
      cover();
      return run(shrink, 460, 60);
    });
  } catch {
    cover();
  }
  return Promise.race([chain || Promise.resolve(), guard]).catch(cover).then(() => el.remove());
}

export const centerOf = (el) => {
  const b = el && el.getBoundingClientRect ? el.getBoundingClientRect() : null;
  if (!b || !b.width) return { x: innerWidth / 2, y: innerHeight / 2 };
  return { x: b.left + b.width / 2, y: b.top + b.height / 2 };
};

// 「完成」演出：一块色圆收拢放大 → 一圈小粒子迸开。
// 用在喊 CABO 上——这是这局游戏真正的 done。用户明确不要那把勾，只留动势。
export function doneBurst(el, tone, hold) {
  if (!el || reduceMotion()) return;
  const { x, y } = centerOf(el);
  const layer = document.createElement('div');
  layer.className = 'burst';
  layer.style.left = x + 'px';
  layer.style.top = y + 'px';
  const n = 18;
  for (let i = 0; i < n; i++) {
    const a = (i / n) * Math.PI * 2 + 0.22;
    const d = 44 + (i % 4) * 15;
    const p = document.createElement('i');
    p.style.setProperty('--tx', Math.cos(a) * d + 'px');
    p.style.setProperty('--ty', Math.sin(a) * d + 'px');
    p.style.animationDelay = 90 + (i % 5) * 24 + 'ms';
    if (i % 4 === 0) p.dataset.k = 'big';
    layer.appendChild(p);
  }
  layer.insertAdjacentHTML(
    'afterbegin',
    `<svg class="burst-ring" viewBox="0 0 44 44" style="--tone:${tone || 'var(--sun)'}">
       <circle class="halo" cx="22" cy="22" r="20"></circle>
       <circle class="core" cx="22" cy="22" r="14"></circle>
     </svg>`
  );
  $('fx').appendChild(layer);
  // hold 只给截图工装用：无头页面的虚拟时间跑得飞快，1s 就收走永远拍不到
  setTimeout(() => layer.remove(), hold || 1000);
}

// 一个动作同时惊动三处：分数、进度条、已知点数估值。
// 参考稿那条「一个动作三处起变」是它最上瘾的地方，牌局里正好对应得上。
export function syncFlash() {
  if (reduceMotion()) return;
  const bar = $('meBar');
  if (!bar) return;
  ['.me-progress', '.me-est', '.combo-chip'].forEach((sel) => {
    const el = bar.querySelector(sel);
    if (!el) return;
    el.classList.remove('sync');
    void el.offsetWidth;
    el.classList.add('sync');
  });
}

/* ---------------- 飞牌 ---------------- */

export function flyCard(fromRect, toRect, card, opts = {}) {
  if (!fromRect || !toRect) return;

  const w = opts.w || 68;
  const h = w * 1.42;

  const el = document.createElement('div');
  el.className = 'card flying';
  if (card) {
    el.dataset.tone = tone(card);
    el.innerHTML = cardFace(card);
  } else {
    el.innerHTML = '<div class="face back"></div>';
  }
  el.style.width = w + 'px';
  el.style.height = h + 'px';

  const cx0 = fromRect.left + fromRect.width / 2;
  const cy0 = fromRect.top + fromRect.height / 2;
  const cx1 = toRect.left + toRect.width / 2;
  const cy1 = toRect.top + toRect.height / 2;

  el.style.left = cx0 - w / 2 + 'px';
  el.style.top = cy0 - h / 2 + 'px';
  $('fx').appendChild(el);

  const dx = cx1 - cx0;
  const dy = cy1 - cy0;
  const dist = Math.hypot(dx, dy);
  const dur = opts.duration || 700;
  // 上抛高度跟着距离走：固定 66px 会让近距离的飞行看着像飘、远距离的像贴地滑
  const lift = clamp(22 + dist * 0.17, 26, 98);
  const spin = (dx < 0 ? -9 : 9) + (Math.random() * 8 - 4);

  const anim = el.animate(
    [
      {
        transform: 'perspective(820px) translate(0,0) scale(.42) rotateZ(-18deg) rotateY(-26deg)',
        opacity: 0,
        offset: 0,
      },
      {
        transform: `perspective(820px) translate(${dx * 0.06}px, ${dy * 0.06 - 10}px) scale(.96) rotateZ(${spin * 0.5}deg) rotateY(-13deg)`,
        opacity: 1,
        offset: 0.16,
      },
      {
        transform: `perspective(820px) translate(${dx * 0.46}px, ${dy * 0.46 - lift}px) scale(1.24) rotateZ(${spin}deg) rotateY(9deg)`,
        opacity: 1,
        offset: 0.54,
      },
      {
        transform: `perspective(820px) translate(${dx}px, ${dy}px) scale(1.06) rotateZ(${spin * 0.3}deg) rotateY(0deg)`,
        opacity: 1,
        offset: 0.87,
      },
      {
        transform: `perspective(820px) translate(${dx}px, ${dy + 4}px) scale(.93) rotateZ(0deg) rotateY(0deg)`,
        opacity: 0,
        offset: 1,
      },
    ],
    { duration: dur, easing: 'cubic-bezier(.32,.72,.3,1)', fill: 'forwards' }
  );

  let landed = false;
  const land = () => {
    if (landed) return;
    landed = true;
    if (el.parentNode) el.remove();
    if (opts.onLand) opts.onLand();
  };
  anim.onfinish = land;
  setTimeout(land, dur + 320);
}

function flySwap(t1, t2) {
  const a = cardRect(t1.player, t1.slot);
  const b = cardRect(t2.player, t2.slot);
  if (!a || !b) return;
  flyCard(a, b, null, { duration: 720, w: 60 });
  setTimeout(() => flyCard(b, a, null, { duration: 720, w: 60 }), 90);
}

// 牌堆摸空、弃牌堆洗回来：整叠牌被抓回牌堆下方，搓几下再落定
function runReshuffle(e) {
  snd('shuffle');

  const from = discardRect();
  const to = deckRect();
  const n = clamp(Math.ceil((e.value || 6) / 7), 3, 6);
  if (from && to) {
    for (let i = 0; i < n; i++) {
      setTimeout(() => flyCard(from, to, null, { duration: 300, w: 54 }), i * 78);
    }
  }

  const at = n * 78 + 200;
  setTimeout(() => {
    const stack = document.querySelector('.deck-stack');
    if (stack) {
      stack.classList.remove('riffle', 'bump');
      void stack.offsetWidth;
      stack.classList.add('riffle');
    }
    if (!to) return;
    const tag = document.createElement('div');
    tag.className = 'reshuffle-tag';
    tag.textContent = '弃牌堆洗回牌堆';
    tag.style.left = to.left + to.width / 2 + 'px';
    tag.style.top = to.top - 14 + 'px';
    $('fx').appendChild(tag);
    setTimeout(() => tag.remove(), 1500);
  }, at);
}

/* ---------------- 事件驱动动画 ---------------- */

// FLIGHT_DUR 是每个事件占用动画通道的"排程预算"（决定下一个事件何时开始），
// 不是单张牌的飞行动画时长——后者在 runFlight/flySwap 内部各自定义。
// 关系是"预算 >= 实际动画"：swap 预算 900ms 覆盖两张错开 90ms、各 720ms 的飞行，
// reshuffle 620ms 同理。所以这里的数字和动画时长故意不同，改它只调事件间隔、不改手感。
const FLIGHT_DUR = { draw: 700, reshuffle: 620, discard: 700, place: 700, take: 700, swap: 900, cabo: 900 };

function runFlight(e) {
  switch (e.kind) {
    case 'draw':
      if (e.player !== 0) {
        snd('draw');
        flyCard(deckRect(), seatHandRect(e.player), null, {
          duration: 700,
          onLand: () => {
            pulseSeat(e.player);
            snd('receive');
          },
        });
      }
      break;
    case 'reshuffle':
      runReshuffle(e);
      break;
    case 'discard':
    case 'place':
      if (e.card) {
        snd(e.kind);
        flyCard(seatHandRect(e.player), discardRect(), e.card, {
          duration: 700,
          onLand: () => {
            pulseDiscard();
            snd('land');
          },
        });
      }
      break;
    case 'take':
      if (e.card) {
        snd('take');
        flyCard(discardRect(), seatHandRect(e.player), e.card, {
          duration: 700,
          onLand: () => {
            pulseSeat(e.player);
            snd('receive');
          },
        });
      }
      break;
    case 'swap':
      if (e.target && e.target2) {
        snd('swap');
        flySwap(e.target, e.target2);
      }
      break;
    case 'cabo':
      showCaboBanner(e.player);
      snd('cabo');
      break;
    default:
      break;
  }
}

// 用旧画面播完动画，再切到新状态——"牌飞过去"和"牌落到堆里"才是先后关系
export function playFlights(events) {
  const list = (events || []).filter((e) => FLIGHT_DUR[e.kind]);
  if (!list.length) return Promise.resolve();

  let t = 0;
  let total = 0;
  for (const e of list) {
    const at = t;
    const dur = FLIGHT_DUR[e.kind];
    setTimeout(() => runFlight(e), at);
    t = at + dur + 90;
    total = at + dur;
  }
  return new Promise((r) => setTimeout(r, total + 40));
}

// 喊 CABO：轻量水玻璃 callout，浮在喊话者座位旁边
export function showCaboBanner(pid) {
  const who = typeof pid === 'number' ? pid : 0;
  const r = panelRect(who);
  if (!r) return;

  const el = document.createElement('div');
  el.className = 'cabo-callout';
  el.textContent = 'CABO';
  $('fx').appendChild(el);

  // 以座位中心为锚，左右各夹一点，避免字太长时贴出视口
  const w = el.offsetWidth;
  const cx = r.left + r.width / 2;
  el.style.left = clamp(cx, w / 2 + 8, innerWidth - w / 2 - 8) + 'px';
  const centerY = who === 0 ? r.top - el.offsetHeight / 2 - 42 : r.top + r.height / 2;
  el.style.top = centerY + 'px';

  setTimeout(() => el.remove(), 1900);
}

function anchor(pid) {
  if (pid === 0) {
    const r = $('meBar').getBoundingClientRect();
    return { x: r.left + r.width / 2, y: r.top - 4 };
  }
  const el = state.seatEls.get(pid);
  if (!el) return { x: innerWidth / 2, y: innerHeight / 2 };
  const r = el.getBoundingClientRect();
  return { x: r.left + r.width / 2, y: r.top - 6 };
}

function pill(pid, text, color) {
  const a = anchor(pid);
  const d = document.createElement('div');
  d.className = 'pill';
  d.style.left = a.x + 'px';
  d.style.top = a.y + 'px';
  if (color) d.style.color = color;
  d.textContent = text;
  $('fx').appendChild(d);
  setTimeout(() => d.remove(), 1800);
}

// 技能发动提示：贴名条的胶囊（图标 + 名字 + 作用对象）。
// 回合中的普通动作不该用大号裸字——那是 CABO 那种大招级事件的语汇，
// 技能色只出现在图标底色和 1px 描边上，名字保持正文墨色。
const SKILL_ICON = {
  peek: '<svg viewBox="0 0 14 14"><path d="M1 7s2.2-3 6-3 6 3 6 3-2.2 3-6 3-6-3-6-3z"/><circle cx="7" cy="7" r="1.5"/></svg>',
  spy: '<svg viewBox="0 0 14 14"><circle cx="6" cy="6" r="3.4"/><path d="M8.6 8.6 12 12"/></svg>',
  swap: '<svg viewBox="0 0 14 14"><path d="M2 5h8.5l-2-2M12 9H3.5l2 2"/></svg>',
};

let skillChipEl = null;

function panelRect(pid) {
  const el = pid === 0 ? $('meBar') : state.seatEls.get(pid);
  return el ? el.getBoundingClientRect() : null;
}

function skillChip(pid, label, sub, tone) {
  // 连发时替换掉旧的，不要叠一屏
  if (skillChipEl) skillChipEl.remove();
  const r = panelRect(pid);
  if (!r) return;

  const el = document.createElement('div');
  el.className = 'skill-chip';
  el.dataset.tone = tone;
  el.innerHTML = `<span class="sk-ico">${SKILL_ICON[tone] || ''}</span><span class="sk-txt"><b>${label}</b><i>${sub}</i></span>`;
  el.style.left = r.right - 2 + 'px';
  el.style.top = r.top + 16 + 'px';
  $('fx').appendChild(el);

  // 右侧放不下时：左边真有位置才翻过去，否则贴住视口右缘（宁可盖住自己的分数，也别压隔壁玩家的名字）
  const w = el.offsetWidth;
  if (r.right - 2 + w > innerWidth - 8) {
    const leftPos = r.left + 2 - w;
    el.style.left = (leftPos >= 8 ? leftPos : Math.max(8, innerWidth - w - 8)) + 'px';
  }

  skillChipEl = el;
  setTimeout(() => {
    el.remove();
    if (skillChipEl === el) skillChipEl = null;
  }, 1560);
}

function hitCard(pid, slot) {
  const el = document.querySelector(`.card[data-ref="${pid}:${slot}"]`);
  if (!el) return;
  el.classList.remove('hit');
  void el.offsetWidth;
  el.classList.add('hit');
}

function pulseDiscard() {
  const el = document.querySelector('.discard-holder');
  if (!el) return;
  el.classList.remove('landing');
  void el.offsetWidth;
  el.classList.add('landing');
}

export function pulseSeat(pid) {
  const el = state.seatEls.get(pid);
  if (!el) return;
  el.classList.remove('receive');
  void el.offsetWidth;
  el.classList.add('receive');
}

export function playEvents(events, opts = {}) {
  if (!events || !events.length) return;
  let delay = 0;
  for (const e of events) {
    const d = delay;
    setTimeout(() => playOne(e, opts), d);
    delay += e.kind === 'cabo' ? 0 : 110;
  }
}

// flown：飞牌路径已经配过动作音了（AI 回合），这里只补胶囊提示，不重复发声。
// 玩家自己的动作走同步路径、没有飞牌，动作音由这里发。
function playOne(e, opts = {}) {
  const flown = !!opts.flown;
  switch (e.kind) {
    case 'cabo':
      if (!flown) {
        showCaboBanner(e.player);
        snd('cabo');
      }
      break;
    case 'draw':
      if (!flown) snd('draw');
      pill(e.player, '摸了一张牌');
      break;
    case 'reshuffle':
      if (!flown) runReshuffle(e);
      break;
    case 'discard':
      if (!flown) {
        snd('discard');
        setTimeout(() => snd('land'), 300);
      }
      pill(e.player, e.card && e.card.label ? `弃掉 ${e.card.label}` : '弃掉一张');
      break;
    case 'place':
      if (!flown) {
        snd('place');
        setTimeout(() => snd('land'), 300);
      }
      pill(e.player, e.card && e.card.label ? `换掉 ${e.card.label}` : '换了一张');
      break;
    case 'take': {
      if (!flown) {
        snd('take');
        setTimeout(() => snd('receive'), 340);
      }
      const lbl = e.card && e.card.label ? e.card.label : '一张牌';
      pill(e.player, `换走 ${lbl}${e.value > 1 ? ` · 出 ${e.value} 张` : ''}`, '#3fbfb4');
      break;
    }
    case 'penalty':
      snd('penalty');
      pill(e.player, '记错了！收下这张', '#f0a5a5');
      break;
    case 'reset':
      snd('score');
      pill(e.player, '正好 100 分，重置为 50', '#e9b44c');
      break;
    case 'peek':
      snd('peek');
      if (e.target) hitCard(e.target.player, e.target.slot);
      skillChip(e.player, '透视', '自己 · 1 张', 'peek');
      break;
    case 'spy':
      snd('spy');
      if (e.target) hitCard(e.target.player, e.target.slot);
      skillChip(e.player, '窥探', '对手 · 1 张', 'spy');
      break;
    case 'swap':
      if (!flown) snd('swap');
      if (e.target) hitCard(e.target.player, e.target.slot);
      if (e.target2) hitCard(e.target2.player, e.target2.slot);
      skillChip(e.player, '置换', '场上 · 2 张', 'swap');
      break;
    default:
      break;
  }
}
