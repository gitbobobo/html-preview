/**
 * Shared HTML/ZIP upload helper.
 */

import { uploadItem } from './api.js';
import { showToast } from './util.js';

/**
 * @param {File} file
 * @param {{ onSuccess?: (item: object) => void }} [opts]
 */
export async function uploadFile(file, opts = {}) {
  const ext = file.name.split('.').pop()?.toLowerCase();
  if (!['html', 'htm', 'zip'].includes(ext)) {
    showToast('仅支持 .html / .zip');
    return null;
  }
  showToast('上传中…');
  try {
    const item = await uploadItem(file, { title: file.name.replace(/\.[^.]+$/, '') });
    showToast('已上传');
    opts.onSuccess?.(item);
    return item;
  } catch (err) {
    showToast(err.message);
    return null;
  }
}

export function pickAndUpload(opts = {}) {
  const input = document.getElementById('file-input');
  if (!input) return;
  input.onchange = async () => {
    const file = input.files[0];
    input.value = '';
    if (!file) return;
    await uploadFile(file, opts);
  };
  input.click();
}
