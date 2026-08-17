/**
 * API client — all requests use credentials (session cookie).
 */

const API_BASE = '/api';

export class ApiError extends Error {
  constructor(code, message) {
    super(message);
    this.code = code;
    this.name = 'ApiError';
  }
}

async function parseResponse(res) {
  const text = await res.text();
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    throw new ApiError(50000, 'invalid response');
  }
  if (json.code !== 0) {
    throw new ApiError(json.code, json.message || 'request failed');
  }
  return json.data;
}

export async function api(path, options = {}) {
  const headers = { ...options.headers };
  const hasBody = options.body !== undefined && options.body !== null;
  if (hasBody && !(options.body instanceof FormData) && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json';
  }

  const res = await fetch(API_BASE + path, {
    credentials: 'include',
    ...options,
    headers,
  });

  return parseResponse(res);
}

export async function apiJSON(method, path, body) {
  return api(path, {
    method,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

export async function getAuthStatus() {
  return api('/auth/status');
}

export async function setup(password) {
  return apiJSON('POST', '/auth/setup', { password });
}

export async function login(password) {
  return apiJSON('POST', '/auth/login', { password });
}

export async function logout() {
  return apiJSON('POST', '/auth/logout');
}

export async function changePassword(oldPassword, newPassword) {
  return apiJSON('POST', '/auth/password', {
    old_password: oldPassword,
    new_password: newPassword,
  });
}

export async function listItems(params = {}) {
  const qs = new URLSearchParams();
  if (params.q) qs.set('q', params.q);
  if (params.status) qs.set('status', params.status);
  // Backend only accepts the literal "true"; anything else is a 40001.
  if (params.favorite === true) qs.set('favorite', 'true');
  if (params.page) qs.set('page', String(params.page));
  if (params.page_size) qs.set('page_size', String(params.page_size));
  const query = qs.toString();
  return api('/items' + (query ? '?' + query : ''));
}

export async function getItem(id) {
  return api('/items/' + encodeURIComponent(id));
}

export async function patchItem(id, data) {
  return apiJSON('PATCH', '/items/' + encodeURIComponent(id), data);
}

export async function uploadItem(file, fields = {}) {
  const fd = new FormData();
  fd.append('file', file);
  if (fields.title) fd.append('title', fields.title);
  if (fields.notes) fd.append('notes', fields.notes);
  if (fields.expires_in) fd.append('expires_in', fields.expires_in);
  if (fields.expires_at) fd.append('expires_at', fields.expires_at);
  return api('/items', { method: 'POST', body: fd });
}

export async function replaceItemContent(id, file) {
  const fd = new FormData();
  fd.append('file', file);
  return api('/items/' + encodeURIComponent(id) + '/content', {
    method: 'PUT',
    body: fd,
  });
}

export async function trashItem(id) {
  return api('/items/' + encodeURIComponent(id), { method: 'DELETE' });
}

export async function restoreItem(id) {
  return api('/items/' + encodeURIComponent(id) + '/restore', { method: 'POST' });
}

export async function permanentDeleteItem(id) {
  return api('/items/' + encodeURIComponent(id) + '/permanent', { method: 'DELETE' });
}

export async function favoriteItem(id) {
  return api('/items/' + encodeURIComponent(id) + '/favorite', { method: 'POST' });
}

export async function unfavoriteItem(id) {
  return api('/items/' + encodeURIComponent(id) + '/favorite', { method: 'DELETE' });
}

export async function listKeys() {
  return api('/keys');
}

export async function createKey(name) {
  return apiJSON('POST', '/keys', { name });
}

export async function revokeKey(id) {
  return api('/keys/' + encodeURIComponent(id), { method: 'DELETE' });
}

export async function getSettingsInfo() {
  return api('/settings/info');
}

export async function getBrowserStatus() {
  return api('/settings/browser');
}

export async function installBrowser() {
  return api('/settings/browser/install', { method: 'POST' });
}
