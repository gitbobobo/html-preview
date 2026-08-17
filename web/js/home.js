/**
 * Home — brand + search + grid; whole page is drop target.
 */

import { formatDate, showToast } from './util.js';
import { getItem, favoriteItem, unfavoriteItem } from './api.js';
import { applyItemToCard } from './thumb.js';
import { tileCardHTML, applyFavoriteToCard } from './tile.js';
import { STAR_ICON } from './icons.js';
import { openDrawer } from './drawer.js';
import { createItemListPage } from './item-list.js';
import { uploadFile } from './upload.js';

let list = null;
let dragDepth = 0;
const screenshotWatchers = new Map();

/** Parse a single root element from an HTML string. */
function htmlToElement(html) {
  const template = document.createElement('template');
  template.innerHTML = html.trim();
  return template.content.firstElementChild;
}

function homeEmptyCopy() {
  if (readFavoriteParam()) {
    return {
      icon: 'star',
      title: '还没有收藏',
      hint: '点亮卡片右上角的星标后，会显示在这里',
    };
  }
  return {
    icon: 'library',
    title: '还没有预览',
    hint: '点击右上角上传，或拖拽文件到此处',
  };
}

/** Called after a successful upload (header or drop). */
export function prependUploadedItem(item) {
  if (!list || !list.matchesItem(item)) return false;
  const state = list.getState();
  state.items.unshift(item);
  const grid = list.getGrid();
  if (!grid) return false;
  const card = htmlToElement(tileCardHTML(item));
  card.dataset.bound = '1';
  bindHomeCard(card);
  grid.prepend(card);
  list.getStatusEl().innerHTML = '';
  watchScreenshot(item.id);
  return true;
}

function applyDrawerUpdate(card, updated) {
  const result = list.syncItem(updated);
  if (result.action === 'removed') {
    stopWatch(card.dataset.id);
    return;
  }
  if (result.action !== 'updated') return;
  applyItemToCard(card, updated);
  if (result.prev.favorite !== updated.favorite) {
    applyFavoriteToCard(card, updated.favorite);
  }
  const meta = card.querySelector('.tile-meta');
  if (meta && updated.updated_at) meta.textContent = formatDate(updated.updated_at);
  if (updated.screenshot_status === 'pending') {
    watchScreenshot(updated.id);
  }
}

function bindHomeCard(card) {
  const favBtn = card.querySelector('.tile-fav');
  if (favBtn) {
    favBtn.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      toggleFavoriteCard(card);
    });
  }

  const openEdit = (e) => {
    e.stopPropagation();
    openDrawer(card.dataset.id, (updated) => {
      if (updated === null) {
        stopWatch(card.dataset.id);
        list.removeItem(card.dataset.id);
        return;
      }
      applyDrawerUpdate(card, updated);
    });
  };

  const onKeyDown = (e) => {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    if (e.target !== card) return;
    e.preventDefault();
    openEdit(e);
  };

  card.addEventListener('click', openEdit);
  card.addEventListener('keydown', onKeyDown);
}

/** Star/unstar from the tile badge — optimistic, reverted + toasted on failure. */
async function toggleFavoriteCard(card) {
  const state = list?.getState();
  const idx = state ? state.items.findIndex((i) => i.id === card.dataset.id) : -1;
  if (idx < 0) return;
  const before = state.items[idx];
  applyFavoriteToCard(card, !before.favorite);
  try {
    const updated = before.favorite
      ? await unfavoriteItem(before.id)
      : await favoriteItem(before.id);
    const result = list.syncItem(updated);
    if (result.action === 'removed') {
      stopWatch(before.id);
    } else if (result.action === 'updated') {
      applyFavoriteToCard(card, updated.favorite);
    }
  } catch (err) {
    applyFavoriteToCard(card, before.favorite);
    showToast(err.message);
  }
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
  setTimeout(tick, 800);
}

/** Only the literal "true" counts — same rule as the backend. */
function readFavoriteParam() {
  return new URLSearchParams(location.search).get('favorite') === 'true';
}

/** Keep the filter shareable/refreshable via the URL (no localStorage). */
function writeFavoriteParam(on) {
  const url = new URL(location.href);
  if (on) url.searchParams.set('favorite', 'true');
  else url.searchParams.delete('favorite');
  history.replaceState(null, '', url);
}

function setFavoriteFilter(on) {
  writeFavoriteParam(on);
  const btn = document.getElementById('home-favorite-toggle');
  if (btn) {
    btn.classList.toggle('active', on);
    btn.setAttribute('aria-pressed', on ? 'true' : 'false');
  }
  list?.setExtraQuery(on ? { favorite: true } : {});
}

export function renderHome(main) {
  const favoriteOnly = readFavoriteParam();
  main.innerHTML = `
    <section class="home-stage">
      <h1 class="brand-mark">HTML Preview</h1>
      <div class="home-controls">
        <div class="search-wrap">
          <input type="search" id="home-search" placeholder="搜索" autocomplete="off">
        </div>
        <button type="button" class="filter-toggle${favoriteOnly ? ' active' : ''}" id="home-favorite-toggle" aria-pressed="${favoriteOnly}" title="仅看收藏">
          ${STAR_ICON}<span>仅看收藏</span>
        </button>
      </div>
    </section>
    <div class="grid" id="item-grid"></div>
    <div class="scroll-sentinel" id="scroll-sentinel"></div>
    <div id="home-status"></div>
  `;

  list = createItemListPage({
    status: 'active',
    extraQuery: favoriteOnly ? { favorite: true } : {},
    matchesItem: (item) => !readFavoriteParam() || item.favorite,
    gridId: 'item-grid',
    sentinelId: 'scroll-sentinel',
    statusId: 'home-status',
    searchInputId: 'home-search',
    emptyText: homeEmptyCopy,
    renderCard: tileCardHTML,
    bindCard: (card, item) => {
      bindHomeCard(card);
      if (item?.screenshot_status === 'pending') {
        watchScreenshot(item.id);
      }
    },
  });

  document.getElementById('home-favorite-toggle').addEventListener('click', (e) => {
    setFavoriteFilter(e.currentTarget.getAttribute('aria-pressed') !== 'true');
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
