/**
 * App shell — routing, auth guard, page lifecycle.
 */

import { getAuthStatus, setup, login, logout } from './api.js';
import { initDrawer } from './drawer.js';
import { renderHome, destroyHome, prependUploadedItem } from './home.js';
import { renderTrash, destroyTrash } from './trash.js';
import { renderSettings, destroySettings } from './settings.js';
import { pickAndUpload } from './upload.js';

const AUTH_ROUTES = ['/setup', '/login'];
const APP_ROUTES = ['/', '/trash', '/settings'];

let authState = { initialized: false, authenticated: false };
let currentRoute = null;
let pageDestroy = null;

const appEl = document.getElementById('app');
const authView = document.getElementById('auth-view');
const mainEl = document.getElementById('main');

async function bootstrap() {
  initDrawer();
  bindGlobalEvents();
  await refreshAuth();
  navigate(location.pathname, true);
  window.addEventListener('popstate', () => navigate(location.pathname, true));
}

async function refreshAuth() {
  try {
    authState = await getAuthStatus();
  } catch {
    authState = { initialized: false, authenticated: false };
  }
}

function bindGlobalEvents() {
  document.querySelectorAll('[data-link]').forEach((a) => {
    a.addEventListener('click', (e) => {
      const href = a.getAttribute('href');
      if (!href || href.startsWith('http')) return;
      e.preventDefault();
      navigate(href);
    });
  });

  document.getElementById('btn-logout').addEventListener('click', async () => {
    try {
      await logout();
    } catch {
      /* ignore */
    }
    authState.authenticated = false;
    navigate('/login');
  });

  document.getElementById('btn-upload').addEventListener('click', () => {
    pickAndUpload({
      onSuccess: (item) => {
        if (currentRoute === '/') {
          prependUploadedItem(item);
        } else {
          navigate('/');
        }
      },
    });
  });
}

function navigate(path, replace = false) {
  path = normalizePath(path);

  if (!AUTH_ROUTES.includes(path) && !APP_ROUTES.includes(path)) {
    path = '/';
  }

  if (!authState.initialized) {
    path = '/setup';
  } else if (!authState.authenticated && !AUTH_ROUTES.includes(path)) {
    path = '/login';
  } else if (authState.authenticated && AUTH_ROUTES.includes(path)) {
    path = '/';
  }

  if (path !== location.pathname) {
    if (replace) {
      history.replaceState(null, '', path);
    } else {
      history.pushState(null, '', path);
    }
  }

  renderRoute(path);
}

function normalizePath(path) {
  if (!path || path === '') return '/';
  const q = path.indexOf('?');
  if (q >= 0) path = path.slice(0, q);
  if (path.length > 1 && path.endsWith('/')) path = path.slice(0, -1);
  return path;
}

function renderRoute(path) {
  if (currentRoute === path) {
    updateNav(path);
    return;
  }

  if (pageDestroy) {
    pageDestroy();
    pageDestroy = null;
  }

  currentRoute = path;

  if (AUTH_ROUTES.includes(path)) {
    appEl.classList.add('hidden');
    authView.classList.remove('hidden');
    renderAuth(path);
    return;
  }

  authView.classList.add('hidden');
  appEl.classList.remove('hidden');
  updateNav(path);

  if (path === '/') {
    renderHome(mainEl);
    pageDestroy = destroyHome;
  } else if (path === '/trash') {
    renderTrash(mainEl);
    pageDestroy = destroyTrash;
  } else if (path === '/settings') {
    renderSettings(mainEl);
    pageDestroy = destroySettings;
  }
}

function updateNav(path) {
  document.querySelectorAll('.nav-link').forEach((link) => {
    link.classList.toggle('active', link.dataset.route === path);
  });
}

function renderAuth(path) {
  if (path === '/setup') {
    authView.innerHTML = `
      <div class="auth-panel">
        <h1 class="brand-mark">HTML Preview</h1>
        <form id="auth-form">
          <div class="form-group">
            <label for="auth-password">密码</label>
            <input type="password" id="auth-password" required minlength="6" autocomplete="new-password">
          </div>
          <div class="form-group">
            <label for="auth-password2">确认</label>
            <input type="password" id="auth-password2" required minlength="6" autocomplete="new-password">
          </div>
          <p id="auth-error" class="form-error hidden"></p>
          <button type="submit" class="btn btn-primary">开始</button>
        </form>
      </div>`;
  } else {
    authView.innerHTML = `
      <div class="auth-panel">
        <h1 class="brand-mark">HTML Preview</h1>
        <form id="auth-form">
          <div class="form-group">
            <label for="auth-password">密码</label>
            <input type="password" id="auth-password" required autocomplete="current-password">
          </div>
          <p id="auth-error" class="form-error hidden"></p>
          <button type="submit" class="btn btn-primary">进入</button>
        </form>
      </div>`;
  }

  document.getElementById('auth-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const errEl = document.getElementById('auth-error');
    errEl.classList.add('hidden');
    const pw = document.getElementById('auth-password').value;

    if (path === '/setup') {
      const pw2 = document.getElementById('auth-password2').value;
      if (pw !== pw2) {
        errEl.textContent = '密码不一致';
        errEl.classList.remove('hidden');
        return;
      }
    }

    try {
      if (path === '/setup') {
        await setup(pw);
        authState = { initialized: true, authenticated: true };
      } else {
        await login(pw);
        authState = { initialized: true, authenticated: true };
      }
      navigate('/', true);
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  });
}

bootstrap();
