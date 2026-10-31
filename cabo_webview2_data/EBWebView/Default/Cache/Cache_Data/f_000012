const $ = (id) => document.getElementById(id);

let view = null;
let mode = 'idle';
let picked = [];
let swapPicks = [];
let busy = false;
let animating = false;
let aiRunning = false;
let aiCount = 2;
let aiLevel = 1;

let prevCards = new Map();
let prevDiscardId = null;
let prevDeckCount = null;
let prevScores = [];
let seatEls = new Map();
let enterCount = 0;
let revealCount = 0;

// audio.js 提供 window.snd；音频起不来时静默降级，不能影响牌局
const snd = (...a) => window.snd && window.snd(...a);
const sndMode = (m) => window.sndMode && window.sndMode(m);

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));
const reduceMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

function esc(s) {
  return String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

async function api(path, body) {
  const res = await fetch(path, {
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return res.json();
}

function tone(card) {
  if (!card) return null;
  if (card.points === 0) return 'zero';
  if (card.power) return card.power;
  if (card.points >= 13) return 'high';
  return 'low';
}

function initial(name) {
  const s = (name || '').trim();
  return s ? s[s.length - 1] : '?';
}

function pct(score) {
  return Math.min(100, Math.max(0, score));
}

function isOwnPickable() {
  return !animating && ['peek', 'claim', 'place', 'peekSelf', 'swap'].includes(mode);
}
function isOppPickable() {
  return !animating && (mode === 'spy' || mode === 'swap');
}

/* ---------------- 卡牌 ---------------- */

function cardFace(card) {
  return (
    '<div class="face front">' +
    `<span class="rank">${esc(card.label)}</span>` +
    (card.power ? `<span class="pw">${esc(card.powerZh)}</span>` : '') +
    '</div>'
  );
}

function makeCard(pid, idx, slot, opts = {}) {
  const el = document.createElement('div');
  el.className = 'card';
  if (opts.small) el.classList.add('small');

  const posKey = pid + ':' + idx;
  el.dataset.ref = posKey;

  const prev = prevCards.get(posKey);
  const card = slot && slot.visible ? slot.card : null;

  let anim = null;
  if (!prev) anim = 'enter';
  else if (!prev.visible && card) anim = 'reveal';
  else if (card && prev.id !== card.id) anim = 'reveal';

  if (card) {
    el.dataset.tone = tone(card);
    el.innerHTML = cardFace(card);
    if (slot.faceUp) {
      const tag = document.createElement('span');
      tag.className = 'pub-tag';
      tag.title = '这张牌对所有人公开';
      el.appendChild(tag);
    }
  } else {
    el.innerHTML = '<div class="face back"></div>';
  }

  if (opts.picked) el.classList.add('picked');
  if (opts.clickable) {
    el.classList.add('clickable');
    el.addEventListener('click', opts.onClick);
  }
  if (anim) {
    el.classList.add(anim);
    if (anim === 'enter') {
      el.style.animationDelay = idx * 62 + 'ms';
      enterCount++;
    } else if (anim === 'reveal') revealCount++;
  }

  prevCards.set(posKey, { visible: !!card, id: card ? card.id : 0 });
  return el;
}

/* ---------------- 渲染 ---------------- */

function render() {
  if (!view) return;
  enterCount = 0;
  revealCount = 0;
  $('roundLabel').textContent = '第 ' + view.round + ' 轮';
  renderTurnChip();
  renderSeats();
  renderMiddle();
  renderMe();
  renderActions();
  renderOverlay();
  // 一次渲染里进来 6 张以上就当作发牌，否则只是手牌张数变化的落位
  if (enterCount >= 6) snd('deal', enterCount);
  else if (enterCount > 0) snd('settle', enterCount);
  for (let i = 0; i < revealCount; i++) setTimeout(() => snd('flip'), i * 95);
}

function renderTurnChip() {
  const el = $('turnChip');
  if (!view || view.gameOver || view.phase === 'roundEnd') {
    el.textContent = '';
    el.className = 'chip turn-chip';
    el.dataset.turn = '';
    return;
  }
  const t = view.players[view.turn];
  if (!t) {
    el.textContent = '';
    return;
  }
  // 只在真的换人时挂 swap，否则每次 render 重写 class 会把还没播完的动画掐掉
  const changed = el.dataset.turn !== String(view.turn);
  el.className = 'chip turn-chip p' + view.turn + (changed ? ' swap' : '');
  el.dataset.turn = String(view.turn);
  el.innerHTML = `<span class="dot"></span>轮到 ${esc(t.name)}`;
}

function renderSeats() {
  const wrap = $('seats');
  wrap.innerHTML = '';
  seatEls = new Map();

  view.players.forEach((p, i) => {
    if (i === 0) return;
    const seat = document.createElement('div');
    seat.className = 'seat p' + i;
    if (p.isTurn) seat.classList.add('active');
    if (p.isCaller) seat.classList.add('caller');

    const head = document.createElement('div');
    head.className = 'seat-head';
    head.innerHTML =
      '<span class="seat-id">' +
      `<span class="avatar">${esc(initial(p.name))}</span>` +
      `<span class="seat-name">${p.isTurn ? '<span class="dot"></span>' : ''}${esc(p.name)}` +
      '<span class="think"><i></i><i></i><i></i></span></span>' +
      '</span>' +
      `<span class="seat-cards${p.handCount === 4 ? '' : ' odd'}" title="手里 ${p.handCount} 张牌">${p.handCount}</span>` +
      `<span class="seat-score">${p.score}</span>`;
    seat.appendChild(head);

    const prog = document.createElement('div');
    prog.className = 'seat-progress';
    prog.innerHTML = `<i style="width:${pct(p.score)}%"></i>`;
    seat.appendChild(prog);

    const hand = document.createElement('div');
    hand.className = 'seat-hand';
    p.hand.forEach((slot, si) => {
      hand.appendChild(
        makeCard(i, si, slot, {
          small: true,
          clickable: isOppPickable(),
          picked: swapPicks.some((t) => t.player === i && t.slot === si),
          onClick: () => pickCard(i, si),
        })
      );
    });
    seat.appendChild(hand);

    seatEls.set(i, seat);
    wrap.appendChild(seat);
  });
}

function renderMiddle() {
  $('deckCount').textContent = view.deckCount;
  $('discardCount').textContent = view.discardCount;

  if (prevDeckCount !== null && prevDeckCount !== view.deckCount) {
    const stack = document.querySelector('.deck-stack');
    if (stack) {
      stack.classList.remove('bump');
      void stack.offsetWidth;
      stack.classList.add('bump');
    }
  }
  prevDeckCount = view.deckCount;

  const holder = document.querySelector('.discard-holder');
  if (holder) {
    holder.innerHTML = '';
    if (view.discardTop) {
      const el = document.createElement('div');
      el.className = 'card';
      el.dataset.tone = tone(view.discardTop);
      el.innerHTML = cardFace(view.discardTop);
      if (prevDiscardId !== view.discardTop.id) el.classList.add('dropIn');
      holder.appendChild(el);
      prevDiscardId = view.discardTop.id;
    } else {
      prevDiscardId = null;
    }
  }

  const slot = $('pendingSlot');
  slot.innerHTML = '';
  if (view.pending) {
    const hint = document.createElement('div');
    hint.className = 'pending-hint';
    hint.textContent = '摸到了';
    slot.appendChild(hint);
    const el = document.createElement('div');
    el.className = 'card enter';
    el.dataset.tone = tone(view.pending);
    el.innerHTML = cardFace(view.pending);
    slot.appendChild(el);
  }
}

function renderMe() {
  const p = view.players[0];
  const bar = $('meBar');
  let badges = '';
  if (p.isTurn && !view.gameOver && view.phase !== 'roundEnd') {
    badges += '<span class="turn-badge">轮到你</span>';
  }
  if (p.isCaller) {
    badges += '<span class="turn-badge cabo">已喊 CABO</span>';
  }
  // 喊 CABO 是全场最大的决定，给一个随时可见的估值，别让玩家靠心算
  let knownSum = 0;
  let unknown = 0;
  p.hand.forEach((sl) => {
    if (sl.visible && sl.card) knownSum += sl.card.points;
    else unknown++;
  });
  const est =
    !view.gameOver && view.phase !== 'roundEnd'
      ? `<span class="me-est" title="已知的牌共 ${knownSum} 点，还有 ${unknown} 张没看过">已知 ${knownSum} 点 · 未知 ${unknown} 张</span>`
      : '';
  bar.innerHTML =
    `<span class="avatar p0">${esc(initial(p.name))}</span>` +
    `<span>${esc(p.name)}</span>` +
    `<b>${p.score} 分</b>` +
    `<span class="me-progress"><i style="width:${pct(p.score)}%"></i></span>` +
    est +
    badges;

  const hand = $('myHand');
  hand.innerHTML = '';
  p.hand.forEach((slot, i) => {
    hand.appendChild(
      makeCard(0, i, slot, {
        clickable: isOwnPickable(),
        picked: picked.includes(i) || swapPicks.some((t) => t.player === 0 && t.slot === i),
        onClick: () => pickCard(0, i),
      })
    );
  });
}

function renderActions() {
  const bar = $('actions');
  bar.innerHTML = '';
  if (!view) return;

  const add = (label, cls, fn, disabled) => {
    const b = document.createElement('button');
    b.className = 'btn' + (cls ? ' ' + cls : '');
    b.textContent = label;
    b.disabled = !!disabled || animating;
    b.addEventListener('click', fn);
    bar.appendChild(b);
  };
  const hint = (t) => {
    const s = document.createElement('div');
    s.className = 'hint';
    s.textContent = t;
    bar.appendChild(s);
  };

  if (view.gameOver || view.phase === 'roundEnd') return;

  if (mode === 'peek') {
    hint(`选 2 张自己的牌偷看（${picked.length}/2）`);
    add('确认', 'primary', doPeek, picked.length !== 2);
    return;
  }
  if (mode === 'claim') {
    hint(`选 1–3 张手牌换出（${picked.length}）`);
    add('确认换牌', 'primary', doClaim, picked.length < 1);
    add('取消', '', () => { mode = 'idle'; picked = []; render(); });
    return;
  }
  if (mode === 'place') {
    hint('点一张手牌，把它换掉');
    add('取消', '', () => { mode = 'idle'; picked = []; render(); });
    return;
  }
  if (mode === 'peekSelf') {
    hint('点自己一张牌偷看');
    add('取消', '', () => { mode = 'idle'; render(); });
    return;
  }
  if (mode === 'spy') {
    hint('点对手的一张牌偷看');
    add('取消', '', () => { mode = 'idle'; render(); });
    return;
  }
  if (mode === 'swap') {
    hint(`点两张牌交换（${swapPicks.length}/2）`);
    add('确认交换', 'primary', doSwap, swapPicks.length !== 2);
    add('取消', '', () => { mode = 'idle'; swapPicks = []; render(); });
    return;
  }

  const c = view.can;

  if (view.phase === 'peek' && c.peek) {
    hint('先看自己 2 张牌，记住它们的位置');
    add('看牌', 'primary', () => { mode = 'peek'; picked = []; render(); });
    return;
  }

  if (c.discardPending) {
    add('弃掉', '', () => act({ type: 'discardPending' }));
    add('换入手牌', 'primary', () => { mode = 'place'; picked = []; render(); });
    if (c.usePower) add('发动技能', '', startPower);
    return;
  }

  if (c.drawDeck) {
    add('摸牌堆', 'primary', () => act({ type: 'drawDeck' }));
    add('拿弃牌堆', '', () => { mode = 'claim'; picked = []; render(); }, !c.takeDiscard);
    add('喊 CABO', 'danger', callCabo, !c.callCabo);
    // 灰掉的按钮要说清楚为什么，别让玩家猜
    if (!c.takeDiscard) hint('弃牌堆还是空的，这回合只能摸牌堆');
    else if (!c.callCabo) hint('已经有人喊过 CABO，本轮不能再喊');
    return;
  }

  const t = view.players[view.turn];
  hint(t ? `等待 ${t.name} 行动…` : '');
}

/* ---------------- 交互 ---------------- */

function pickCard(player, slot) {
  if (busy || animating || !view) return;

  if (mode === 'peek' || mode === 'claim') {
    if (player !== 0) return;
    const at = picked.indexOf(slot);
    if (at >= 0) {
      picked.splice(at, 1);
      snd('deselect');
    } else if (picked.length < (mode === 'peek' ? 2 : 3)) {
      picked.push(slot);
      snd('select', picked.length - 1);
    } else {
      snd('error');
      return;
    }
    render();
    return;
  }
  if (mode === 'place') {
    if (player !== 0) return;
    picked = [slot];
    doPlace();
    return;
  }
  if (mode === 'peekSelf') {
    if (player !== 0) return;
    act({ type: 'usePower', targets: [{ player: 0, slot }] });
    return;
  }
  if (mode === 'spy') {
    if (player === 0) return;
    act({ type: 'usePower', targets: [{ player, slot }] });
    return;
  }
  if (mode === 'swap') {
    const at = swapPicks.findIndex((t) => t.player === player && t.slot === slot);
    if (at >= 0) {
      swapPicks.splice(at, 1);
      snd('deselect');
    } else if (swapPicks.length < 2) {
      swapPicks.push({ player, slot });
      snd('select', swapPicks.length - 1);
    } else {
      snd('error');
      return;
    }
    render();
    return;
  }
}

function startPower() {
  const pw = view.pending && view.pending.power;
  if (pw === 'peek') { mode = 'peekSelf'; render(); }
  else if (pw === 'spy') { mode = 'spy'; render(); }
  else if (pw === 'swap') { mode = 'swap'; swapPicks = []; render(); }
}

function doPeek() { act({ type: 'peek', indices: picked.slice() }); }
function doClaim() { act({ type: 'takeDiscard', indices: picked.slice() }); }
function doPlace() { act({ type: 'place', indices: picked.slice() }); }
function doSwap() { act({ type: 'usePower', targets: swapPicks.slice() }); }
function callCabo() { act({ type: 'callCabo' }); }

async function act(body) {
  if (busy || animating) return;
  busy = true;
  const r = await api('/api/action', body);
  busy = false;
  if (!r.ok) { toast(r.error); return; }
  applyView(r.view);
}

async function nextRound() {
  if (busy) return;
  busy = true;
  const r = await api('/api/next', {});
  busy = false;
  if (!r.ok) { toast(r.error); return; }
  applyView(r.view);
}

// .hidden 是 display:none，退场动画必须先播完再挂上，否则直接被砍掉
function hideOverlay(el) {
  if (el.classList.contains('hidden')) return;
  if (reduceMotion()) {
    el.classList.add('hidden');
    return;
  }
  el.classList.add('leaving');
  setTimeout(() => {
    el.classList.remove('leaving');
    el.classList.add('hidden');
  }, 330);
}

function toast(msg) {
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
  const seat = seatEls.get(pid);
  if (!seat) return null;
  const h = seat.querySelector('.seat-hand');
  return (h || seat).getBoundingClientRect();
}

function cardRect(pid, slot) {
  const el = document.querySelector(`.card[data-ref="${pid}:${slot}"]`);
  return el ? el.getBoundingClientRect() : seatHandRect(pid);
}

/* ---------------- 飞牌 ---------------- */

function flyCard(fromRect, toRect, card, opts = {}) {
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

/* ---------------- 事件驱动动画 ---------------- */

const FLIGHT_DUR = { draw: 700, discard: 700, place: 700, take: 700, swap: 900, cabo: 900 };

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
      showCaboBanner();
      snd('cabo');
      break;
    default:
      break;
  }
}

// 用旧画面播完动画，再切到新状态——"牌飞过去"和"牌落到堆里"才是先后关系
function playFlights(events) {
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

function showCaboBanner() {
  const b = document.createElement('div');
  b.className = 'cabo-banner';
  b.textContent = 'CABO';
  $('fx').appendChild(b);
  const f = document.createElement('div');
  f.className = 'flash';
  $('fx').appendChild(f);
  setTimeout(() => { b.remove(); f.remove(); }, 2100);
}

function anchor(pid) {
  if (pid === 0) {
    const r = $('meBar').getBoundingClientRect();
    return { x: r.left + r.width / 2, y: r.top - 4 };
  }
  const el = seatEls.get(pid);
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

function pulseSeat(pid) {
  const el = seatEls.get(pid);
  if (!el) return;
  el.classList.remove('receive');
  void el.offsetWidth;
  el.classList.add('receive');
}

function playEvents(events, opts = {}) {
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
        showCaboBanner();
        snd('cabo');
      }
      pill(e.player, e.player === 0 ? '你喊了 CABO' : '对手喊了 CABO', '#e9b44c');
      break;
    case 'draw':
      if (!flown) snd('draw');
      pill(e.player, '摸了一张牌');
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
      pill(e.player, '透视', '#3fbfb4');
      break;
    case 'spy':
      snd('spy');
      if (e.target) hitCard(e.target.player, e.target.slot);
      pill(e.player, '窥探', '#9b7bf0');
      break;
    case 'swap':
      if (!flown) snd('swap');
      if (e.target) hitCard(e.target.player, e.target.slot);
      if (e.target2) hitCard(e.target2.player, e.target2.slot);
      pill(e.player, '置换', '#ee8148');
      break;
    default:
      break;
  }
}

/* ---------------- 结算面板 ---------------- */

function countUp(el, from, to, dur) {
  let t0 = null;
  const step = (now) => {
    if (t0 === null) t0 = now;
    const k = Math.max(0, Math.min(1, (now - t0) / dur));
    const eased = 1 - Math.pow(1 - k, 3);
    el.textContent = Math.round(from + (to - from) * eased);
    if (k < 1) requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

function renderOverlay() {
  const ov = $('roundOverlay');
  const show = view.phase === 'roundEnd' || view.gameOver;
  if (!show) {
    hideOverlay(ov);
    return;
  }
  ov.classList.remove('hidden');
  if (ov.dataset.round === String(view.round) && ov.dataset.over === String(view.gameOver)) return;
  ov.dataset.round = String(view.round);
  ov.dataset.over = String(view.gameOver);

  const panel = document.createElement('div');
  panel.className = 'panel';

  let html = '';
  if (view.gameOver) {
    html += `<div class="result-title">${esc(view.players[view.winner].name)} 获胜</div>`;
    html += '<div class="result-sub">总分最低</div>';
  } else {
    html += `<div class="result-title">第 ${view.round} 轮结算</div>`;
    const who = view.caller >= 0 ? `${esc(view.players[view.caller].name)} 喊了 CABO` : '牌堆摸完了';
    html += `<div class="result-sub">${who}</div>`;
  }
  html += '<div class="score-list">';
  view.players.forEach((p, i) => {
    const from = prevScores[i] === undefined ? 0 : prevScores[i];
    const pct = Math.min(100, (p.score / 100) * 100);
    html += `<div class="score-row ${i === 0 ? 'me' : ''}" style="animation-delay:${i * 70}ms">
      <span class="nm">${esc(p.name)}</span>
      <span class="gain">+${p.roundPts}</span>
      <span class="bar"><i style="width:${pct}%;animation-delay:${i * 70 + 120}ms"></i></span>
      <span class="total" data-from="${from}" data-to="${p.score}">${p.score}</span>
    </div>`;
  });
  html += '</div>';
  panel.innerHTML = html;

  const btn = document.createElement('button');
  btn.className = 'primary';
  btn.textContent = view.gameOver ? '再来一局' : '下一轮';
  btn.addEventListener('click', view.gameOver ? restart : nextRound);
  panel.appendChild(btn);

  ov.innerHTML = '';
  ov.appendChild(panel);

  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  panel.querySelectorAll('.score-row').forEach((row, i) => {
    const t = row.querySelector('.total');
    const from = Number(t.dataset.from);
    const to = Number(t.dataset.to);
    t.textContent = to;
    if (!reduceMotion && from !== to) countUp(t, from, to, 720);
    setTimeout(() => snd('tick', i), 480 + i * 90);
  });

  // 让 CABO 的余音先落下去，再报结算
  setTimeout(() => snd(view.gameOver ? (view.winner === 0 ? 'win' : 'lose') : 'score'), 300);
}

/* ---------------- 主流程 ---------------- */

function setThinking(pid) {
  seatEls.forEach((el, i) => el.classList.toggle('thinking', i === pid));
}

// 有人喊了 CABO 或分数逼近 100，配乐就换成更紧的一档；结算/结束时退回稀疏的音乐盒
function musicMode(v) {
  if (!v || v.gameOver || v.phase === 'roundEnd') return 'menu';
  if (v.players.some((p) => p.isCaller) || v.players.some((p) => p.score >= 80)) return 'tension';
  return 'table';
}

function setView(v, opts = {}) {
  prevScores = view ? view.players.map((p) => p.score) : [];
  if (view && view.round !== v.round) {
    prevCards = new Map();
    prevDiscardId = null;
    prevDeckCount = null;
  }
  view = v;
  mode = 'idle';
  picked = [];
  swapPicks = [];
  sndMode(musicMode(v));
  render();
  playEvents(v.events, opts);
}

function applyView(v) {
  setView(v);
  runAI();
}

// AI 的动作：先用旧画面播完飞牌动画，再更新状态
async function applyViewAnimated(v) {
  animating = true;
  try {
    await playFlights(v.events);
  } finally {
    animating = false;
  }
  setView(v, { flown: true });
  for (const e of v.events || []) {
    if (e.kind === 'draw' || e.kind === 'take') pulseSeat(e.player);
  }
}

async function runAI() {
  if (aiRunning) return;
  aiRunning = true;
  try {
    for (let guard = 0; guard < 400; guard++) {
      if (!view || view.gameOver) break;
      if (view.phase !== 'turn' && view.phase !== 'place') break;
      if (view.turn === 0) break;

      const pid = view.turn;
      // 已经摸完牌、正在决定怎么处理时多停一会儿，像在思考
      const wait = view.phase === 'place' ? 1050 : 820;

      setThinking(pid);
      await sleep(wait);

      const r = await api('/api/ai', {});
      setThinking(-1);
      if (!r.ok) { toast(r.error); break; }
      await applyViewAnimated(r.view);
    }
  } finally {
    aiRunning = false;
    setThinking(-1);
  }
}

async function startGame() {
  const name = ($('nameInput').value || '').trim() || '玩家';
  const r = await api('/api/new', { name, ai: aiCount, level: aiLevel });
  if (!r.ok) { toast(r.error || '开局失败'); return; }
  snd('start');
  hideOverlay($('startOverlay'));
  $('app').classList.remove('hidden');
  prevCards = new Map();
  prevDiscardId = null;
  prevDeckCount = null;
  prevScores = [];
  applyView(r.view);
}

function restart() {
  const ov = $('roundOverlay');
  ov.classList.add('hidden');
  ov.dataset.round = '';
  ov.dataset.over = '';
  $('app').classList.add('hidden');
  $('startOverlay').classList.remove('hidden');
  view = null;
  sndMode('menu');
}
// 统一的按钮反馈音；用委托监听，动态生成的操作按钮也能覆盖到
document.addEventListener('click', (e) => {
  if (e.target.closest('button')) snd('ui');
});

$('aiPicker').addEventListener('click', (e) => {
  const b = e.target.closest('button[data-ai]');
  if (!b) return;
  aiCount = Number(b.dataset.ai);
  [...$('aiPicker').children].forEach((x) => x.classList.toggle('on', x === b));
});

$('levelPicker').addEventListener('click', (e) => {
  const b = e.target.closest('button[data-lv]');
  if (!b) return;
  aiLevel = Number(b.dataset.lv);
  [...$('levelPicker').children].forEach((x) => x.classList.toggle('on', x === b));
});

$('btnStart').addEventListener('click', startGame);
$('btnRestart').addEventListener('click', restart);

api('/api/state').then((r) => {
  if (r && r.ready && r.view) {
    $('startOverlay').classList.add('hidden');
    $('app').classList.remove('hidden');
    applyView(r.view);
  }
});
