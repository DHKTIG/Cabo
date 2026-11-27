import { state, snd, sndMode, sleep, clamp } from './core.mjs';
import { $ } from './dom.mjs';
import { api } from './api.mjs';
import { comboTier } from './stats.mjs';
import { render, renderCareer, renderSelection } from './render.mjs';
import { toast, comboFlare, centerOf, circleWipe, doneBurst, syncFlash, playFlights, playEvents, pulseSeat } from './effects.mjs';

/* ---------------- 交互 ---------------- */

export function pickCard(player, slot) {
  if (state.busy || state.animating || !state.view) return;

  if (state.mode === 'peek' || state.mode === 'claim') {
    if (player !== 0) return;
    const at = state.picked.indexOf(slot);
    if (at >= 0) {
      state.picked.splice(at, 1);
      snd('deselect');
    } else if (state.picked.length < (state.mode === 'peek' ? 2 : 3)) {
      state.picked.push(slot);
      snd('select', state.picked.length - 1);
    } else {
      snd('error');
      return;
    }
    renderSelection();
    return;
  }
  if (state.mode === 'place') {
    if (player !== 0) return;
    state.picked = [slot];
    doPlace();
    return;
  }
  if (state.mode === 'peekSelf') {
    if (player !== 0) return;
    act({ type: 'usePower', targets: [{ player: 0, slot }] });
    return;
  }
  if (state.mode === 'spy') {
    if (player === 0) return;
    act({ type: 'usePower', targets: [{ player, slot }] });
    return;
  }
  if (state.mode === 'swap') {
    const at = state.swapPicks.findIndex((t) => t.player === player && t.slot === slot);
    if (at >= 0) {
      state.swapPicks.splice(at, 1);
      snd('deselect');
    } else if (state.swapPicks.length < 2) {
      state.swapPicks.push({ player, slot });
      snd('select', state.swapPicks.length - 1);
    } else {
      snd('error');
      return;
    }
    renderSelection();
    return;
  }
}

export function startPower() {
  const pw = state.view.pending && state.view.pending.power;
  if (pw === 'peek') { state.mode = 'peekSelf'; render(); }
  else if (pw === 'spy') { state.mode = 'spy'; render(); }
  else if (pw === 'swap') { state.mode = 'swap'; state.swapPicks = []; render(); }
}

export function doPeek() { act({ type: 'peek', indices: state.picked.slice() }); }
export function doClaim() { act({ type: 'takeDiscard', indices: state.picked.slice() }); }
export function doPlace() { act({ type: 'place', indices: state.picked.slice() }); }
export function doSwap() { act({ type: 'usePower', targets: state.swapPicks.slice() }); }

// 喊 CABO 是这局真正的「完成」键：先把 done 演出放出去，再交给服务端结算。
// 单机本地接口不会失败，所以这里用乐观演出换跟手感。
export function callCabo(e) {
  doneBurst(e && e.currentTarget, 'var(--sun)');
  act({ type: 'callCabo' });
}

export async function act(body) {
  if (state.busy || state.animating) return;
  state.busy = true;
  const before = state.view;
  let r;
  try {
    r = await api('/api/action', body);
  } catch (err) {
    // api 抛异常（服务端 panic / 非 JSON / 断网）时 busy 必须复位，否则整局 UI 永久卡死
    state.busy = false;
    toast('网络异常，请重试');
    return;
  }
  state.busy = false;
  if (!r.ok) { toast(r.error); return; }
  judgeStreak(before, body, r.view);
  const mine = body.type === 'place' || body.type === 'takeDiscard' || body.type === 'discardPending';
  applyView(r.view);
  // 一个动作同时惊动分数、进度条和估值，别只让数字悄悄变
  if (mine) setTimeout(syncFlash, 260);
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
    setStreak(before.pending.points < old.card.points ? state.streak + 1 : 0);
    return;
  }

  if (body.type === 'takeDiscard' && before.discardTop) {
    const out = body.indices.map((i) => me.hand[i]);
    if (out.some((s) => !s || !s.visible || !s.card)) return;
    const tp = before.discardTop.points;
    const better = out.every((s) => tp < s.card.points);
    setStreak(better ? state.streak + 1 : 0);
  }
}

function setStreak(n) {
  const prev = state.streak;
  state.streak = clamp(n, 0, 99);
  if (state.streak > state.roundBestStreak) state.roundBestStreak = state.streak;
  if (state.streak > prev && state.streak >= 2) state.pendingComboFx = state.streak;
  else if (state.streak < prev && prev >= 3) {
    snd('comboBreak');
    comboFlare('连击断了', '#9b9387', comboTier(prev));
  }
}

