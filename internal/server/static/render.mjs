import { state, snd, reduceMotion } from './core.mjs';
import { $, esc, tone, mistFigure, pct, cardFace, makeCard } from './dom.mjs';
import { buildRecap, comboTier, RECAP_ICON } from './stats.mjs';
import { comboFlare } from './effects.mjs';

let callbacks = {};

export function configureRenderer(next) {
  callbacks = { ...callbacks, ...next };
}

function isOwnPickable() {
  return !state.animating && ['peek', 'claim', 'place', 'peekSelf', 'swap'].includes(state.mode);
}
function isOppPickable() {
  return !state.animating && (state.mode === 'spy' || state.mode === 'swap');
}

/* ---------------- 渲染 ---------------- */

export function render() {
  if (!state.view) return;
  state.enterCount = 0;
  state.revealCount = 0;
  $('roundLabel').textContent = '第 ' + state.view.round + ' 轮';
  renderTurnChip();
  renderSeats();
  renderMiddle();
  renderMe();
  renderActions();
  renderOverlay();
  // 一次渲染里进来 6 张以上就当作发牌，否则只是手牌张数变化的落位
  if (state.enterCount >= 6) snd('deal', state.enterCount);
  else if (state.enterCount > 0) snd('settle', state.enterCount);
  for (let i = 0; i < state.revealCount; i++) setTimeout(() => snd('flip'), i * 95);
}

function renderTurnChip() {
  const el = $('turnChip');
  if (!state.view || state.view.gameOver || state.view.phase === 'roundEnd') {
    el.textContent = '';
    el.className = 'chip turn-chip';
    el.dataset.turn = '';
    return;
  }
  const t = state.view.players[state.view.turn];
  if (!t) {
    el.textContent = '';
    return;
  }
  // 只在真的换人时挂 swap，否则每次 render 重写 class 会把还没播完的动画掐掉
  const changed = el.dataset.turn !== String(state.view.turn);
  el.className = 'chip turn-chip p' + state.view.turn + (changed ? ' swap' : '');
  el.dataset.turn = String(state.view.turn);
  el.innerHTML = `<span class="dot"></span>轮到 ${esc(t.name)}`;
}

function renderSeats() {
  const wrap = $('seats');
  wrap.innerHTML = '';
  state.seatEls = new Map();

  state.view.players.forEach((p, i) => {
    if (i === 0) return;
    const seat = document.createElement('div');
    seat.className = 'seat p' + i;
    if (p.isTurn) seat.classList.add('active');
    if (p.isCaller) seat.classList.add('caller');

    const head = document.createElement('div');
    head.className = 'seat-head';
    head.innerHTML =
      '<span class="seat-id">' +
      `<span class="avatar">${mistFigure()}</span>` +
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
          picked: state.swapPicks.some((t) => t.player === i && t.slot === si),
          onClick: () => callbacks.pickCard(i, si),
        })
      );
    });
    seat.appendChild(hand);

    state.seatEls.set(i, seat);
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
  $('deckCount').textContent = state.view.deckCount;
  $('discardCount').textContent = state.view.discardCount;

  const stack = document.querySelector('.deck-stack');
  if (stack) {
    stack.dataset.tier = deckTier(state.view.deckCount);
    if (state.prevDeckCount !== null && state.prevDeckCount !== state.view.deckCount) {
      stack.classList.remove('bump', 'riffle');
      void stack.offsetWidth;
      stack.classList.add('bump');
    }
  }
  state.prevDeckCount = state.view.deckCount;

  const holder = document.querySelector('.discard-holder');
  if (holder) {
    holder.classList.toggle('open', state.discardOpen);
    holder.innerHTML = '';
    if (state.view.discardUnder && state.view.discardUnder.length) {
      const fan = document.createElement('div');
      fan.className = 'discard-fan';
      state.view.discardUnder.forEach((c) => {
        const f = document.createElement('div');
        f.className = 'card small';
        f.dataset.tone = tone(c);
        f.innerHTML = cardFace(c);
        fan.appendChild(f);
      });
      holder.appendChild(fan);
    }
    if (state.view.discardTop) {
      const el = document.createElement('div');
      el.className = 'card';
      el.dataset.tone = tone(state.view.discardTop);
      el.innerHTML = cardFace(state.view.discardTop);
      if (state.prevDiscardId !== state.view.discardTop.id) {
        el.classList.add('dropIn');
        // 参考稿那条「卡一停稳，线才画出」：等落牌动画结束再描下划线
        setTimeout(() => el.classList.add('settled'), 440);
      }
      holder.appendChild(el);
      state.prevDiscardId = state.view.discardTop.id;
    } else {
      state.prevDiscardId = null;
    }
  }

  const slot = $('pendingSlot');
  slot.innerHTML = '';
  if (state.view.pending) {
    const hint = document.createElement('div');
    hint.className = 'pending-hint';
    hint.textContent = '摸到了';
    slot.appendChild(hint);
    const el = document.createElement('div');
    el.className = 'card enter';
    el.dataset.tone = tone(state.view.pending);
    el.innerHTML = cardFace(state.view.pending);
    slot.appendChild(el);
  }
}

