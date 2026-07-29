/**
 * Home — brand + search + grid; whole page is drop target.
 */

import { escapeHtml, formatDate } from './util.js';
import { getItem } from './api.js';
import { renderCardThumb, applyItemToCard } from './thumb.js';
import { openDrawer } from './drawer.js';
import { createItemListPage } from './item-list.js';
import { uploadFile } from './upload.js';

let list = null;
let dragDepth = 0;
const screenshotWatchers = new Map();

/** Called after a successful upload (header or drop). */
export function prependUploadedItem(item) {
  if (!list) return false;
  const state = list.getState();
  state.items.unshift(item);
  const grid = list.getGrid();
  if (!grid) return false;
  const card = document.createElement('article');
  card.className = 'tile';
  card.dataset.id = item.id;
  card.dataset.bound = '1';
  card.innerHTML = `
    <a class="tile-media" href="/c/${escapeHtml(item.id)}/" target="_blank" rel="noopener">${renderCardThumb(item)}</a>
    <div class="tile-caption">
      <div class="tile-caption-row">
        <h3 class="tile-title">${escapeHtml(item.title || '未命名')}</h3>
        <button type="button" class="btn-tile-edit" aria-label="编辑">编辑</button>
      </div>
      <p class="tile-meta">${formatDate(item.updated_at)}</p>
    </div>`;
  bindHomeCard(card);
  grid.prepend(card);
  list.getStatusEl().innerHTML = '';
  watchScreenshot(item.id);
  return true;
}

function bindHomeCard(card) {
  const openEdit = (e) => {
    e.preventDefault();
    e.stopPropagation();
    openDrawer(card.dataset.id, (updated) => {
      const state = list.getState();
      if (updated === null) {
        stopWatch(card.dataset.id);
        card.remove();
        state.items = state.items.filter((i) => i.id !== card.dataset.id);
        return;
      }
      const idx = state.items.findIndex((i) => i.id === updated.id);
      if (idx >= 0) state.items[idx] = updated;
      applyItemToCard(card, updated);
      const meta = card.querySelector('.tile-meta');
      if (meta && updated.updated_at) meta.textContent = formatDate(updated.updated_at);
      if (updated.screenshot_status === 'pending') {
        watchScreenshot(updated.id);
      }
    });
  };
  card.querySelector('.btn-tile-edit')?.addEventListener('click', openEdit);
}

function stopWatch(id) {
  const t = screenshotWatchers.get(id);
  if (t) {
    clearInterval(t);
    screenshotWatchers.delete(id);
  }
}

function watchScreenshot(id) {
  stopWatch(id);
  let tries = 0;
  const tick = async () => {
    tries += 1;
    if (tries > 60) {
      stopWatch(id);
      return;
    }
    try {
      const item = await getItem(id);
      const state = list?.getState();
      if (state) {
        const idx = state.items.findIndex((i) => i.id === id);
        if (idx >= 0) state.items[idx] = item;
      }
      const card = list?.getGrid()?.querySelector(`[data-id="${CSS.escape(id)}"]`);
      if (card) applyItemToCard(card, item);

      if (item.screenshot_status !== 'pending') {
        stopWatch(id);
      }
    } catch {
      /* keep trying a bit */
    }
  };
  const timer = setInterval(tick, 1500);
  screenshotWatchers.set(id, timer);
  // first check soon — screenshot often finishes in a few seconds
  setTimeout(tick, 800);
}

export function renderHome(main) {
  main.innerHTML = `
    <section class="home-stage">
      <h1 class="brand-mark">HTML Preview</h1>
      <div class="home-controls">
        <div class="search-wrap">
          <input type="search" id="home-search" placeholder="搜索" autocomplete="off">
        </div>
      </div>
    </section>
    <div class="grid" id="item-grid"></div>
    <div class="scroll-sentinel" id="scroll-sentinel"></div>
    <div id="home-status"></div>
  `;

  list = createItemListPage({
    status: 'active',
    gridId: 'item-grid',
    sentinelId: 'scroll-sentinel',
    statusId: 'home-status',
    searchInputId: 'home-search',
    emptyText: {
      icon: 'library',
      title: '还没有预览',
      hint: '点击右上角上传，或拖拽文件到此处',
    },
    renderCard: (item) => `
      <article class="tile" data-id="${escapeHtml(item.id)}">
        <a class="tile-media" href="/c/${escapeHtml(item.id)}/" target="_blank" rel="noopener">${renderCardThumb(item)}</a>
        <div class="tile-caption">
          <div class="tile-caption-row">
            <h3 class="tile-title">${escapeHtml(item.title || '未命名')}</h3>
            <button type="button" class="btn-tile-edit" aria-label="编辑">编辑</button>
          </div>
          <p class="tile-meta">${formatDate(item.updated_at)}</p>
        </div>
      </article>
    `,
    bindCard: (card, item) => {
      bindHomeCard(card);
      if (item?.screenshot_status === 'pending') {
        watchScreenshot(item.id);
      }
    },
  });

  setupPageDrop(main);
  list.init();
}

function setupPageDrop(main) {
  const onEnter = (e) => {
    e.preventDefault();
    dragDepth += 1;
    main.classList.add('drop-active');
  };
  const onLeave = (e) => {
    e.preventDefault();
    dragDepth = Math.max(0, dragDepth - 1);
    if (dragDepth === 0) main.classList.remove('drop-active');
  };
  const onOver = (e) => e.preventDefault();
  const onDrop = (e) => {
    e.preventDefault();
    dragDepth = 0;
    main.classList.remove('drop-active');
    const file = e.dataTransfer?.files?.[0];
    if (file) uploadFile(file, { onSuccess: prependUploadedItem });
  };

  main.addEventListener('dragenter', onEnter);
  main.addEventListener('dragleave', onLeave);
  main.addEventListener('dragover', onOver);
  main.addEventListener('drop', onDrop);
  main._dropCleanup = () => {
    main.removeEventListener('dragenter', onEnter);
    main.removeEventListener('dragleave', onLeave);
    main.removeEventListener('dragover', onOver);
    main.removeEventListener('drop', onDrop);
    main.classList.remove('drop-active');
    delete main._dropCleanup;
  };
}

export function destroyHome() {
  const main = document.getElementById('main');
  if (main?._dropCleanup) main._dropCleanup();
  dragDepth = 0;
  for (const id of [...screenshotWatchers.keys()]) stopWatch(id);
  if (list) {
    list.destroy();
    list = null;
  }
}
