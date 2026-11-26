// 全局可变状态与跨模块基础能力：渲染、流程和动画共享同一份实时状态。
export const state = {
  view: null,
  mode: 'idle',
  picked: [],
  swapPicks: [],
  busy: false,
  animating: false,
  aiRunning: false,
  aiCount: 2,
  aiLevel: 1,

  prevCards: new Map(),
  prevDiscardId: null,
  prevDeckCount: null,
  prevScores: [],
  seatEls: new Map(),
  enterCount: 0,
  revealCount: 0,

  // 累积式反馈：连击只在「本轮」内累加，跨局的生涯记录存服务端（见 internal/server/stats.go）
  streak: 0,
  roundBestStreak: 0,
  roundEvents: [],
  career: null,
  pendingComboFx: 0,
};

// audio.mjs 提供 window.snd；音频起不来时静默降级，不能影响牌局。
export const snd = (...a) => window.snd && window.snd(...a);
export const sndMode = (m) => window.sndMode && window.sndMode(m);

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
export const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));
// 缓存 MediaQueryList，避免每次调用都新建对象（hideOverlay/circleWipe/syncFlash 等高频用到）
const reduceMotionMql = window.matchMedia('(prefers-reduced-motion: reduce)');
export const reduceMotion = () => reduceMotionMql.matches;
