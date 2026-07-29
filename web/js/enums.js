/**
 * Frontend mirrors of server-side enums / presets.
 */

export const EXPIRES_PRESETS = [
  { value: 'never', label: '永久' },
  { value: '1d', label: '1 天' },
  { value: '7d', label: '7 天' },
  { value: '30d', label: '30 天' },
  { value: '90d', label: '90 天' },
  { value: 'custom', label: '自定义' },
];

export const SCREENSHOT_STATUS_LABELS = {
  pending: '生成中',
  ready: '已完成',
  failed: '失败',
  no_browser: '无浏览器',
};

export const SCREENSHOT_PLACEHOLDERS = {
  pending: '/assets/placeholder-pending.svg',
  no_browser: '/assets/placeholder-no-browser.svg',
  failed: '/assets/placeholder-failed.svg',
};

export const INSTALL_STATUS_LABELS = {
  pending: '等待中',
  installing: '安装中',
  done: '完成',
  failed: '失败',
};
