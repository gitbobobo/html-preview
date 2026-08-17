/**
 * Tile markup — single source of truth for the home grid card.
 */

import { escapeHtml, formatDate } from './util.js';
import { renderCardThumb } from './thumb.js';
import { STAR_ICON } from './icons.js';

export function tileDisplayTitle(item) {
  return item.title || '未命名';
}

export function tileAriaLabel(item) {
  return '编辑：' + tileDisplayTitle(item);
}

/** Always-visible favorite badge pinned to the media's top-right corner. */
export function tileFavoriteHTML(item) {
  const on = Boolean(item.favorite);
  return `
        <button type="button" class="tile-fav${on ? ' active' : ''}" aria-pressed="${on}" aria-label="${on ? '取消收藏' : '收藏'}" title="${on ? '取消收藏' : '收藏'}">${STAR_ICON}</button>
      `;
}

/** Sync an existing tile's favorite badge to the item's favorite state. */
export function applyFavoriteToCard(card, favorite) {
  const btn = card ? card.querySelector('.tile-fav') : null;
  if (!btn) return;
  const on = Boolean(favorite);
  btn.classList.toggle('active', on);
  btn.setAttribute('aria-pressed', on ? 'true' : 'false');
  btn.setAttribute('aria-label', on ? '取消收藏' : '收藏');
  btn.setAttribute('title', on ? '取消收藏' : '收藏');
}

export function tileCardHTML(item) {
  return `
      <article class="tile" data-id="${escapeHtml(item.id)}" tabindex="0" role="button" aria-label="${escapeHtml(tileAriaLabel(item))}">
        <div class="tile-media">${renderCardThumb(item)}${tileFavoriteHTML(item)}</div>
        <div class="tile-caption">
          <h3 class="tile-title">${escapeHtml(tileDisplayTitle(item))}</h3>
          <p class="tile-meta">${formatDate(item.updated_at)}</p>
        </div>
      </article>
    `;
}