export async function nextRound(e) {
  if (state.busy) return;
  state.busy = true;
  const c = centerOf(e && e.currentTarget);
  let r;
  try {
    r = await api('/api/next', {});
  } catch (err) {
    state.busy = false;
    toast('网络异常，请重试');
    return;
  }
  state.busy = false;
  if (!r.ok) { toast(r.error); return; }
  await circleWipe(c.x, c.y, () => applyView(r.view));
}

// 本轮结果并回生涯记录。roundEvents 为空说明是刷新页面重进的结算，不再重复上报。
export function reportStats() {
  if (!state.roundEvents.length) return;
  api('/api/stats').then((r) => {
    if (r && r.ok && r.stats) {
      state.career = r.stats;
      renderCareer();
    }
  });
}

/* ---------------- 主流程 ---------------- */

function setThinking(pid) {
  state.seatEls.forEach((el, i) => el.classList.toggle('thinking', i === pid));
}

// 有人喊了 CABO 或分数逼近 100，配乐就换成更紧的一档；结算/结束时退回稀疏的音乐盒
function musicMode(v) {
  if (!v || v.gameOver || v.phase === 'roundEnd') return 'menu';
  if (v.players.some((p) => p.isCaller) || v.players.some((p) => p.score >= 80)) return 'tension';
  return 'table';
}

// 换局/重开/开局时清掉本轮的动画与连击状态，三处调用点共用，避免字段漂移
export function resetRoundState() {
  state.prevCards = new Map();
  state.prevDiscardId = null;
  state.prevDeckCount = null;
  state.roundEvents = [];
  state.streak = 0;
  state.roundBestStreak = 0;
  state.pendingComboFx = 0;
}

export function setView(v, opts = {}) {
  state.prevScores = state.view ? state.view.players.map((p) => p.score) : [];
  if (state.view && state.view.round !== v.round) {
    resetRoundState();
  }
  // 回顾卡的原料：事件在服务端投递一次就清空，所以本轮的历史只能由客户端攒
  if (v.events && v.events.length) state.roundEvents = state.roundEvents.concat(v.events);
  state.view = v;
  state.mode = 'idle';
  state.picked = [];
  state.swapPicks = [];
  sndMode(musicMode(v));
  render();
  playEvents(v.events, opts);
}

export function applyView(v) {
  setView(v);
  runAI();
}

// AI 的动作：先用旧画面播完飞牌动画，再更新状态
async function applyViewAnimated(v) {
  state.animating = true;
  try {
    await playFlights(v.events);
  } finally {
    state.animating = false;
  }
  setView(v, { flown: true });
  for (const e of v.events || []) {
    if (e.kind === 'draw' || e.kind === 'take') pulseSeat(e.player);
  }
}

export async function runAI() {
  if (state.aiRunning) return;
  state.aiRunning = true;
  try {
    for (let guard = 0; guard < 400; guard++) {
      if (!state.view || state.view.gameOver) break;
      if (state.view.phase !== 'turn' && state.view.phase !== 'place') break;
      if (state.view.turn === 0) break;

      const pid = state.view.turn;
      // 已经摸完牌、正在决定怎么处理时多停一会儿，像在思考
      const wait = state.view.phase === 'place' ? 1050 : 820;

      setThinking(pid);
      await sleep(wait);

      const r = await api('/api/ai', {});
      setThinking(-1);
      if (!r.ok) { toast(r.error); break; }
      await applyViewAnimated(r.view);
    }
  } finally {
    state.aiRunning = false;
    setThinking(-1);
  }
}

export async function startGame(e) {
  const name = ($('nameInput').value || '').trim() || '玩家';
  let r;
  try {
    r = await api('/api/new', { name, ai: state.aiCount, level: state.aiLevel });
  } catch (err) {
    toast('网络异常，请重试');
    return;
  }
  if (!r.ok) { toast(r.error || '开局失败'); return; }
  const c = centerOf(e && e.currentTarget);
  // 转场已经把屏幕盖住了，旧overlay直接切掉，不用再走一遍退场动画
  await circleWipe(c.x, c.y, () => {
    $('startOverlay').classList.add('hidden');
    $('app').classList.remove('hidden');
    resetRoundState();
    state.prevScores = [];
    snd('start');
    applyView(r.view);
  });
}

export function restart(e) {
  const c = centerOf(e && e.currentTarget);
  circleWipe(c.x, c.y, () => restartNow());
}

function restartNow() {
  const ov = $('roundOverlay');
  ov.classList.add('hidden');
  ov.dataset.round = '';
  ov.dataset.over = '';
  $('app').classList.add('hidden');
  $('startOverlay').classList.remove('hidden');
  state.view = null;
  resetRoundState();
  sndMode('menu');
}
