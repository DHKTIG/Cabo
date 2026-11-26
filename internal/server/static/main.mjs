import { dispose } from './audio.mjs';
import { state, snd } from './core.mjs';
import { $ } from './dom.mjs';
import { api } from './api.mjs';
import { configureRenderer, render, renderCareer } from './render.mjs';
import {
  pickCard,
  act,
  doPeek,
  doClaim,
  doPlace,
  doSwap,
  startPower,
  callCabo,
  restart,
  nextRound,
  reportStats,
  startGame,
  applyView,
  setView,
} from './flow.mjs';

configureRenderer({
  pickCard,
  act,
  doPeek,
  doClaim,
  doPlace,
  doSwap,
  startPower,
  callCabo,
  restart,
  nextRound,
  reportStats,
});

// 统一的按钮反馈音；用委托监听，动态生成的操作按钮也能覆盖到
document.addEventListener('click', (e) => {
  if (e.target.closest('button')) snd('ui');
});

$('aiPicker').addEventListener('click', (e) => {
  const b = e.target.closest('button[data-ai]');
  if (!b) return;
  state.aiCount = Number(b.dataset.ai);
  [...$('aiPicker').children].forEach((x) => x.classList.toggle('on', x === b));
});

$('levelPicker').addEventListener('click', (e) => {
  const b = e.target.closest('button[data-lv]');
  if (!b) return;
  state.aiLevel = Number(b.dataset.lv);
  [...$('levelPicker').children].forEach((x) => x.classList.toggle('on', x === b));
});

$('btnStart').addEventListener('click', startGame);
$('btnRestart').addEventListener('click', restart);

window.addEventListener('pagehide', dispose);

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
    state.career = r.stats;
    renderCareer();
  }
});

// ?dev=1 才挂：把内部状态和渲染函数交给 window，并加载 devshots.mjs 里的画面场景，
// 供无头截图工装用 ?dev=1&shot=recap 直接把界面推到某个状态。
// 正式入口的 URL 由 exe 生成，不带这个参数，所以对玩家是死代码。
if (location.search.includes('dev=1')) {
  window.__cab = {
    get: () => ({ view: state.view, streak: state.streak, roundEvents: state.roundEvents, career: state.career, roundBestStreak: state.roundBestStreak }),
    // AI 循环会自己 applyView 覆盖外部推进去的画面，工装必须先等它停手
    idle: () => !state.busy && !state.animating && !state.aiRunning,
    apply: (p) => {
      if (p.view) setView(p.view);
      if (p.roundEvents) state.roundEvents = p.roundEvents;
      if (typeof p.streak === 'number') state.streak = p.streak;
      if (typeof p.roundBestStreak === 'number') state.roundBestStreak = p.roundBestStreak;
      if (p.career) {
        state.career = p.career;
        renderCareer();
      }
      // 结算面板有「同一轮只渲染一次」的守卫，工装要重推画面必须先把它清掉，
      // 否则第二次 render() 直接 return，看到的还是第一次（真实事件）的结果。
      const ov = $('roundOverlay');
      if (ov) {
        ov.dataset.round = '';
        ov.dataset.over = '';
      }
      render();
    },
  };
  import('./devshots.mjs');
}

window.__CABO_READY__ = true;
