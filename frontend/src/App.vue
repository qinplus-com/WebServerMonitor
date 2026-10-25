<template>
  <router-view />
</template>

<script setup>
</script>

<style>
/* ===== 主题色板：默认暗色（SmartTerm 风格），:root.light 为亮色 =====
   暗色: bg0 #0d0d0d 输入框底 | bg1 #121212 页面底 | bg2 #171717 面板/卡片
   bg3 #1e1e1e 悬浮/弹窗 | border #2a2a2a | 文字 #e6e6e6 | 强调 #2f81f7 */
:root {
  color-scheme: dark;
  --bg-page: #121212;        /* 页面底色 */
  --bg-rail: #141414;        /* 侧栏 */
  --bg-bar: #171717;         /* 顶条/工具条 */
  --bg-panel: #171717;       /* 卡片/面板 */
  --bg-panel-2: #161616;     /* 次级面板 */
  --bg-elev: #1e1e1e;        /* 悬浮层/按钮/下拉/消息框 */
  --bg-dialog: #1a1a1a;      /* 弹窗 */
  --bg-table: #111111;       /* 表格 */
  --bg-table-head: #181818;  /* 表头 */
  --bg-input: #0d0d0d;       /* 输入框/终端底 */
  --bg-hover: #262626;       /* hover 背景 */
  --border: #2a2a2a;
  --border-soft: #262626;
  --text-1: #e6e6e6;                        /* 主文字 */
  --text-2: rgba(255, 255, 255, 0.65);      /* 次文字 */
  --text-3: rgba(255, 255, 255, 0.4);       /* 弱文字/占位 */
  --accent: #2f81f7;
  --hover-overlay: rgba(255, 255, 255, 0.08);   /* 导航/条目 hover */
  --active-overlay: rgba(47, 129, 247, 0.15);   /* 导航选中 */
  --scrollbar-thumb: #3a3a3a;
  --scrollbar-track: #1a1a1a;
}

:root.light {
  color-scheme: light;
  --bg-page: #f5f7fa;
  --bg-rail: #ffffff;
  --bg-bar: #ffffff;
  --bg-panel: #ffffff;
  --bg-panel-2: #fafbfc;
  --bg-elev: #f0f2f5;
  --bg-dialog: #ffffff;
  --bg-table: #ffffff;
  --bg-table-head: #f5f7fa;
  --bg-input: #ffffff;
  --bg-hover: #e4e7ed;
  --border: #dcdfe6;
  --border-soft: #e4e7ed;
  --text-1: #303133;
  --text-2: #606266;
  --text-3: #a8abb2;
  --accent: #2f81f7;
  --hover-overlay: rgba(0, 0, 0, 0.06);
  --active-overlay: rgba(47, 129, 247, 0.12);
  --scrollbar-thumb: #c0c4cc;
  --scrollbar-track: #f0f2f5;
}

html, body, #app { height: 100%; margin: 0; }
body { background: var(--bg-page); color: var(--text-1); }

/* ===== 高分辨率整体放大字体（--fs 全局字号系数，所有页面文字等比缩放） ===== */
:root { --fs: 1; }
@media (min-width: 2200px) {
  :root {
    --fs: 1.25;
    --el-font-size-extra-small: 14px;
    --el-font-size-small: 15px;
    --el-font-size-base: 16px;
    --el-font-size-medium: 17px;
    --el-font-size-large: 20px;
    --el-font-size-extra-large: 24px;
  }
  body { font-size: 16px; }
}
@media (min-width: 3400px) {
  :root {
    --fs: 1.5;
    --el-font-size-extra-small: 16px;
    --el-font-size-small: 17px;
    --el-font-size-base: 18px;
    --el-font-size-medium: 19px;
    --el-font-size-large: 22px;
    --el-font-size-extra-large: 26px;
  }
  body { font-size: 18px; }
}

/* ===== 页面容器内非终端页 ===== */
.page-host > *:not(.terminal-page) {
  padding: 16px;
}

/* ===== 弹窗适配 ===== */
.el-dialog { background: var(--bg-dialog); border: 1px solid var(--border); border-radius: 8px; }
.el-dialog .el-dialog__header { border-bottom: 1px solid var(--border); padding: 14px 20px; margin-right: 0; }
.el-dialog .el-dialog__title { color: var(--text-1); font-size: calc(14px * var(--fs, 1)); font-weight: 600; }
.el-dialog .el-dialog__body { color: var(--text-1); padding: 16px 20px; }
.el-dialog .el-dialog__footer { border-top: 1px solid var(--border); padding: 12px 20px; }
.el-form-item__label { color: var(--text-2) !important; }
.el-input__inner, .el-input__wrapper { background: var(--bg-input) !important; border-color: var(--border) !important; color: var(--text-1) !important; box-shadow: none !important; }
.el-input__inner::placeholder { color: var(--text-3); }
.el-input__wrapper.is-focus { border-color: var(--accent) !important; }
.el-select .el-select__wrapper { background: var(--bg-input) !important; border-color: var(--border) !important; box-shadow: none !important; }
.el-select .el-select__placeholder { color: var(--text-3); }
.el-select .el-select__selected-item { color: var(--text-1); }
.el-textarea__inner { background: var(--bg-input) !important; border-color: var(--border) !important; color: var(--text-1) !important; }
.el-input-number { background: var(--bg-input) !important; }
.el-input-number .el-input-number__decrease, .el-input-number .el-input-number__increase { background: var(--bg-input) !important; border-color: var(--border) !important; color: var(--text-2); }
.el-input-number .el-input__inner { background: var(--bg-input) !important; color: var(--text-1) !important; }

