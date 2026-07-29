/**
 * Settings — password, API keys, browser, server info.
 */

import {
  changePassword,
  listKeys,
  createKey,
  revokeKey,
  getSettingsInfo,
  getBrowserStatus,
  installBrowser,
} from './api.js';
import { showToast, escapeHtml, formatDate } from './util.js';
import { INSTALL_STATUS_LABELS } from './enums.js';

let browserPollTimer = null;

export function renderSettings(main) {
  main.innerHTML = `
    <div class="page-header"><h1>设置</h1></div>
    <div class="settings-stack">
      <section class="settings-block">
        <h2>服务</h2>
        <dl class="info-list" id="info-list"><p class="loading-more">…</p></dl>
      </section>

      <section class="settings-block">
        <h2>密码</h2>
        <form id="password-form">
          <div class="form-group">
            <label for="old-password">当前</label>
            <input type="password" id="old-password" required autocomplete="current-password">
          </div>
          <div class="form-group">
            <label for="new-password">新密码</label>
            <input type="password" id="new-password" required minlength="6" autocomplete="new-password">
          </div>
          <p id="password-error" class="form-error hidden"></p>
          <button type="submit" class="btn btn-primary">更新</button>
        </form>
      </section>

      <section class="settings-block">
        <h2>API Key</h2>
        <form id="key-form" class="key-form-row">
          <input type="text" id="key-name" placeholder="名称" required>
          <button type="submit" class="btn btn-primary">创建</button>
        </form>
        <div id="key-reveal" class="hidden"></div>
        <ul class="key-list" id="key-list"><li class="loading-more">…</li></ul>
      </section>

      <section class="settings-block">
        <h2>浏览器</h2>
        <div id="browser-status"><p class="loading-more">…</p></div>
        <button type="button" class="btn btn-secondary" id="btn-install">安装 Chrome</button>
      </section>
    </div>
  `;

  document.getElementById('password-form').addEventListener('submit', handlePassword);
  document.getElementById('key-form').addEventListener('submit', handleCreateKey);
  document.getElementById('btn-install').addEventListener('click', handleInstall);

  loadInfo();
  loadKeys();
  loadBrowser();
}

async function loadInfo() {
  try {
    const info = await getSettingsInfo();
    const lan = Array.isArray(info.lan_urls) ? info.lan_urls : [];
    const lanHtml = lan.length
      ? lan.map((u) => `<div class="info-row"><dt>局域网</dt><dd><a href="${escapeHtml(u)}/" target="_blank" rel="noopener">${escapeHtml(u)}/</a></dd></div>`).join('')
      : '';
    document.getElementById('info-list').innerHTML = `
      <div class="info-row"><dt>数据</dt><dd>${escapeHtml(info.data_dir)}</dd></div>
      <div class="info-row"><dt>监听</dt><dd>${escapeHtml(info.host)}:${info.port}</dd></div>
      <div class="info-row"><dt>本机</dt><dd><a href="${escapeHtml(info.local_url || '#')}" target="_blank" rel="noopener">${escapeHtml(info.local_url || '')}</a></dd></div>
      ${lanHtml}
    `;
  } catch (err) {
    document.getElementById('info-list').innerHTML = '<p class="form-error">' + escapeHtml(err.message) + '</p>';
  }
}

async function handlePassword(e) {
  e.preventDefault();
  const errEl = document.getElementById('password-error');
  errEl.classList.add('hidden');
  try {
    await changePassword(
      document.getElementById('old-password').value,
      document.getElementById('new-password').value,
    );
    showToast('已更新');
    e.target.reset();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('hidden');
  }
}

async function loadKeys() {
  const list = document.getElementById('key-list');
  try {
    const keys = await listKeys();
    if (!keys.length) {
      list.innerHTML = '';
      return;
    }
    list.innerHTML = keys.map((k) => `
      <li class="key-item" data-id="${escapeHtml(k.id)}">
        <div class="key-item-info">
          <div class="key-item-name">${escapeHtml(k.name)}</div>
          <div class="key-item-prefix">${escapeHtml(k.key_prefix)} · ${formatDate(k.created_at)}</div>
        </div>
        <button type="button" class="btn btn-danger btn-revoke">吊销</button>
      </li>
    `).join('');

    list.querySelectorAll('.btn-revoke').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const id = btn.closest('.key-item').dataset.id;
        if (!confirm('吊销此 Key？')) return;
        try {
          await revokeKey(id);
          showToast('已吊销');
          loadKeys();
        } catch (err) {
          showToast(err.message);
        }
      });
    });
  } catch (err) {
    list.innerHTML = '<li class="form-error">' + escapeHtml(err.message) + '</li>';
  }
}

async function handleCreateKey(e) {
  e.preventDefault();
  const name = document.getElementById('key-name').value.trim();
  if (!name) return;
  try {
    const created = await createKey(name);
    const reveal = document.getElementById('key-reveal');
    reveal.classList.remove('hidden');
    reveal.className = 'key-reveal';
    reveal.innerHTML = `
      <code id="new-key-value">${escapeHtml(created.key)}</code>
      <button type="button" class="btn btn-secondary" id="btn-copy-key">复制</button>
    `;
    document.getElementById('btn-copy-key').addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(created.key);
        showToast('已复制');
      } catch {
        showToast('复制失败');
      }
    });
    document.getElementById('key-name').value = '';
    loadKeys();
  } catch (err) {
    showToast(err.message);
  }
}

function renderBrowserStatus(data) {
  const available = data.available;
  let installBit = '';
  if (data.install) {
    const label = INSTALL_STATUS_LABELS[data.install.status] || data.install.status;
    installBit = `<span class="status-badge warn">${escapeHtml(label)}</span>`;
  }
  return `
    <div class="browser-line">
      <span class="status-dot ${available ? 'ok' : 'warn'}">${available ? '可用' : '未就绪'}</span>
      ${installBit}
    </div>
    ${data.path ? `<div class="browser-path">${escapeHtml(data.path)}</div>` : ''}
  `;
}

async function loadBrowser() {
  const el = document.getElementById('browser-status');
  const btn = document.getElementById('btn-install');
  try {
    const data = await getBrowserStatus();
    el.innerHTML = renderBrowserStatus(data);
    const installing = data.install && ['pending', 'installing'].includes(data.install.status);
    btn.disabled = installing || (data.available && !data.install);
    if (installing) startBrowserPoll();
    else stopBrowserPoll();
  } catch (err) {
    el.innerHTML = '<p class="form-error">' + escapeHtml(err.message) + '</p>';
  }
}

function startBrowserPoll() {
  stopBrowserPoll();
  browserPollTimer = setInterval(loadBrowser, 2000);
}

function stopBrowserPoll() {
  if (browserPollTimer) {
    clearInterval(browserPollTimer);
    browserPollTimer = null;
  }
}

async function handleInstall() {
  const btn = document.getElementById('btn-install');
  btn.disabled = true;
  try {
    await installBrowser();
    showToast('安装中');
    startBrowserPoll();
    loadBrowser();
  } catch (err) {
    showToast(err.message);
    btn.disabled = false;
  }
}

export function destroySettings() {
  stopBrowserPoll();
}
