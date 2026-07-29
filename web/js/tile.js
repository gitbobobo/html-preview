/**
 * Tile markup — single source of truth for the home grid card.
 */

import { escapeHtml, formatDate } from './util.js';
import { renderCardThumb } from './thumb.js';

export function tileDisplayTitle(item) {
  return item.title || '未命名';
}

export function tileAriaLabel(item) {
  return '编辑：' + tileDisplayTitle(item);
}

export function tileCardHTML(item) {
  return `
      <article class="tile" data-id="${escapeHtml(item.id)}" tabindex="0" role="button" aria-label="${escapeHtml(tileAriaLabel(item))}">
        <div class="tile-media">${renderCardThumb(item)}</div>
        <div class="tile-caption">
          <h3 class="tile-title">${escapeHtml(tileDisplayTitle(item))}</h3>
          <p class="tile-meta">${formatDate(item.updated_at)}</p>
        </div>
      </article>
    `;
}
