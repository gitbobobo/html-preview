/**
 * Shared infinite-scroll item list for home / trash pages.
 */

import { listItems } from './api.js';
import { escapeHtml, debounce } from './util.js';

const DEFAULT_PAGE_SIZE = 24;

/**
 * @param {object} opts
 * @param {'active'|'trash'} opts.status
 * @param {string} opts.gridId
 * @param {string} opts.sentinelId
 * @param {string} opts.statusId
 * @param {(item: object) => string} opts.renderCard
 * @param {(card: HTMLElement, item: object) => void} opts.bindCard
 * @param {{ title: string, hint?: string, icon?: 'library'|'trash'|'search'|'star' } | () => { title: string, hint?: string, icon?: 'library'|'trash'|'search'|'star' }} opts.emptyText
 * @param {string} [opts.searchInputId]
 * @param {object} [opts.extraQuery] — opaque fields merged into listItems
 * @param {(item: object) => boolean} [opts.matchesItem] — client-side membership filter
 */
export function createItemListPage({
  status,
  gridId,
  sentinelId,
  statusId,
  renderCard,
  bindCard,
  emptyText,
  searchInputId,
  extraQuery: initialExtraQuery = {},
  matchesItem = () => true,
}) {
  const state = {
    q: '',
    page: 1,
    pageSize: DEFAULT_PAGE_SIZE,
    total: 0,
    items: [],
    loading: false,
    hasMore: true,
  };

  let extraQuery = { ...initialExtraQuery };
  let observer = null;

  function resolveEmptyCopy() {
    if (typeof emptyText === 'function') return emptyText();
    return emptyText;
  }

  function setupObserver() {
    if (observer) observer.disconnect();
    const sentinel = document.getElementById(sentinelId);
    observer = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting && state.hasMore && !state.loading) {
        loadMore();
      }
    }, { rootMargin: '200px' });
    observer.observe(sentinel);
  }

  function resetAndLoad() {
    state.page = 1;
    state.items = [];
    state.hasMore = true;
    document.getElementById(gridId).innerHTML = '';
    loadMore();
  }

  async function loadMore() {
    if (state.loading || !state.hasMore) return;
    state.loading = true;
    setStatus('loading');

    try {
      const data = await listItems({
        q: state.q,
        status,
        page: state.page,
        page_size: state.pageSize,
        ...extraQuery,
      });

      state.total = data.total;
      const newItems = data.items || [];
      state.items.push(...newItems);
      state.hasMore = state.items.length < state.total;
      state.page += 1;

      renderGrid(newItems);
      setStatus(state.items.length === 0 ? 'empty' : state.hasMore ? 'more' : 'done');
    } catch (err) {
      setStatus('error', err.message);
    } finally {
      state.loading = false;
    }
  }

  function renderGrid(items) {
    const grid = document.getElementById(gridId);
    const html = items.map((item) => renderCard(item)).join('');
    grid.insertAdjacentHTML('beforeend', html);

    grid.querySelectorAll('[data-id]:not([data-bound])').forEach((card) => {
      card.setAttribute('data-bound', '1');
      const item = state.items.find((i) => i.id === card.dataset.id);
      if (item) bindCard(card, item);
    });
  }

  function setStatus(kind, msg) {
    const el = document.getElementById(statusId);
    if (kind === 'loading' && state.items.length > 0) {
      el.innerHTML = '';
    } else if (kind === 'empty') {
      const searching = Boolean(state.q);
      let iconKind;
      let title;
      let hint;
      if (searching) {
        iconKind = 'search';
        title = '没有匹配结果';
        hint = '试试其他关键词';
      } else {
        const copy = resolveEmptyCopy();
        iconKind = copy.icon || 'library';
        title = copy.title;
        hint = copy.hint || '';
      }
      const hintHtml = hint
        ? `<p class="empty-hint">${escapeHtml(hint)}</p>`
        : '';
      const icon = emptyIcon(iconKind);
      el.innerHTML = `
        <div class="empty-state">
          <div class="empty-icon" aria-hidden="true">${icon}</div>
          <p class="empty-title">${escapeHtml(title)}</p>
          ${hintHtml}
        </div>`;
    } else if (kind === 'error') {
      el.innerHTML = '<p class="form-error">' + escapeHtml(msg) + '</p>';
    } else {
      el.innerHTML = '';
    }
  }

  function findCard(id) {
    const grid = document.getElementById(gridId);
    return grid?.querySelector(`[data-id="${CSS.escape(id)}"]`);
  }

  function removeItem(id) {
    const idx = state.items.findIndex((i) => i.id === id);
    if (idx >= 0) state.items.splice(idx, 1);
    const card = findCard(id);
    if (card) card.remove();
    if (state.items.length === 0) setStatus('empty');
    return { action: 'removed' };
  }

  function syncItem(item) {
    if (!item) return { action: 'none' };

    if (!matchesItem(item)) {
      return removeItem(item.id);
    }

    const idx = state.items.findIndex((i) => i.id === item.id);
    const card = findCard(item.id);

    if (idx >= 0) {
      const prev = state.items[idx];
      state.items[idx] = item;
      return { action: 'updated', card, item, prev };
    }

    return { action: 'none', item };
  }

  function init() {
    if (searchInputId) {
      document.getElementById(searchInputId).addEventListener('input', debounce(() => {
        state.q = document.getElementById(searchInputId).value.trim();
        resetAndLoad();
      }, 300));
    }
    setupObserver();
    loadMore();
  }

  function destroy() {
    if (observer) {
      observer.disconnect();
      observer = null;
    }
  }

  function setExtraQuery(next) {
    const merged = next ? { ...next } : {};
    const same = Object.keys(merged).length === Object.keys(extraQuery).length
      && Object.keys(merged).every((k) => merged[k] === extraQuery[k]);
    if (same) return;
    extraQuery = merged;
    resetAndLoad();
  }

  return {
    init,
    destroy,
    resetAndLoad,
    setExtraQuery,
    setStatus,
    syncItem,
    removeItem,
    matchesItem,
    getState: () => state,
    getGrid: () => document.getElementById(gridId),
    getStatusEl: () => document.getElementById(statusId),
  };
}

function emptyIcon(kind) {
  if (kind === 'trash') {
    return `
      <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="14" y="20" width="36" height="34" rx="4" stroke="currentColor" stroke-width="2.5"/>
        <path d="M24 20V16a4 4 0 0 1 4-4h8a4 4 0 0 1 4 4v4" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
        <path d="M10 20h44" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
        <path d="M26 30v14M32 30v14M38 30v14" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
      </svg>`;
  }
  if (kind === 'search') {
    return `
      <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
        <circle cx="28" cy="28" r="14" stroke="currentColor" stroke-width="2.5"/>
        <path d="M38 38l12 12" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
      </svg>`;
  }
  if (kind === 'star') {
    return `
      <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M32 6L39.6 21.4 56.4 23.8 44.2 35.6 47.1 52.3 32 44.3 16.9 52.3 19.8 35.6 7.6 23.8 24.4 21.4Z" stroke="currentColor" stroke-width="2.5" stroke-linejoin="round"/>
      </svg>`;
  }
  // library / default — stacked frames
  return `
    <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
      <rect x="12" y="18" width="34" height="28" rx="3" stroke="currentColor" stroke-width="2.5"/>
      <rect x="18" y="12" width="34" height="28" rx="3" stroke="currentColor" stroke-width="2.5" opacity="0.45"/>
      <path d="M20 30h18M20 36h12" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
    </svg>`;
}