/* ===== 表格 ===== */
.el-table { background: var(--bg-table) !important; }
.el-table th.el-table__cell { background: var(--bg-table-head) !important; color: var(--text-1) !important; border-bottom: 1px solid var(--border) !important; font-size: calc(var(--el-font-size-base, 12px) + 2px); font-weight: 600; }
.el-table td.el-table__cell { background: var(--bg-table) !important; color: var(--text-1) !important; border-bottom: 1px solid var(--border) !important; font-size: calc(var(--el-font-size-base, 12px) + 1px); }
.el-table tr:hover > td.el-table__cell { background: var(--bg-hover) !important; }
.el-table--border { border: 1px solid var(--border) !important; }
.el-table--border .el-table__cell { border-right: 1px solid var(--border) !important; }
.el-table--border th.el-table__cell { border-right: 1px solid var(--border) !important; }
.el-table .el-table__empty-text { color: var(--text-3); }

/* ===== Tooltip 放大 ===== */
.el-popper.el-tooltip { font-size: calc(14px * var(--fs, 1)) !important; padding: 6px 12px !important; }

/* ===== 卡片 ===== */
.el-card { background: var(--bg-panel) !important; border: 1px solid var(--border) !important; color: var(--text-1); border-radius: 8px; }
.el-card .el-card__header { border-bottom: 1px solid var(--border); color: var(--text-1); }

/* ===== 按钮适配 ===== */
.el-button--default { background: var(--bg-elev) !important; border-color: var(--border) !important; color: var(--text-1) !important; }
.el-button--default:hover { background: var(--bg-hover) !important; border-color: var(--accent) !important; }

/* ===== 标签页 ===== */
.el-tabs__item { color: var(--text-2) !important; }
.el-tabs__item.is-active { color: var(--accent) !important; }
.el-tabs__active-bar { background: var(--accent) !important; }
.el-tabs__nav-wrap::after { background: var(--border) !important; }

/* ===== 标签 ===== */
.el-tag { border-color: var(--border); }

/* ===== 下拉菜单 ===== */
.el-dropdown-menu { background: var(--bg-elev) !important; border: 1px solid var(--border) !important; }
.el-dropdown-menu .el-dropdown-menu__item { color: var(--text-1) !important; }
.el-dropdown-menu .el-dropdown-menu__item:hover { background: var(--active-overlay) !important; }
.el-popper.is-light { background: var(--bg-elev) !important; border-color: var(--border) !important; }
.el-popper.is-light .el-popper__arrow::before { background: var(--bg-elev) !important; border-color: var(--border) !important; }

/* ===== 消息框 ===== */
.el-message-box { background: var(--bg-elev) !important; border: 1px solid var(--border) !important; }
.el-message-box .el-message-box__title { color: var(--text-1); }
.el-message-box .el-message-box__content { color: var(--text-2); }

/* ===== 分页 ===== */
.el-pagination .el-pager li { background: var(--bg-elev) !important; color: var(--text-2) !important; }
.el-pagination .el-pager li.is-active { color: var(--accent) !important; }
.el-pagination button { background: var(--bg-elev) !important; color: var(--text-2) !important; }

/* ===== 开关 ===== */
.el-switch__core { background: var(--border) !important; border-color: var(--border) !important; }

/* ===== 颜色选择器 ===== */
.el-color-picker__trigger { border: 1px solid var(--border) !important; }

/* ===== 分隔线 ===== */
.el-divider { border-color: var(--border) !important; }

/* ===== 主题切换按钮 ===== */
.theme-toggle {
  width: 36px; height: 36px; border-radius: 9px; margin-right: 10px;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; color: var(--text-2); transition: all .15s;
}
.theme-toggle:hover { color: var(--accent); background: var(--hover-overlay); }

/* ===== SSH 连接失败对话框：长文本完整可读 ===== */
.ssh-error-box { width: min(620px, 94%) !important; }
.ssh-error-box .el-message-box__message {
  white-space: pre-wrap; word-break: break-all; line-height: 1.9;
  max-height: 55vh; overflow-y: auto; color: var(--text-2);
  font-size: calc(13px * var(--fs, 1));
}

/* ===== 响应式 ===== */
@media (max-width: 768px) {
  .el-dialog { width: 94% !important; max-width: 94vw; }
  .el-message-box { width: 92% !important; max-width: 92vw; }
  .el-table { width: 100% !important; }
}
@media (max-width: 640px) {
  .page-host > *:not(.terminal-page) { padding: 10px; }
}
</style>
