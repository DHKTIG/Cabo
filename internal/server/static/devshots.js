/* devshots.js —— 只在 ?dev=1 下加载的截图场景（见 app.js 末尾的挂载点）。
   用途：无头 Edge 用 ?dev=1&shot=recap 直接把界面推到某个状态，
   这样动效和版式改动能被截图验证，而不是靠读代码想象。
   这里只做「把画面推到某一刻」，不参与任何游戏规则。 */

(async () => {
  const q = new URLSearchParams(location.search);
  const scene = q.get('shot') || 'table';
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  // AI 循环会自己 applyView 覆盖外部推进去的画面，先等它停手
  const settle = async () => {
    for (let i = 0; i < 90 && !__cab.idle(); i++) await sleep(150);
  };

  if (scene === 'start') return; // 首页：什么都别做，停在开场屏
  await startGame();
  await sleep(2600);

  await act({ type: 'peek', indices: [0, 1] });
  await sleep(1400);
  if (scene === 'table') return;

  const v = __cab.get().view;

  if (scene === 'combo') {
    await settle();
    __cab.apply({ streak: Number(q.get('n') || 5) });
    return;
  }

  if (scene === 'burst') {
    await settle();
    const btn = document.querySelector('#actions .btn.danger') || document.querySelector('#actions .btn.primary');
    doneBurst(btn, 'var(--sun)', 60000);
    // 冻在迸发中间那一帧，否则截图永远只拍到空场
    await sleep(320);
    document.querySelectorAll('.burst, .burst *').forEach((el) => {
      el.getAnimations().forEach((a) => {
        a.pause();
        a.currentTime = 300;
      });
    });
    return;
  }

  if (scene === 'discard') {
    await settle();
    const v2 = __cab.get().view;
    const base = JSON.parse(JSON.stringify(v2));
    base.discardCount = 14;
    base.discardTop = { id: 71, label: '5', points: 5, power: '', powerZh: '' };
    base.discardUnder = [9, 12, 2, 0, 13, 7].map((n, i) => ({
      id: 80 + i,
      label: String(n),
      points: n,
      power: n === 12 ? 'swap' : n === 9 ? 'spy' : n === 7 ? 'peek' : '',
      powerZh: n === 12 ? '换' : n === 9 ? '窥' : n === 7 ? '透' : '',
    }));
    __cab.apply({ view: base });
    toggleDiscard();
    return;
  }

  if (scene === 'recap') {
    await settle();
    const base = JSON.parse(JSON.stringify(v));
    base.phase = 'roundEnd';
    base.caller = 1;
    base.players = base.players.map((p, i) =>
      Object.assign({}, p, {
        roundPts: [8, 0, 41][i],
        score: [23, 100, 61][i],
        name: ['你', '电脑 A', '电脑 B'][i],
        isCaller: i === 1,
      })
    );
    __cab.apply({
      view: base,
      roundBestStreak: 5,
      career: { bestRound: 14, bestStreak: 4, rounds: 9, perfect: 2, cabos: 3, cabosWon: 1 },
      roundEvents: [
        { kind: 'take', player: 0, value: 3, card: { label: '2', points: 2 } },
        { kind: 'penalty', player: 2, card: { label: '12', points: 12 } },
        { kind: 'swap', player: 1, target: { player: 0, slot: 1 }, target2: { player: 1, slot: 0 } },
        { kind: 'swap', player: 1, target: { player: 2, slot: 2 }, target2: { player: 0, slot: 3 } },
        { kind: 'reshuffle', player: -1, value: 28 },
        { kind: 'cabo', player: 1 },
        { kind: 'place', player: 0, card: { label: '7', points: 7 } },
        { kind: 'discard', player: 2, card: { label: '4', points: 4 } },
        { kind: 'reset', player: 2 },
      ],
    });
    return;
  }

  if (scene === 'place') {
    // 摸一张牌进「换入手牌」状态，看选牌态与描边
    await act({ type: 'drawDeck' });
    await sleep(900);
    mode = 'place';
    render();
    return;
  }

  if (scene === 'cabo') {
    showCaboBanner(0);
    return;
  }
})();
