import assert from 'node:assert/strict';
import test from 'node:test';

globalThis.window = { matchMedia: () => ({ matches: false }) };

const { buildRecap, comboTier } = await import('../../internal/server/static/stats.mjs');

test('combo tiers keep the existing thresholds', () => {
  assert.equal(comboTier(1), 'cool');
  assert.equal(comboTier(2), 'cool');
  assert.equal(comboTier(3), 'warm');
  assert.equal(comboTier(5), 'hot');
  assert.equal(comboTier(7), 'flow');
});

test('recap uses public events without exposing hidden card faces', () => {
  const view = {
    caller: 1,
    players: [
      { name: '你', roundPts: 8 },
      { name: '电脑 A', roundPts: 0 },
    ],
  };
  const events = [
    { kind: 'take', player: 0, value: 3, card: { label: '2', points: 2 } },
    { kind: 'penalty', player: 0, card: { label: '13', points: 13 } },
    { kind: 'swap', player: 1, target: { player: 0, slot: 1 }, target2: { player: 1, slot: 0 } },
  ];
  const recap = buildRecap(view, events, null);
  assert.ok(recap.some((card) => card.icon === 'take'));
  assert.ok(recap.some((card) => card.icon === 'fail'));
  assert.ok(recap.every((card) => !card.text.includes('13') && !card.sub.includes('13')));
});

test('recap reports a new career low only when the round is lower', () => {
  const view = {
    caller: -1,
    players: [{ name: '你', roundPts: 5 }],
  };
  const recap = buildRecap(view, [], { bestRound: 8 });
  assert.ok(recap.some((card) => card.icon === 'record' && card.text.includes('生涯新低')));
  assert.ok(!buildRecap(view, [], { bestRound: 5 }).some((card) => card.icon === 'record'));
});
