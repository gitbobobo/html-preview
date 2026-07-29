/**
 * Trash — restore / permanent delete.
 */

import { restoreItem, permanentDeleteItem } from './api.js';
import { showToast, escapeHtml, formatDate } from './util.js';
import { renderCardThumb } from './thumb.js';
import { createItemListPage } from './item-list.js';

let list = null;

export function renderTrash(main) {
  main.innerHTML = `
    <div class="page-header"><h1>回收站</h1></div>
    <div class="page-toolbar">
      <div class="search-wrap">
        <input type="search" id="trash-search" placeholder="搜索" autocomplete="off">
      </div>
    </div>
    <div class="grid" id="trash-grid"></div>
    <div class="scroll-sentinel" id="trash-sentinel"></div>
    <div id="trash-status"></div>
  `;

  list = createItemListPage({
    status: 'trash',
    gridId: 'trash-grid',
    sentinelId: 'trash-sentinel',
    statusId: 'trash-status',
    searchInputId: 'trash-search',
    emptyText: {
      icon: 'trash',
      title: '回收站是空的',
      hint: '删除的项目会暂存在这里',
    },
    renderCard: (item) => `
      <article class="tile" data-id="${escapeHtml(item.id)}" style="cursor:default;">
        <div class="tile-media">${renderCardThumb(item)}</div>
        <div class="tile-caption">
          <h3 class="tile-title">${escapeHtml(item.title || '未命名')}</h3>
          <p class="tile-meta">${formatDate(item.trashed_at)}</p>
        </div>
        <div class="tile-actions">
          <button type="button" class="btn btn-secondary btn-restore">恢复</button>
          <button type="button" class="btn btn-danger btn-delete">删除</button>
        </div>
      </article>
    `,
    bindCard: (card) => {
      card.querySelector('.btn-restore').addEventListener('click', (e) => {
        e.stopPropagation();
        handleRestore(card);
      });
      card.querySelector('.btn-delete').addEventListener('click', (e) => {
        e.stopPropagation();
        handleDelete(card);
      });
    },
  });

  list.init();
}

async function handleRestore(card) {
  const id = card.dataset.id;
  try {
    await restoreItem(id);
    showToast('已恢复');
    card.remove();
    const state = list.getState();
    state.items = state.items.filter((i) => i.id !== id);
    if (state.items.length === 0) list.setStatus('empty');
  } catch (err) {
    showToast(err.message);
  }
}

async function handleDelete(card) {
  const id = card.dataset.id;
  const title = card.querySelector('.tile-title')?.textContent || '此项';
  if (!confirm('彻底删除「' + title + '」？')) return;
  try {
    await permanentDeleteItem(id);
    showToast('已删除');
    card.remove();
    const state = list.getState();
    state.items = state.items.filter((i) => i.id !== id);
    if (state.items.length === 0) list.setStatus('empty');
  } catch (err) {
    showToast(err.message);
  }
}

export function destroyTrash() {
  if (list) {
    list.destroy();
    list = null;
  }
}