function renderMe() {
  const p = state.view.players[0];
  const bar = $('meBar');
  let badges = '';
  if (p.isTurn && !state.view.gameOver && state.view.phase !== 'roundEnd') {
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
    !state.view.gameOver && state.view.phase !== 'roundEnd'
      ? `<span class="me-est" title="已知的牌共 ${knownSum} 点，还有 ${unknown} 张没看过">已知 ${knownSum} 点 · 未知 ${unknown} 张</span>`
      : '';
  const combo =
    state.streak >= 2 && !state.view.gameOver && state.view.phase !== 'roundEnd'
      ? `<span class="combo-chip" data-tier="${comboTier(state.streak)}" title="连续 ${state.streak} 次把自己已知的牌换成更小的">连击 ×${state.streak}</span>`
      : '';
  bar.innerHTML =
    `<span class="avatar p0">${mistFigure()}</span>` +
    `<span>${esc(p.name)}</span>` +
    `<b>${p.score} 分</b>` +
    `<span class="me-progress"><i style="width:${pct(p.score)}%"></i></span>` +
    est +
    combo +
    badges;

  if (state.pendingComboFx) {
    const n = state.pendingComboFx;
    state.pendingComboFx = 0;
    const chip = bar.querySelector('.combo-chip');
    if (chip) chip.classList.add('pop');
    snd('combo', n);
    if (n >= 4) comboFlare(n >= 7 ? '手感来了' : '还在涨', n >= 7 ? '#E9A427' : '#3FBFB4', comboTier(n));
  }

  const hand = $('myHand');
  hand.innerHTML = '';
  p.hand.forEach((slot, i) => {
    hand.appendChild(
      makeCard(0, i, slot, {
        clickable: isOwnPickable(),
        picked: state.picked.includes(i) || state.swapPicks.some((t) => t.player === 0 && t.slot === i),
        onClick: () => callbacks.pickCard(0, i),
      })
    );
  });
}

function renderActions() {
  const bar = $('actions');
  bar.innerHTML = '';
  if (!state.view) return;

  const add = (label, cls, fn, disabled) => {
    const b = document.createElement('button');
    b.className = 'btn' + (cls ? ' ' + cls : '');
    b.textContent = label;
    b.disabled = !!disabled || state.animating;
    b.addEventListener('click', fn);
    bar.appendChild(b);
  };
  const hint = (t) => {
    const s = document.createElement('div');
    s.className = 'hint';
    s.textContent = t;
    bar.appendChild(s);
  };

  if (state.view.gameOver || state.view.phase === 'roundEnd') return;

  if (state.mode === 'peek') {
    hint(`选 2 张自己的牌偷看（${state.picked.length}/2）`);
    add('确认', 'primary', callbacks.doPeek, state.picked.length !== 2);
    return;
  }
  if (state.mode === 'claim') {
    hint(`选 1–3 张手牌换出（${state.picked.length}）`);
    add('确认换牌', 'primary', callbacks.doClaim, state.picked.length < 1);
    add('取消', '', () => { state.mode = 'idle'; state.picked = []; render(); });
    return;
  }
  if (state.mode === 'place') {
    hint('点一张手牌，把它换掉');
    add('取消', '', () => { state.mode = 'idle'; state.picked = []; render(); });
    return;
  }
  if (state.mode === 'peekSelf') {
    hint('点自己一张牌偷看');
    add('取消', '', () => { state.mode = 'idle'; render(); });
    return;
  }
  if (state.mode === 'spy') {
    hint('点对手的一张牌偷看');
    add('取消', '', () => { state.mode = 'idle'; render(); });
    return;
  }
  if (state.mode === 'swap') {
    hint(`点两张牌交换（${state.swapPicks.length}/2）`);
    add('确认交换', 'primary', callbacks.doSwap, state.swapPicks.length !== 2);
    add('取消', '', () => { state.mode = 'idle'; state.swapPicks = []; render(); });
    return;
  }

  const c = state.view.can;

  if (state.view.phase === 'peek' && c.peek) {
    hint('先看自己 2 张牌，记住它们的位置');
    add('看牌', 'primary', () => { state.mode = 'peek'; state.picked = []; render(); });
    return;
  }

  if (c.discardPending) {
    add('弃掉', '', () => callbacks.act({ type: 'discardPending' }));
    add('换入手牌', 'primary', () => { state.mode = 'place'; state.picked = []; render(); });
    if (c.usePower) add('发动技能', '', callbacks.startPower);
    return;
  }

  if (c.drawDeck) {
    add('摸牌堆', 'primary', () => callbacks.act({ type: 'drawDeck' }));
    add('拿弃牌堆', '', () => { state.mode = 'claim'; state.picked = []; render(); }, !c.takeDiscard);
    add('喊 CABO', 'danger', callbacks.callCabo, !c.callCabo);
    // 灰掉的按钮要说清楚为什么，别让玩家猜
    if (!c.takeDiscard) hint('弃牌堆还是空的，这回合只能摸牌堆');
    else if (!c.callCabo) hint('已经有人喊过 CABO，本轮不能再喊');
    return;
  }

  const t = state.view.players[state.view.turn];
  hint(t ? `等待 ${t.name} 行动…` : '');
}

export function renderCareer() {
  const el = $('careerChip');
  if (!el) return;
  if (!state.career) {
    el.classList.add('hidden');
    return;
  }
  // 顶栏是不换行的 flex 行，这里只放两格能一眼扫到的数，其余收进 tooltip：
  // 五个格会把「重新开局」按钮挤出视口（实测 534px 宽时按钮跑到 659px）
  const low = state.career.bestRound >= 0 ? `${state.career.bestRound} 点` : '—';
  const st = state.career.bestStreak >= 2 ? `×${state.career.bestStreak}` : '—';
  const rate = state.career.cabos ? Math.round((100 * state.career.cabosWon) / state.career.cabos) + '%' : '—';
  el.title = `已结算 ${state.career.rounds} 轮\n单轮最低 ${low}\n最长连击 ${st}\n喊 CABO ${state.career.cabos} 次，对了 ${rate}\n零失误 ${state.career.perfect} 轮`;
  el.classList.remove('hidden');
  el.innerHTML = `<span class="cr">最低 ${esc(low)}</span><span class="cr">连击 ${esc(st)}</span>`;
}

export function countUp(el, from, to, dur) {
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

// .hidden 是 display:none，退场动画必须先播完再挂上，否则直接被砍掉
export function hideOverlay(el) {
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

export function renderOverlay() {
  const ov = $('roundOverlay');
  const show = state.view.phase === 'roundEnd' || state.view.gameOver;
  if (!show) {
    hideOverlay(ov);
    return;
  }
  ov.classList.remove('hidden');
  if (ov.dataset.round === String(state.view.round) && ov.dataset.over === String(state.view.gameOver)) return;
  ov.dataset.round = String(state.view.round);
  ov.dataset.over = String(state.view.gameOver);

  const panel = document.createElement('div');
  panel.className = 'panel';

  let html = '';
  if (state.view.gameOver) {
    html += `<div class="result-title">${esc(state.view.players[state.view.winner].name)} 获胜</div>`;
    html += '<div class="result-sub">总分最低</div>';
  } else {
    html += `<div class="result-title">第 ${state.view.round} 轮结算</div>`;
    const who = state.view.caller >= 0 ? `${esc(state.view.players[state.view.caller].name)} 喊了 CABO` : '一副牌全用完了';
    html += `<div class="result-sub">${who}</div>`;
  }
  const recap = buildRecap(state.view, state.roundEvents, state.career);
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
  state.view.players.forEach((p, i) => {
    const from = state.prevScores[i] === undefined ? 0 : state.prevScores[i];
    const pct = Math.min(100, (p.score / 100) * 100);
    html += `<div class="score-row p${i}${i === 0 ? ' me' : ''}" style="animation-delay:${i * 70}ms">
      <span class="avatar">${mistFigure()}</span>
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
  btn.textContent = state.view.gameOver ? '再来一局' : '下一轮';
  btn.addEventListener('click', state.view.gameOver ? callbacks.restart : callbacks.nextRound);
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
  setTimeout(() => snd(state.view.gameOver ? (state.view.winner === 0 ? 'win' : 'lose') : 'score'), seq + 300);

  callbacks.reportStats();
}
