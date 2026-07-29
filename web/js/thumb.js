/**
 * Thumbnail rendering — responsive picture + placeholders.
 */

import { SCREENSHOT_PLACEHOLDERS } from './enums.js';
import { tileDisplayTitle, tileAriaLabel } from './tile.js';

export function renderThumbPicture(item) {
  const ss = item.screenshot_status;
  const desktop = item.thumbs?.desktop;
  const mobile = item.thumbs?.mobile;

  if (ss === 'ready' && (desktop || mobile)) {
    const bust = item.updated_at ? `?v=${encodeURIComponent(item.updated_at)}` : `?v=${Date.now()}`;
    const dSrc = (desktop || mobile) + bust;
    const mSrc = (mobile || desktop) + bust;
    return `
      <picture>
        <source media="(max-width: 480px)" srcset="${mSrc}">
        <img src="${dSrc}" alt="" loading="lazy">
      </picture>`;
  }

  const placeholder = SCREENSHOT_PLACEHOLDERS[ss] || SCREENSHOT_PLACEHOLDERS.pending;
  return `<img src="${placeholder}" alt="" loading="lazy">`;
}

export function renderCardThumb(item) {
  return `<div class="card-thumb">${renderThumbPicture(item)}</div>`;
}

/** Update an existing grid tile's media + optional title from latest item. */
export function applyItemToCard(card, item) {
  if (!card || !item) return;
  const title = card.querySelector('.tile-title');
  if (title) title.textContent = tileDisplayTitle(item);
  const media = card.querySelector('.tile-media');
  if (media) {
    media.innerHTML = renderCardThumb(item);
  }
  if (card.hasAttribute('aria-label')) {
    card.setAttribute('aria-label', tileAriaLabel(item));
  }
}
