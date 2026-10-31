const $ = (id) => document.getElementById(id);

let view = null;
let mode = 'idle';
let picked = [];
let swapPicks = [];
let busy = false;
let animating = false;
let aiRunning = false;
let aiCount = 2;

let prevCards = new Map();
let prevDiscardId = null;
let prevDeckCount = null;
let prevScores = [];
let seatEls = new Map();

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

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
    if (anim === 'enter') el.style.animationDelay = idx * 45 + 'ms';
  }

  prevCards.set(posKey, { visible: !!card, id: card ? card.id : 0 });
  return el;
}

/* ---------------- 渲染 ---------------- */

function render() {
  if (!view) return;
  $('roundLabel').textContent = '第 ' + view.round + ' 轮';
  renderTurnChip();
  renderSeats();
  renderMiddle();
  renderMe();
  renderActions();
  renderOverlay();
}

function renderTurnChip() {
  const el = $('turnChip');
  if (!view || view.gameOver || view.phase === 'roundEnd') {
    el.textContent = '';
    el.className = 'chip turn-chip';
    return;
  }
  const t = view.players[view.turn];
  if (!t) {
    el.textContent = '';
    return;
  }
  el.className = 'chip turn-chip p' + view.turn;
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
  bar.innerHTML =
    `<span class="avatar p0">${esc(initial(p.name))}</span>` +
    `<span>${esc(p.name)}</span>` +
    `<b>${p.score} 分</b>` +
    `<span class="me-progress"><i style="width:${pct(p.score)}%"></i></span>` +
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
    if (at >= 0) picked.splice(at, 1);
    else if (picked.length < (mode === 'peek' ? 2 : 3)) picked.push(slot);
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
    if (at >= 0) swapPicks.splice(at, 1);
    else if (swapPicks.length < 2) swapPicks.push({ player, slot });
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

function toast(msg) {
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
  const dur = opts.duration || 700;

  const anim = el.animate(
    [
      { transform: 'translate(0,0) scale(.45) rotate(-16deg)', opacity: 0, offset: 0 },
      {
        transform: `translate(${dx * 0.05}px, ${dy * 0.05 - 8}px) scale(.94) rotate(-10deg)`,
        opacity: 1,
        offset: 0.14,
      },
      {
        transform: `translate(${dx * 0.45}px, ${dy * 0.45 - 66}px) scale(1.22) rotate(3deg)`,
        opacity: 1,
        offset: 0.56,
      },
      {
        transform: `translate(${dx}px, ${dy}px) scale(1.04) rotate(7deg)`,
        opacity: 1,
        offset: 0.86,
      },
      {
        transform: `translate(${dx}px, ${dy}px) scale(.9) rotate(5deg)`,
        opacity: 0,
        offset: 1,
      },
    ],
    { duration: dur, easing: 'cubic-bezier(.3,.8,.35,1)', fill: 'forwards' }
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
        flyCard(deckRect(), seatHandRect(e.player), null, { duration: 700, onLand: () => pulseSeat(e.player) });
      }
      break;
    case 'discard':
    case 'place':
      if (e.card) {
        flyCard(seatHandRect(e.player), discardRect(), e.card, { duration: 700, onLand: pulseDiscard });
      }
      break;
    case 'take':
      if (e.card) {
        flyCard(discardRect(), seatHandRect(e.player), e.card, { duration: 700, onLand: () => pulseSeat(e.player) });
      }
      break;
    case 'swap':
      if (e.target && e.target2) flySwap(e.target, e.target2);
      break;
    case 'cabo':
      showCaboBanner();
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

function playOne(e, opts = {}) {
  switch (e.kind) {
    case 'cabo':
      if (!opts.skipCabo) showCaboBanner();
      pill(e.player, e.player === 0 ? '你喊了 CABO' : '对手喊了 CABO', '#e9b44c');
      break;
    case 'draw':
      pill(e.player, '摸了一张牌');
      break;
    case 'discard':
      pill(e.player, e.card && e.card.label ? `弃掉 ${e.card.label}` : '弃掉一张');
      break;
    case 'place':
      pill(e.player, e.card && e.card.label ? `换掉 ${e.card.label}` : '换了一张');
      break;
    case 'take': {
      const lbl = e.card && e.card.label ? e.card.label : '一张牌';
      pill(e.player, `换走 ${lbl}${e.value > 1 ? ` · 出 ${e.value} 张` : ''}`, '#3fbfb4');
      break;
    }
    case 'penalty':
      pill(e.player, '记错了！收下这张', '#f0a5a5');
      break;
    case 'peek':
      if (e.target) hitCard(e.target.player, e.target.slot);
      pill(e.player, '透视', '#3fbfb4');
      break;
    case 'spy':
      if (e.target) hitCard(e.target.player, e.target.slot);
      pill(e.player, '窥探', '#9b7bf0');
      break;
    case 'swap':
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
  ov.classList.toggle('hidden', !show);
  if (!show) return;
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
  panel.querySelectorAll('.score-row').forEach((row) => {
    const t = row.querySelector('.total');
    const from = Number(t.dataset.from);
    const to = Number(t.dataset.to);
    t.textContent = to;
    if (!reduceMotion && from !== to) countUp(t, from, to, 720);
  });
}

/* ---------------- 主流程 ---------------- */

function setThinking(pid) {
  seatEls.forEach((el, i) => el.classList.toggle('thinking', i === pid));
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
  setView(v, { skipCabo: true });
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
  const r = await api('/api/new', { name, ai: aiCount });
  if (!r.ok) { toast(r.error || '开局失败'); return; }
  $('startOverlay').classList.add('hidden');
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
}

$('aiPicker').addEventListener('click', (e) => {
  const b = e.target.closest('button[data-ai]');
  if (!b) return;
  aiCount = Number(b.dataset.ai);
  [...$('aiPicker').children].forEach((x) => x.classList.toggle('on', x === b));
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
