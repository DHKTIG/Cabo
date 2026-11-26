import { esc } from './dom.mjs';

// 连击档位：2 起步、3 变色、5 加描边、7 起才是「手感来了」的金档
export function comboTier(n) {
  if (n >= 7) return 'flow';
  if (n >= 5) return 'hot';
  if (n >= 3) return 'warm';
  return 'cool';
}

export const RECAP_ICON = {
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
export function buildRecap(v, evts, career) {
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
