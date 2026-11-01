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

// 累积式反馈：连击只在「本轮」内累加，跨局的生涯记录存服务端（见 internal/server/stats.go）
let streak = 0;
let roundBestStreak = 0;
let roundEvents = [];
let career = null;
let pendingComboFx = 0;

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

function deckTier(n) {
  if (n > 30) return 'full';
  if (n > 18) return 'mid';
  if (n > 6) return 'low';
  return 'empty';
}

function renderMiddle() {
  $('deckCount').textContent = view.deckCount;
  $('discardCount').textContent = view.discardCount;

  const stack = document.querySelector('.deck-stack');
  if (stack) {
    stack.dataset.tier = deckTier(view.deckCount);
    if (prevDeckCount !== null && prevDeckCount !== view.deckCount) {
      stack.classList.remove('bump', 'riffle');
      void stack.offsetWidth;
      stack.classList.add('bump');
    }
  }
  prevDeckCount = view.deckCount;

  const holder = document.querySelector('.discard-holder');
  if (holder) {
    holder.innerHTML = '';
    if (view.discardUnder && view.discardUnder.length) {
      const fan = document.createElement('div');
      fan.className = 'discard-fan';
      view.discardUnder.forEach((c) => {
        const f = document.createElement('div');
        f.className = 'card small';
        f.dataset.tone = tone(c);
        f.innerHTML = cardFace(c);
        fan.appendChild(f);
      });
      holder.appendChild(fan);
    }
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
  const combo =
    streak >= 2 && !view.gameOver && view.phase !== 'roundEnd'
      ? `<span class="combo-chip" data-tier="${comboTier(streak)}" title="连续 ${streak} 次把自己已知的牌换成更小的">连击 ×${streak}</span>`
      : '';
  bar.innerHTML =
    `<span class="avatar p0">${esc(initial(p.name))}</span>` +
    `<span>${esc(p.name)}</span>` +
    `<b>${p.score} 分</b>` +
    `<span class="me-progress"><i style="width:${pct(p.score)}%"></i></span>` +
    est +
    combo +
    badges;

  if (pendingComboFx) {
    const n = pendingComboFx;
    pendingComboFx = 0;
    const chip = bar.querySelector('.combo-chip');
    if (chip) chip.classList.add('pop');
    snd('combo', n);
    if (n >= 4) comboFlare(n >= 7 ? '手感来了' : '还在涨', n >= 7 ? '#E9A427' : '#3FBFB4', n);
  }

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
  const before = view;
  const r = await api('/api/action', body);
  busy = false;
  if (!r.ok) { toast(r.error); return; }
  judgeStreak(before, body, r.view);
  applyView(r.view);
}

/* ---------------- 连击（累积式反馈） ---------------- */

// 只认「自己验证过的降分」：CABO 的爽点是记住牌、把大牌甩掉，
// 所以看不见面目的安置算中性——既奖励记忆，也不惩罚合理试探。
function judgeStreak(before, body, after) {
  if (!before || !after || !body) return;
  const me = before.players[0];
  if (!me) return;

  // 记错牌（1 换多点数对不上）一律断，这是本轮最实在的失误
  const broke = (after.events || []).some((e) => e.kind === 'penalty' && e.player === 0);
  if (broke) return setStreak(0);

  if (body.type === 'place' && before.pending) {
    const old = me.hand[body.indices[0]];
    if (!old || !old.visible || !old.card) return;
    setStreak(before.pending.points < old.card.points ? streak + 1 : 0);
    return;
  }

  if (body.type === 'takeDiscard' && before.discardTop) {
    const out = body.indices.map((i) => me.hand[i]);
    if (out.some((s) => !s || !s.visible || !s.card)) return;
    const tp = before.discardTop.points;
    const better = out.every((s) => tp < s.card.points);
    setStreak(better ? streak + 1 : 0);
  }
}

function setStreak(n) {
  const prev = streak;
  streak = clamp(n, 0, 99);
  if (streak > roundBestStreak) roundBestStreak = streak;
  if (streak > prev && streak >= 2) pendingComboFx = streak;
  else if (streak < prev && prev >= 3) {
    snd('comboBreak');
    comboFlare('连击断了', '#9b9387', prev);
  }
}

// 连击档位：2 起步、3 变色、5 加描边、7 起才是「手感来了」的金档
function comboTier(n) {
  if (n >= 7) return 'flow';
  if (n >= 5) return 'hot';
  if (n >= 3) return 'warm';
  return 'cool';
}

function comboFlare(text, color, n) {
  const chip = document.querySelector('.combo-chip');
  const base = chip ? chip.getBoundingClientRect() : $('meBar').getBoundingClientRect();
  const el = document.createElement('div');
  el.className = 'combo-flare';
  el.textContent = text;
  if (color) el.style.color = color;
  el.style.top = base.top - 6 + 'px';
  el.dataset.tier = comboTier(n || streak);
  $('fx').appendChild(el);
  // 连击标识贴在名条右端时，爆发字条可能被视口切掉，夹一下
  const w = el.offsetWidth;
  el.style.left = clamp(base.left + base.width / 2, w / 2 + 8, innerWidth - w / 2 - 8) + 'px';
  setTimeout(() => el.remove(), 1100);
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

// 喊 CABO：金色气球字，盖在喊话者座位的正中间
function showCaboBanner(pid) {
  const who = typeof pid === 'number' ? pid : 0;
  const r = panelRect(who);
  if (!r) return;

  const el = document.createElement('div');
  el.className = 'cabo-balloon';
  el.dataset.text = 'CABO';
  el.textContent = 'CABO';
  el.style.top = r.top + r.height / 2 + 'px';
  $('fx').appendChild(el);

  // 以座位中心为锚，左右各夹一点，避免字太长时贴出视口
  const w = el.offsetWidth;
  const cx = r.left + r.width / 2;
  el.style.left = clamp(cx, w / 2 + 8, innerWidth - w / 2 - 8) + 'px';

  setTimeout(() => el.remove(), 1900);
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
  const el = pid === 0 ? $('meBar') : seatEls.get(pid);
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

/* ---------------- 回合回顾（结算要讲故事） ---------------- */

const RECAP_ICON = {
  cabo: '<svg viewBox="0 0 14 14"><path d="M2 5.4h2.6L8.6 2.6v8.8L4.6 8.6H2z"/><path d="M10.6 5.2a2.6 2.6 0 0 1 0 3.6"/></svg>',
  take: '<svg viewBox="0 0 14 14"><path d="M2 11.4h10"/><path d="M7 2v6.6M4.4 6.2 7 8.8l2.6-2.6"/></svg>',
  swap: '<svg viewBox="0 0 14 14"><path d="M2 5h8.5l-2-2M12 9H3.5l2 2"/></svg>',
  fail: '<svg viewBox="0 0 14 14"><circle cx="7" cy="7" r="5.4"/><path d="M5 5l4 4M9 5l-4 4"/></svg>',
  clean: '<svg viewBox="0 0 14 14"><circle cx="7" cy="7" r="5.4"/><path d="M4.4 7.1 6.2 9l3.5-4.2"/></svg>',
  shuffle: '<svg viewBox="0 0 14 14"><path d="M2 4.6h9M9.6 2.2l2.4 2.4-2.4 2.4M12 9.4H3M5.4 7l-2.4 2.4L5.4 11.8"/></svg>',
  reset: '<svg viewBox="0 0 14 14"><path d="M11.6 7a4.6 4.6 0 1 1-1.4-3.3"/><path d="M11.8 1.6v2.6H9.2"/></svg>',
  record: '<svg viewBox="0 0 14 14"><path d="M7 1.8l1.6 3.3 3.6.5-2.6 2.6.6 3.6L7 10.4 4.4 11.8l.6-3.6L2.4 5.6l3.6-.5z"/></svg>',
};

// 原料是客户端本轮收到的事件 + 结算视图。只用看得见的信息：
// 换了几张、谁动了谁、进账多少——暗牌的面额一概不进回顾卡。
function buildRecap(v, evts, career) {
  const cards = [];
  // 中文里「你 一手甩掉」中间那个空格是多余的，只有别人名后才需要隔一下
  const who = (i) => (i === 0 ? '你' : esc((v.players[i] || {}).name || '对手') + ' ');
  const of = (kind, pid) => evts.filter((e) => e.kind === kind && (pid === undefined || e.player === pid));
  const me = v.players[0];
  const peak = (map) => Object.keys(map).sort((a, b) => map[b] - map[a])[0];

  if (career && career.bestRound >= 0 && me.roundPts < career.bestRound) {
    cards.push({
      tone: 'gold',
      icon: 'record',
      text: `本轮 ${me.roundPts} 点，生涯新低`,
      sub: career.bestRound === 0 ? '已经是最低分了' : `原纪录 ${career.bestRound} 点`,
    });
  }

  if (v.caller >= 0) {
    const won = v.players[v.caller].roundPts === 0;
    cards.push({
      tone: won ? 'mint' : 'coral',
      icon: 'cabo',
      text: `${who(v.caller)}的 CABO ${won ? '赌赢了' : '赌输了'}`,
      sub: won ? '桌上最低，0 分入账' : `这轮进账 ${v.players[v.caller].roundPts} 分`,
    });
  }

  let big = null;
  of('take').forEach((e) => {
    if (e.value >= 2 && (!big || e.value > big.value)) big = e;
  });
  if (big) {
    cards.push({
      tone: 'mint',
      icon: 'take',
      text: `${who(big.player)}一手甩掉 ${big.value} 张`,
      sub: '同点数打包换出',
    });
  }

  const touched = {};
  of('swap').forEach((e) => {
    [e.target, e.target2].forEach((t) => {
      if (t && t.player === 0 && e.player !== 0) touched[e.player] = (touched[e.player] || 0) + 1;
    });
  });
  const culprit = peak(touched);
  if (culprit !== undefined) {
    cards.push({
      tone: 'violet',
      icon: 'swap',
      text: `${who(+culprit)}用置换动了你 ${touched[culprit]} 张牌`,
      sub: '记好的位置得重推一遍',
    });
  }

  const pen = {};
  of('penalty').forEach((e) => {
    pen[e.player] = (pen[e.player] || 0) + 1;
  });
  if (pen[0]) {
    cards.push({ tone: 'coral', icon: 'fail', text: `你记错了 ${pen[0]} 次`, sub: '翻开的牌当场公示' });
  } else {
    const worst = peak(pen);
    if (worst !== undefined) {
      cards.push({ tone: 'coral', icon: 'fail', text: `${who(+worst)}记错 ${pen[worst]} 次`, sub: '亮出来的牌跑不掉了' });
    }
  }

  const handled = of('place').length + of('take').length + of('discard').length;
  if (!pen[0] && handled >= 3 && me.roundPts <= 12) {
    cards.push({ tone: 'mint', icon: 'clean', text: '全程零失误', sub: `${handled} 次处理牌，收在 ${me.roundPts} 点` });
  }

  const rs = of('reshuffle').length;
  if (rs) {
    cards.push({ tone: 'amber', icon: 'shuffle', text: `中途洗了 ${rs} 次牌`, sub: '弃牌堆回了牌堆' });
  }

  of('reset').forEach((e) => {
    cards.push({ tone: 'gold', icon: 'reset', text: `${who(e.player)}正好 100 分`, sub: '重置回 50，本场限一次' });
  });

  return cards.slice(0, 4);
}

// 本轮结果并回生涯记录。roundEvents 为空说明是刷新页面重进的结算，不再重复上报。
function reportStats(v) {
  if (!roundEvents.length) return;
  const called = roundEvents.some((e) => e.kind === 'cabo' && e.player === 0);
  const won = called && v.caller === 0 && v.players[0].roundPts === 0;
  const mistakes = roundEvents.filter((e) => e.kind === 'penalty' && e.player === 0).length;
  api('/api/stats', {
    rounds: 1,
    bestRound: v.players[0].roundPts,
    bestStreak: roundBestStreak,
    perfect: mistakes ? 0 : 1,
    cabos: called ? 1 : 0,
    cabosWon: won ? 1 : 0,
  }).then((r) => {
    if (r && r.ok && r.stats) {
      career = r.stats;
      renderCareer();
    }
  });
}

function renderCareer() {
  const el = $('careerChip');
  if (!el) return;
  if (!career) {
    el.classList.add('hidden');
    return;
  }
  // 顶栏是不换行的 flex 行，这里只放两格能一眼扫到的数，其余收进 tooltip：
  // 五个格会把「重新开局」按钮挤出视口（实测 534px 宽时按钮跑到 659px）
  const low = career.bestRound >= 0 ? `${career.bestRound} 点` : '—';
  const st = career.bestStreak >= 2 ? `×${career.bestStreak}` : '—';
  const rate = career.cabos ? Math.round((100 * career.cabosWon) / career.cabos) + '%' : '—';
  el.title = `已结算 ${career.rounds} 轮\n单轮最低 ${low}\n最长连击 ${st}\n喊 CABO ${career.cabos} 次，对了 ${rate}\n零失误 ${career.perfect} 轮`;
  el.classList.remove('hidden');
  el.innerHTML = `<span class="cr">最低 ${esc(low)}</span><span class="cr">连击 ${esc(st)}</span>`;
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
    const who = view.caller >= 0 ? `${esc(view.players[view.caller].name)} 喊了 CABO` : '一副牌全用完了';
    html += `<div class="result-sub">${who}</div>`;
  }
  const recap = buildRecap(view, roundEvents, career);
  if (recap.length) {
    html += '<div class="recap">';
    recap.forEach((c, i) => {
      html += `<div class="recap-card" data-tone="${c.tone}" style="animation-delay:${i * 150}ms">
        <span class="rc-ico">${RECAP_ICON[c.icon] || ''}</span>
        <span class="rc-txt"><b>${c.text}</b><i>${c.sub}</i></span>
      </div>`;
    });
    html += '</div>';
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

  const reduce = reduceMotion();
  // 回顾卡先讲，再报分数；没有回顾卡时保持原来的 480ms 节奏
  const seq = reduce ? 480 : Math.max(480, 220 + recap.length * 150);
  panel.querySelectorAll('.recap-card').forEach((el, i) => {
    if (!reduce) setTimeout(() => snd('recap', i), 220 + i * 150);
  });
  panel.querySelectorAll('.score-row').forEach((row, i) => {
    const t = row.querySelector('.total');
    const from = Number(t.dataset.from);
    const to = Number(t.dataset.to);
    t.textContent = to;
    setTimeout(() => {
      if (!reduce && from !== to) countUp(t, from, to, 720);
      snd('tick', i);
    }, seq + i * 90);
  });

  // 让 CABO 的余音和回顾先落下去，再报结算
  setTimeout(() => snd(view.gameOver ? (view.winner === 0 ? 'win' : 'lose') : 'score'), seq + 300);

  reportStats(view);
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
    roundEvents = [];
    streak = 0;
    roundBestStreak = 0;
    pendingComboFx = 0;
  }
  // 回顾卡的原料：事件在服务端投递一次就清空，所以本轮的历史只能由客户端攒
  if (v.events && v.events.length) roundEvents = roundEvents.concat(v.events);
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
  roundEvents = [];
  streak = 0;
  roundBestStreak = 0;
  pendingComboFx = 0;
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
  roundEvents = [];
  streak = 0;
  roundBestStreak = 0;
  pendingComboFx = 0;
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

// 生涯记录：读不到就整块不显示，不影响牌局
api('/api/stats').then((r) => {
  if (r && r.ok && r.stats) {
    career = r.stats;
    renderCareer();
  }
});
