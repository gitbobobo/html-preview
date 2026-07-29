/**
 * Item drawer — edit metadata, copy link, replace, trash.
 */

import {
  getItem,
  patchItem,
  replaceItemContent,
  trashItem,
} from './api.js';
import { showToast, escapeHtml } from './util.js';
import { renderThumbPicture } from './thumb.js';
import { EXPIRES_PRESETS, SCREENSHOT_STATUS_LABELS } from './enums.js';

let currentId = null;
let onUpdate = null;

/** RFC3339 → datetime-local（本地时区，分钟精度）。 */
function toDatetimeLocalValue(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** datetime-local → RFC3339（UTC，带 Z）。 */
function toRFC3339(local) {
  if (!local) return '';
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return '';
  return d.toISOString();
}

const overlay = () => document.getElementById('drawer-overlay');
const body = () => document.getElementById('drawer-body');
const titleEl = () => document.getElementById('drawer-title');

export function initDrawer() {
  document.getElementById('drawer-close').addEventListener('click', closeDrawer);
  overlay().addEventListener('click', (e) => {
    if (e.target === overlay()) closeDrawer();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && !overlay().classList.contains('hidden')) {
      closeDrawer();
    }
  });
}

export function openDrawer(id, callback) {
  currentId = id;
  onUpdate = callback;
  overlay().classList.remove('hidden');
  overlay().setAttribute('aria-hidden', 'false');
  document.body.style.overflow = 'hidden';
  loadDrawer(id);
}

export function closeDrawer() {
  overlay().classList.add('hidden');
  overlay().setAttribute('aria-hidden', 'true');
  document.body.style.overflow = '';
  currentId = null;
  onUpdate = null;
}

async function loadDrawer(id) {
  body().innerHTML = '<p class="loading-more">加载中…</p>';
  try {
    const item = await getItem(id);
    titleEl().textContent = item.title || '未命名';
    body().innerHTML = renderDrawerContent(item);
    bindDrawerEvents(item);
    // Sync latest status/thumbs back to grid while drawer is open.
    if (typeof onUpdate === 'function') onUpdate(item);
  } catch (err) {
    body().innerHTML = '<p class="form-error">' + escapeHtml(err.message) + '</p>';
  }
}

function renderDrawerContent(item) {
  const publicUrl = location.origin + item.public_path;
  const ss = item.screenshot_status;
  const ssLabel = SCREENSHOT_STATUS_LABELS[ss] || ss;

  let ssClass = 'warn';
  if (ss === 'ready') ssClass = 'ok';
  if (ss === 'failed') ssClass = 'err';

  const expiresVal = toDatetimeLocalValue(item.expires_at);

  return `
    <div class="drawer-preview">
      <a class="card-thumb" href="${escapeHtml(publicUrl)}" target="_blank" rel="noopener">${renderThumbPicture(item)}</a>
    </div>

    <div class="screenshot-status">
      <span class="status-badge ${ssClass}">${escapeHtml(ssLabel)}</span>
      ${item.screenshot_error ? '<span class="form-error"> ' + escapeHtml(item.screenshot_error) + '</span>' : ''}
    </div>

    <form id="drawer-form">
      <div class="form-group">
        <label for="drawer-title-input">标题</label>
        <input type="text" id="drawer-title-input" value="${escapeHtml(item.title)}" required>
      </div>
      <div class="form-group">
        <label for="drawer-notes">备注</label>
        <textarea id="drawer-notes">${escapeHtml(item.notes || '')}</textarea>
      </div>
      <div class="form-group">
        <label for="drawer-expires">有效期</label>
        <select id="drawer-expires">
          ${EXPIRES_PRESETS.map((p) => {
            const selected = p.value === 'custom'
              ? !!item.expires_at
              : p.value === 'never'
                ? !item.expires_at
                : false;
            return `<option value="${p.value}" ${selected ? 'selected' : ''}>${p.label}</option>`;
          }).join('')}
        </select>
      </div>
      <div class="form-group ${item.expires_at ? '' : 'hidden'}" id="drawer-expires-custom-wrap">
        <label for="drawer-expires-at">到期</label>
        <input type="datetime-local" id="drawer-expires-at" value="${escapeHtml(expiresVal)}">
      </div>
      <p id="drawer-form-error" class="form-error hidden"></p>
      <button type="submit" class="btn btn-primary">保存</button>
    </form>

    <div class="drawer-actions">
      <a class="btn btn-primary" id="drawer-open-preview" href="${escapeHtml(publicUrl)}" target="_blank" rel="noopener">打开预览</a>
      <button type="button" class="btn btn-secondary" id="drawer-copy-link">复制链接</button>
      <button type="button" class="btn btn-secondary" id="drawer-replace">替换</button>
      <button type="button" class="btn btn-danger" id="drawer-trash">回收站</button>
    </div>
    <input type="file" id="drawer-file-input" class="sr-only" accept=".html,.htm,.zip">
  `;
}

function bindDrawerEvents(item) {
  const publicUrl = location.origin + item.public_path;
  const expiresSelect = document.getElementById('drawer-expires');
  const customWrap = document.getElementById('drawer-expires-custom-wrap');
  const expiresAtInput = document.getElementById('drawer-expires-at');
  const initialExpiry = {
    mode: item.expires_at ? 'custom' : 'never',
    customLocal: toDatetimeLocalValue(item.expires_at),
  };

  expiresSelect.addEventListener('change', () => {
    customWrap.classList.toggle('hidden', expiresSelect.value !== 'custom');
  });

  document.getElementById('drawer-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = document.getElementById('drawer-form-error');
    errEl.classList.add('hidden');
    try {
      const payload = {
        title: document.getElementById('drawer-title-input').value.trim(),
        notes: document.getElementById('drawer-notes').value,
      };
      const exp = expiresSelect.value;
      const customLocal = expiresAtInput.value;
      const expiryDirty =
        exp !== initialExpiry.mode ||
        (exp === 'custom' && customLocal !== initialExpiry.customLocal);

      if (expiryDirty) {
        if (exp === 'custom') {
          const rfc = toRFC3339(customLocal);
          if (!rfc) {
            errEl.textContent = '请填写有效的到期时间';
            errEl.classList.remove('hidden');
            return;
          }
          payload.expires_at = rfc;
        } else {
          payload.expires_in = exp;
        }
      }
      const updated = await patchItem(item.id, payload);
      titleEl().textContent = updated.title || '未命名';
      showToast('已保存');
      if (onUpdate) onUpdate(updated);
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });

  document.getElementById('drawer-copy-link').addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(publicUrl);
      showToast('链接已复制');
    } catch {
      showToast('复制失败，请手动复制：' + publicUrl);
    }
  });

  const fileInput = document.getElementById('drawer-file-input');
  document.getElementById('drawer-replace').addEventListener('click', () => fileInput.click());
  fileInput.addEventListener('change', async () => {
    const file = fileInput.files[0];
    if (!file) return;
    try {
      const updated = await replaceItemContent(item.id, file);
      showToast('已替换');
      loadDrawer(updated.id);
      if (onUpdate) onUpdate(updated);
    } catch (err) {
      showToast(err.message);
    }
    fileInput.value = '';
  });

  document.getElementById('drawer-trash').addEventListener('click', async () => {
    if (!confirm('确定将此项目移到回收站？')) return;
    try {
      await trashItem(item.id);
      showToast('已移到回收站');
      closeDrawer();
      if (onUpdate) onUpdate(null);
    } catch (err) {
      showToast(err.message);
    }
  });
}
