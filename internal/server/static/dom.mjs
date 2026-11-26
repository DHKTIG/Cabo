import { state } from './core.mjs';

export const $ = (id) => document.getElementById(id);

export function esc(s) {
  return String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

export function tone(card) {
  if (!card) return null;
  if (card.points === 0) return 'zero';
  if (card.power) return card.power;
  if (card.points >= 13) return 'high';
  return 'low';
}

// 气雾灵体：柔光雾团 + 素净五官，配色由所在容器的 --mist-* 变量决定（见 .p0–.p3）
export function mistFigure() {
  return (
    '<svg class="mist" viewBox="0 0 64 64" aria-hidden="true">' +
    '<g class="puffs">' +
    '<circle class="pa" cx="25" cy="34" r="9"/><circle class="pa" cx="40" cy="34" r="9"/>' +
    '<circle class="pb" cx="32" cy="28" r="10"/><circle class="pc" cx="32" cy="39" r="11"/>' +
    '</g>' +
    '<circle class="core" cx="32" cy="36" r="12"/>' +
    '<circle class="eye" cx="28" cy="35" r="2.1"/><circle class="eye" cx="36" cy="35" r="2.1"/>' +
    '<circle class="glint" cx="28.6" cy="34.3" r=".6"/><circle class="glint" cx="36.6" cy="34.3" r=".6"/>' +
    '<path class="mouth" d="M30 40q2 1.5 4 0"/>' +
    '</svg>'
  );
}

export function pct(score) {
  return Math.min(100, Math.max(0, score));
}

export function cardFace(card) {
  return (
    '<div class="face front">' +
    '<i class="wash"></i>' +
    `<span class="rank">${esc(card.label)}</span>` +
    (card.power ? `<span class="tag">${esc(card.powerZh)}</span>` : '') +
    '<i class="dot"></i>' +
    '</div>'
  );
}

export function makeCard(pid, idx, slot, opts = {}) {
  const el = document.createElement('div');
  el.className = 'card';
  if (opts.small) el.classList.add('small');

  const posKey = pid + ':' + idx;
  el.dataset.ref = posKey;

  const prev = state.prevCards.get(posKey);
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
      state.enterCount++;
    } else if (anim === 'reveal') state.revealCount++;
  }

  state.prevCards.set(posKey, { visible: !!card, id: card ? card.id : 0 });
  return el;
}
