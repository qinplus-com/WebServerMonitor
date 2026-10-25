﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿<template>
  <div class="terminal-page">
    <!-- ===== 标签 + 工具同一行 ===== -->
    <div class="tab-bar">
      <div class="tab-list">
        <div v-for="(t, idx) in tabs" :key="t.key" class="tab-item" :class="{ active: t.key === activeKey }" :style="t.key === activeKey ? { color: accentColor, boxShadow: `inset 0 -2px 0 ${accentColor}` } : {}" @click="activeKey = t.key" :title="`${t.name} - ${t.sessionId || ''}`">
          <span class="tab-index">{{ idx + 1 }}</span>
          <span class="tab-name">{{ t.name }}</span>
          <el-icon class="tab-close" @click.stop="closeTab(t.key)"><Close /></el-icon>
        </div>
      </div>

      <div class="tab-actions">
        <el-dropdown trigger="click" placement="bottom-start" @command="openServer">
          <div class="sbtn" title="打开服务器新连接"><el-icon :size="18"><Plus /></el-icon></div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="s in servers" :key="s.id" :command="s.id">
                <el-icon><Monitor /></el-icon> {{ s.name }} ({{ s.ip }})
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-tooltip content="服务器列表" placement="bottom" :show-after="200">
          <div class="sbtn" :class="{ on: showServerPanel }" @click="togglePanel('server')"><el-icon :size="18"><Grid /></el-icon></div>
        </el-tooltip>
        <el-tooltip content="文件管理（SFTP）" placement="bottom" :show-after="200">
          <div class="sbtn" @click="$router.push('/sftp')"><el-icon :size="18"><FolderOpened /></el-icon></div>
        </el-tooltip>

        <el-tooltip content="AI 助手（操作系统级技术支持）" placement="bottom" :show-after="200">
          <div class="sbtn" :class="{ on: showAiPanel }" @click="toggleAi"><el-icon :size="18"><ChatDotRound /></el-icon></div>
        </el-tooltip>

        <span class="sdivider" />

        <el-dropdown trigger="click" placement="bottom" @command="applyPreset">
          <div class="sbtn" title="配色方案"><el-icon :size="18"><Brush /></el-icon></div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="t in presetThemes" :key="t.name" :command="t.name">
                <span class="theme-swatch" :style="{ background: t.bg }"><i :style="{ color: t.fg }">Aa</i></span>
                {{ t.name }}
                <el-icon v-if="fgColor === t.fg && bgColor === t.bg" class="theme-check"><Check /></el-icon>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-tooltip content="终端设置（字号/颜色/反空闲）" placement="bottom" :show-after="200">
          <div class="sbtn" :class="{ on: showFontPanel }" @click="togglePanel('font')"><el-icon :size="18"><Tools /></el-icon></div>
        </el-tooltip>
        <el-tooltip :content="antiIdle ? '关闭反空闲' : '开启反空闲'" placement="bottom" :show-after="200">
          <div class="sbtn" :class="{ on: antiIdle }" @click="antiIdle = !antiIdle; restartAllAntiIdle()">
            <el-icon :size="18"><Timer /></el-icon>
          </div>
        </el-tooltip>
        <el-tooltip content="全屏" placement="bottom" :show-after="200">
          <div class="sbtn" @click="toggleFullscreen"><el-icon :size="18"><FullScreen v-if="!isFullscreen" /><Aim v-else /></el-icon></div>
        </el-tooltip>
      </div>
    </div>

    <!-- ===== 浮层遮罩（点击外部关闭面板） ===== -->
    <div v-if="showServerPanel || showFontPanel" class="panel-overlay" @click="closePanels" />

    <!-- ===== 浮动面板：服务器列表 ===== -->
    <div v-if="showServerPanel" class="float-panel server-panel">
      <div class="panel-header">服务器列表</div>
      <div class="panel-list">
        <div v-for="s in servers" :key="s.id" class="panel-item" @click="openServer(s.id)">
          <el-icon><Monitor /></el-icon>
          <div class="panel-item-info">
            <div class="panel-item-name">{{ s.name }}</div>
            <div class="panel-item-ip">{{ s.ip }}:{{ s.port }}</div>
          </div>
          <span v-if="tabs.filter(t => t.serverId === s.id).length" class="panel-item-badge">{{ tabs.filter(t => t.serverId === s.id).length }}</span>
        </div>
        <div v-if="!loading && servers.length === 0" class="panel-empty">暂无服务器</div>
      </div>
    </div>

    <!-- ===== 浮动面板：终端设置 ===== -->
    <div v-if="showFontPanel" class="float-panel settings-panel">
      <div class="panel-header">终端设置</div>
      <div class="panel-body">
        <div class="panel-row">
          <span class="panel-label">字号</span>
          <el-button-group size="small">
            <el-button @click="fontSize = Math.max(10, fontSize - 1)"><el-icon><Minus /></el-icon></el-button>
            <el-button style="pointer-events:none">{{ fontSize }}</el-button>
            <el-button @click="fontSize = Math.min(26, fontSize + 1)"><el-icon><Plus /></el-icon></el-button>
          </el-button-group>
        </div>
        <div class="panel-row">
          <span class="panel-label">文字色</span>
          <el-color-picker v-model="fgColor" size="small" :predefine="predefineColors" @change="applyTheme" />
        </div>
        <div class="panel-row">
          <span class="panel-label">背景色</span>
          <el-color-picker v-model="bgColor" size="small" :predefine="predefineBgColors" @change="applyTheme" />
        </div>
        <div class="panel-row">
          <span class="panel-label">反空闲</span>
          <el-switch v-model="antiIdle" size="small" @change="restartAllAntiIdle" />
        </div>
        <template v-if="antiIdle">
          <div class="panel-row">
            <span class="panel-label">方式</span>
            <el-select v-model="idleMethod" size="small" style="width:118px" @change="restartAllAntiIdle">
              <el-option label="空字符" value="null" />
              <el-option label="空格+退格" value="space" />
              <el-option label="回车" value="enter" />
              <el-option label="自定义" value="custom" />
            </el-select>
          </div>
          <div class="panel-row">
            <span class="panel-label">间隔</span>
            <el-select v-model="idleInterval" size="small" style="width:82px" @change="restartAllAntiIdle">
              <el-option label="15s" :value="15" />
              <el-option label="30s" :value="30" />
              <el-option label="60s" :value="60" />
              <el-option label="120s" :value="120" />
            </el-select>
          </div>
          <div v-if="idleMethod === 'custom'" class="panel-row">
            <span class="panel-label">字符</span>
            <el-input v-model="idleCustom" size="small" style="width:118px" placeholder="\n \r \t" @change="restartAllAntiIdle" />
          </div>
        </template>
      </div>
    </div>

    <!-- ===== 终端区 ===== -->
    <div class="term-area" :class="{ 'with-ai': showAiPanel }">
      <div class="term-body" :style="{ background: bgColor, border: tabs.length ? `2px solid ${accentColor}` : '2px solid transparent' }">
        <div v-for="t in tabs" :key="t.key" v-show="t.key === activeKey" class="term-wrap">
          <div class="xterm" :data-key="t.key"></div>
        </div>
        <div v-if="tabs.length === 0" class="empty-tip">
          <el-icon :size="52" color="#3a3a3a"><Monitor /></el-icon>
          <p>点击上方「<el-icon class="inline-icon"><Grid /></el-icon>」或「+」选择服务器</p>
          <p v-if="!loading && servers.length === 0" class="empty-sub">暂无可用服务器，请先到服务器管理添加</p>
        </div>
      </div>

      <!-- 底部状态栏 -->
      <div class="status-bar">
        <div class="status-left">
          <span class="status-item" v-if="activeTab">
            <span class="dot" :class="activeTab.connected ? 'ok' : 'err'"></span>
            {{ activeTab.name }} {{ activeTab.connected ? '已连接' : '未连接' }}
          </span>
          <span class="status-item dim" v-else>未打开终端</span>
        </div>
        <div class="status-right">
          <span class="status-item">{{ tabs.length }} 个会话</span>
          <span class="status-item">{{ fontSize }}px</span>
          <span class="status-item">{{ currentThemeName }}</span>
        </div>
      </div>
    </div>

    <!-- ===== AI 助手面板（右侧停靠） ===== -->
    <div v-if="showAiPanel" class="ai-panel">
      <div class="ai-header">
        <el-icon :size="16" color="#2f81f7"><ChatDotRound /></el-icon>
        <span class="ai-title">AI 助手</span>
        <span class="ai-sub">操作系统级技术支持</span>
        <div class="ai-header-actions">
          <el-tooltip content="设置" placement="bottom" :show-after="200">
            <div class="sbtn sm" :class="{ on: aiSettingsOpen }" @click="aiSettingsOpen = !aiSettingsOpen"><el-icon :size="15"><Setting /></el-icon></div>
          </el-tooltip>
          <el-tooltip content="清空对话" placement="bottom" :show-after="200">
            <div class="sbtn sm" @click="aiMessages = []"><el-icon :size="15"><Delete /></el-icon></div>
          </el-tooltip>
          <el-tooltip content="关闭" placement="bottom" :show-after="200">
            <div class="sbtn sm" @click="showAiPanel = false"><el-icon :size="15"><Close /></el-icon></div>
          </el-tooltip>
        </div>
      </div>

      <div v-if="aiSettingsOpen" class="ai-settings">
        <div class="panel-row">
          <span class="panel-label">供应商</span>
          <el-select v-model="aiProvider" size="small" style="flex:1">
            <el-option label="智谱 GLM-4-Flash（免费）" value="zhipu" />
            <el-option label="SiliconFlow（免费模型）" value="siliconflow" />
            <el-option label="Ollama（本地，完全免费）" value="ollama" />
          </el-select>
        </div>
        <div class="panel-row">
          <span class="panel-label">API Key</span>
          <el-input v-model="aiKey" size="small" :placeholder="aiProvider === 'ollama' ? '本地运行无需Key' : '必填'" show-password />
        </div>
        <div class="panel-row">
          <span class="panel-label">模型</span>
          <el-input v-model="aiModel" size="small" :placeholder="aiModelPlaceholder" />
        </div>
        <div class="ai-settings-tip" v-if="aiProvider === 'zhipu'">免费 Key：open.bigmodel.cn 注册即可领取，GLM-4-Flash 不收费</div>
        <div class="ai-settings-tip" v-else-if="aiProvider === 'siliconflow'">免费 Key：cloud.siliconflow.cn 注册领取，Qwen 小模型免费</div>
        <div class="ai-settings-tip" v-else>本机安装并运行 Ollama 即可，无需联网、完全免费</div>
      </div>

      <div class="ai-messages" ref="aiMsgBox">
        <div v-if="aiMessages.length === 0" class="ai-empty">
          <el-icon :size="38" color="#2f81f7"><ChatDotRound /></el-icon>
          <p>AI 运维助手已就绪</p>
          <p class="ai-empty-sub">可解答 Linux 命令、Shell 脚本、服务配置、故障排查、性能调优等操作系统级问题</p>
        </div>
        <div v-for="(m, i) in aiMessages" :key="i" class="ai-msg" :class="m.role">
          <div class="ai-bubble" :class="{ err: m.error }" v-html="renderMd(m.content)"></div>
        </div>
        <div v-if="aiLoading" class="ai-msg assistant">
          <div class="ai-bubble ai-thinking">思考中<span class="ai-dots"><i></i><i></i><i></i></span></div>
        </div>
      </div>

      <div class="ai-input-area">
        <div class="ai-ctx-row" :title="aiCtx ? '将把当前终端最后40行输出作为上下文发给AI' : '不携带终端上下文'">
          <el-switch v-model="aiCtx" size="small" />
          <span class="ai-ctx-label">附终端最后40行{{ activeTab ? `（${activeTab.name}）` : '（未打开终端）' }}</span>
        </div>
        <div class="ai-input-row">
          <el-input
            v-model="aiInput" type="textarea" :rows="2" resize="none"
            placeholder="描述你的问题，Enter 发送，Shift+Enter 换行&#10;如：磁盘占用满了怎么排查？"
            @keydown.enter.exact.prevent="aiSend"
          />
          <el-button type="primary" class="ai-send" :loading="aiLoading" @click="aiSend">
            <el-icon v-if="!aiLoading"><Promotion /></el-icon>
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, onActivated, onDeactivated, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
import { terminalApi, serverApi, userApi, aiApi } from '../api'
import {
  findCompletions, matchLocalIntent, searchCandidates, looksLikeNaturalLanguage,
  buildAiIntentMessages, parseAiIntent, drawConfirmCard, drawAskCard, visualWidth,
  suggestSimilarCommands, detectCommandNotFound,
} from '../utils/terminal-smart'

// keep-alive 按此名称缓存组件（切走时不销毁，SSH连接保持）
defineOptions({ name: 'Terminal' })

const route = useRoute()
const router = useRouter()
const servers = ref([])
const loading = ref(true)
const showServerPanel = ref(false)
const showFontPanel = ref(false)
const isFullscreen = ref(false)

// ===== 全局设置 =====
const fontSize = ref(Number(localStorage.getItem('term_font_size')) || 14)
const fgColor = ref(localStorage.getItem('term_fg') || '#d4d4d4')
const bgColor = ref(localStorage.getItem('term_bg') || '#0d0d0d')
const antiIdle = ref(localStorage.getItem('term_anti_idle') !== '0')
const idleInterval = ref(Number(localStorage.getItem('term_idle_interval')) || 30)
const idleMethod = ref(localStorage.getItem('term_idle_method') || 'null')
const idleCustom = ref(localStorage.getItem('term_idle_custom') || '')
const predefineColors = ['#d4d4d4', '#ffffff', '#00ff00', '#00e5ff', '#ffd700', '#ff79c6', '#8be9fd']
const predefineBgColors = ['#0d0d0d', '#000000', '#1e1e1e', '#002b36', '#300a24', '#121212', '#282a36']

const presetThemes = [
  { name: '默认深色', fg: '#d4d4d4', bg: '#0d0d0d' },
  { name: '经典黑绿', fg: '#00ff00', bg: '#000000' },
  { name: 'Linux纯黑', fg: '#ffffff', bg: '#000000' },
  { name: 'Solarized Dark', fg: '#93a1a1', bg: '#002b36' },
  { name: 'Dracula', fg: '#f8f8f2', bg: '#282a36' },
  { name: 'Tokyo Night', fg: '#a9b1d6', bg: '#121212' },
  { name: 'Nord', fg: '#d8dee9', bg: '#2e3440' },
  { name: '护眼浅色', fg: '#383a42', bg: '#fafafa' }
]

const currentThemeName = computed(() => {
  const t = presetThemes.find(t => t.fg === fgColor.value && t.bg === bgColor.value)
  return t ? t.name : '自定义'
})

// 激活态强调色：跟随终端前景色；若与背景亮度过于接近则回退为亮蓝
const accentColor = computed(() => {
  const lum = hex => {
    const c = hex.replace('#', '')
    const n = c.length === 3 ? c.split('').map(x => x + x).join('') : c
    const r = parseInt(n.slice(0, 2), 16) / 255, g = parseInt(n.slice(2, 4), 16) / 255, b = parseInt(n.slice(4, 6), 16) / 255
    return 0.2126 * r + 0.7152 * g + 0.0722 * b
  }
  try {
    return Math.abs(lum(fgColor.value) - lum(bgColor.value)) < 0.18 ? '#2f81f7' : fgColor.value
  } catch { return '#2f81f7' }
})

function applyPreset(name) {
  const t = presetThemes.find(t => t.name === name)
  if (t) { fgColor.value = t.fg; bgColor.value = t.bg }
}

// 面板开关（互斥）
function togglePanel(name) {
  if (name === 'server') { showServerPanel.value = !showServerPanel.value; if (showServerPanel.value) showFontPanel.value = false }
  else { showFontPanel.value = !showFontPanel.value; if (showFontPanel.value) showServerPanel.value = false }
}
function closePanels() { showServerPanel.value = false; showFontPanel.value = false }

watch(fontSize, v => {
  localStorage.setItem('term_font_size', v)
  for (const inst of tabInstances.values()) {
    try { inst.term.options.fontSize = v; inst.fit.fit() } catch {}
  }
})
watch([fgColor, bgColor], () => {
  localStorage.setItem('term_fg', fgColor.value)
  localStorage.setItem('term_bg', bgColor.value)
  applyTheme()
})
watch(idleInterval, v => localStorage.setItem('term_idle_interval', v))
watch(idleMethod, v => localStorage.setItem('term_idle_method', v))
watch(idleCustom, v => localStorage.setItem('term_idle_custom', v))
watch(antiIdle, v => localStorage.setItem('term_anti_idle', v ? '1' : '0'))

// ===== 服务器端偏好同步 =====
let prefLoaded = false
let prefSaveTimer = null
function schedulePrefSave() {
  if (!prefLoaded) return
  clearTimeout(prefSaveTimer)
  prefSaveTimer = setTimeout(() => {
    userApi.prefsSave({
      font_size: fontSize.value, fg: fgColor.value, bg: bgColor.value,
      anti_idle: antiIdle.value ? 1 : 0, idle_interval: idleInterval.value,
      idle_method: idleMethod.value, idle_custom: idleCustom.value
    }).catch(() => {})
  }, 600)
}
watch([fontSize, fgColor, bgColor, antiIdle, idleInterval, idleMethod, idleCustom], schedulePrefSave)

async function loadServerPrefs() {
  try {
    const res = await userApi.prefsGet()
    if (res.data) {
      const d = res.data
      if (d.font_size) fontSize.value = Number(d.font_size)
      if (d.fg) fgColor.value = d.fg
      if (d.bg) bgColor.value = d.bg
      if (d.anti_idle !== undefined) antiIdle.value = !!Number(d.anti_idle)
      if (d.idle_interval) idleInterval.value = Number(d.idle_interval)
      if (d.idle_method) idleMethod.value = d.idle_method
      if (d.idle_custom !== undefined) idleCustom.value = d.idle_custom
    }
  } catch {}
  prefLoaded = true
}

// ===== 多标签管理 =====
const tabs = ref([])
const activeKey = ref('')
const tabInstances = new Map()
const activeTab = computed(() => tabs.value.find(t => t.key === activeKey.value))
const activeInst = computed(() => tabInstances.get(activeKey.value))

function applyTheme() {
  for (const inst of tabInstances.values()) {
    try { inst.term.options.theme = buildTheme(); inst.fit.fit() } catch {}
  }
}
function buildTheme() {
  return { background: bgColor.value, foreground: fgColor.value, cursor: '#ffffff', selectionBackground: '#555f6d' }
}

async function openServer(serverId) {
  if (!serverId) return
  closePanels()
  const server = servers.value.find(s => s.id === serverId)
  try {
    const res = await terminalApi.connect(serverId, { silent: true })
    const sessionId = res.data.session_id
    const key = sessionId
    const sameCount = tabs.value.filter(t => t.serverId === serverId).length
    const name = server ? server.name : `服务器${serverId}`
    tabs.value.push({ key, serverId, name: sameCount > 0 ? `${name} (${sameCount + 1})` : name, sessionId, connected: false })
    activeKey.value = key
    await nextTick()
    const el = document.querySelector(`.xterm[data-key="${key}"]`)
    if (!el) return
    initTerminal(key, el, sessionId)
  } catch (e) {
    // 连接失败用对话框完整展示原因与排查建议（长文本消息条看不清）
    ElMessageBox.alert(e?.message || '连接失败', '连接服务器失败', {
      confirmButtonText: '知道了',
      customClass: 'ssh-error-box',
      autofocus: false
    }).catch(() => {})
  }
}

// 重连：对当前标签的服务器再开一个连接
function reconnect() {
  if (activeTab.value) openServer(activeTab.value.serverId)
}

function initTerminal(key, el, sessionId) {
  const term = new Terminal({
    cursorBlink: true, fontSize: fontSize.value,
    fontFamily: 'Consolas, "Courier New", monospace',
    theme: buildTheme(), allowProposedApi: true
  })
  const fit = new FitAddon()
  term.loadAddon(fit)
  term.open(el)
  fit.fit()

  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const token = localStorage.getItem('token')
  const ws = new WebSocket(`${proto}://${location.host}/ws/terminal?session_id=${sessionId}&token=${token}`)

  const tab = tabs.value.find(t => t.key === key)
  const inst = { term, fit, ws, heartbeat: null, idleTimer: null, sessionId }
  tabInstances.set(key, inst)

  // 智能输入：命令 ghost 联想 + 自然语言→命令确认卡
  const smart = setupSmartInput(inst)
  inst.smart = smart

  term.onData(data => {
    // false = 智能输入拦截（如自然语言回车），不发送到后端
    if (!smart.beforeSend(data)) return
    if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'command', data }))
  })
  term.onResize(({ cols, rows }) => {
    if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', cols, rows }))
  })
  // 复制到剪贴板：优先 Clipboard API，失败时用 execCommand 兜底。
  // onSelectionChange 中 navigator.clipboard 常因脱离用户激活上下文被浏览器拒绝，
  // 导致选中后无法自动复制、必须右键才能复制。
  function copyToClipboard(text) {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).catch(() => fallbackCopy(text))
    } else {
      fallbackCopy(text)
    }
  }
  function fallbackCopy(text) {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    try { document.execCommand('copy') } catch {}
    document.body.removeChild(ta)
  }
  term.onSelectionChange(() => {
    if (term.hasSelection()) copyToClipboard(term.getSelection())
  })
  if (term.element) {
    term.element.addEventListener('contextmenu', e => {
      e.preventDefault()
      if (term.hasSelection()) {
        copyToClipboard(term.getSelection())
        term.clearSelection()
      } else {
        navigator.clipboard.readText().then(text => {
          if (text && ws.readyState === WebSocket.OPEN) {
            smart.track(text)
            ws.send(JSON.stringify({ type: 'command', data: text }))
          }
        }).catch(() => ElMessage.warning('剪贴板权限受限，请用 Ctrl+V'))
      }
    })
  }

  ws.onopen = () => {
    if (tab) tab.connected = true
    term.writeln('\x1b[1;32m连接成功\x1b[0m\r\n')
    inst.heartbeat = setInterval(() => { if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'ping' })) }, 30000)
    startAntiIdle(inst)
    nextTick(() => {
      fit.fit()
      // 主动同步终端尺寸到后端 PTY：connect 未传 cols/rows 时后端默认 120×40，
      // 而 fit.fit() 不触发 onResize，若不同步，shell readline 会按错误列宽计算光标，
      // 导致编辑历史命令时连续按 ← 光标跳转到上一行
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
      }
    })
  }
  ws.onmessage = e => {
    try {
      const msg = JSON.parse(e.data)
      // 写入回调中再扫描输出：确保「命令未找到」卡片绘制在已更新后的缓冲位置上
      if (msg.type === 'stdout') term.write(msg.data, () => { smart.afterWrite(); smart.scanOutput(msg.data) })
    } catch { term.write(e.data, () => { smart.afterWrite(); smart.scanOutput(e.data) }) }
  }
  ws.onclose = () => { if (tab) tab.connected = false; term.writeln('\r\n\x1b[1;31m连接已断开\x1b[0m') }
}

function idlePayload() {
  switch (idleMethod.value) {
    case 'space': return '\x20\x08'
    case 'enter': return '\r'
    case 'custom': return unescapeCtrl(idleCustom.value)
    default: return '\x00'
  }
}
function unescapeCtrl(s) {
  return (s || '').replace(/\\(n|r|t|0|\\)/g, (_, c) => ({ n: '\n', r: '\r', t: '\t', 0: '\x00', '\\': '\\' }[c]))
}

function startAntiIdle(inst) {
  stopAntiIdle(inst)
  if (!antiIdle.value) return
  inst.idleTimer = setInterval(() => {
    if (inst.ws && inst.ws.readyState === WebSocket.OPEN) inst.ws.send(JSON.stringify({ type: 'command', data: idlePayload() }))
  }, idleInterval.value * 1000)
}
function stopAntiIdle(inst) { if (inst.idleTimer) { clearInterval(inst.idleTimer); inst.idleTimer = null } }
function restartAllAntiIdle() { for (const inst of tabInstances.values()) startAntiIdle(inst) }

function closeTab(key) {
  const inst = tabInstances.get(key)
  if (inst) {
    stopAntiIdle(inst); if (inst.heartbeat) clearInterval(inst.heartbeat)
    if (inst.smart) inst.smart.dispose()
    try { inst.ws.close() } catch {}
    terminalApi.close(inst.sessionId).catch(() => {})
    try { inst.term.dispose() } catch {}
    tabInstances.delete(key)
  }
  const idx = tabs.value.findIndex(t => t.key === key)
  if (idx >= 0) tabs.value.splice(idx, 1)
  if (activeKey.value === key) {
    activeKey.value = tabs.value.length ? tabs.value[Math.min(idx, tabs.value.length - 1)].key : ''
    nextTick(() => fitActive())
  }
}

watch(activeKey, () => nextTick(() => fitActive()))
function fitActive() { const inst = tabInstances.get(activeKey.value); if (inst) try { inst.fit.fit() } catch {} }
function sendCtrlC() {
  const inst = activeInst.value
  if (inst && inst.ws.readyState === WebSocket.OPEN) {
    if (inst.smart && inst.smart.reset) inst.smart.reset()
    inst.ws.send(JSON.stringify({ type: 'command', data: '\x03' }))
  }
}

// ============================================================
// 智能输入（命令 ghost 联想 + 自然语言→命令确认卡）
// ============================================================
function setupSmartInput(inst) {
  const { term, ws } = inst
  let ghost = null    // {cmd, x, y, rest, candidates, idx}
  let pending = null  // {commands, btnAbsLine, buttons}
  // 「命令未找到」相似建议：最近一次真正发往 shell 的输入行 + 输出尾部滚动窗口
  let lastSentCmd = ''
  let outTail = ''
  // 客户端镜像当前 shell 输入行：不依赖 PTY 回显时序，抗 SSH 延迟与 IME 整串提交
  // mirrorDirty：发生不可镜像的操作（↑↓历史、Ctrl+U 等）后置位，下次刷新从终端缓冲恢复
  let inputLine = ''
  let mirrorDirty = false
  const waiters = []

  // 中文输入法（IME）整串提交后，xterm 的 onData 与终端回显存在时序错位，
  // 输入行镜像可能漂移，导致随后输入英文命令时联想失灵。合成结束时标记脏行，
  // 让下一次刷新改从终端缓冲重算输入行，从根上恢复联想。
  function onImeCompositionEnd() {
    mirrorDirty = true
    setTimeout(() => { if (mirrorDirty) scheduleRefresh() }, 50)
  }
  if (term.textarea) term.textarea.addEventListener('compositionend', onImeCompositionEnd)

  function sendRaw(s) {
    if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'command', data: s }))
  }
  // 终端写完一批数据后的回调（刷新 ghost + 唤醒等待者）
  function afterWrite() {
    const w = waiters.shift()
    if (w) w()
    scheduleRefresh()
  }
  // ghost 刷新防抖：PTY 常把一次输入拆成多字节逐帧回显，合并后再重算避免抖动
  let refreshScheduled = false
  function scheduleRefresh() {
    if (refreshScheduled) return
    refreshScheduled = true
    setTimeout(() => { refreshScheduled = false; refreshGhost() }, 40)
  }
  // 等待下一批终端写入（带超时兜底）
  function waitNextWrite(timeoutMs = 600) {
    return new Promise(resolve => {
      let done = false
      const fin = () => { if (!done) { done = true; const i = waiters.indexOf(fin); if (i >= 0) waiters.splice(i, 1); resolve() } }
      waiters.push(fin)
      setTimeout(fin, timeoutMs)
    })
  }

  // ----- 当前输入行读取 -----
  // 两个关键坑（曾导致有回滚历史后 ghost 永久失效、中文输入后联想失灵）：
  // ① cursorY 是视口内行号，getLine 需要绝对缓冲行号 = baseY + cursorY，
  //    否则缓冲区一旦滚动（输出超过一屏）就会读到顶部旧行；
  // ② cursorX 按“列数”计，中文宽字符占 2 列 1 字符，substring 按字符数截取
  //    会把宽字符后面的填充空格算进文本 → 按单元格逐格游走取字符
  function readLine() {
    const buf = term.buffer.active
    const x = buf.cursorX, y = buf.baseY + buf.cursorY
    const line = buf.getLine(y)
    if (!line) return { x, y, text: '' }
    let text = ''
    for (let i = 0; i < x; i++) {
      const c = line.getCell(i)
      if (!c) break
      if (c.getWidth() === 0) continue // 宽字符右半格
      text += c.getChars() || ' '
    }
    return { x, y, text }
  }
  // 剥离 shell 提示符，取真正输入的命令（仅脏行恢复时使用）
  // 兼容 bash(#$%)、zsh(%)、oh-my-zsh(➜ … ✗) 与无空格变体
  function stripPrompt(line) {
    const marks = /[#$%>\u279c\u2717\u2707]/g
    let last = -1
    let m
    while ((m = marks.exec(line)) !== null) last = m.index
    const raw = last >= 0 ? line.slice(last + 1) : line
    return raw.replace(/^\s+/, '')
  }

  // ----- ghost 浮层（纯 DOM 覆盖，不写入终端缓冲，杜绝与 PTY 逐字节回显互相覆盖） -----
  const overlay = document.createElement('div')
  Object.assign(overlay.style, {
    position: 'absolute', display: 'none', zIndex: 20, gap: '3px',
    pointerEvents: 'none', alignItems: 'center', whiteSpace: 'pre',
    fontFamily: term.options.fontFamily || 'Consolas, "Courier New", monospace',
  })
  const gtext = document.createElement('span')
  Object.assign(gtext.style, {
    color: '#8a8a8a', pointerEvents: 'none',
    // 不透明底色：遮住光标后可能存在的 shell 自动建议（zsh autosuggestions 等），
    // 避免我们的补全与 shell 补全文字叠加成乱码
    background: term.options.background || '#000',
    padding: '0 1px',
  })
  overlay.appendChild(gtext)
  function makeBtn(chr, title, onClick) {
    const b = document.createElement('div')
    b.title = title
    b.textContent = chr
    Object.assign(b.style, {
      width: '19px', height: '19px', lineHeight: '17px', textAlign: 'center',
      border: '1px solid rgba(140,140,140,.45)', borderRadius: '5px',
      background: 'rgba(30,30,30,.85)', color: '#cfcfcf', cursor: 'pointer',
      fontSize: '12px', userSelect: 'none', boxSizing: 'border-box',
      pointerEvents: 'auto',
    })
    b.addEventListener('mousedown', e => e.preventDefault())
    b.addEventListener('click', () => { onClick(); term.focus() })
    return b
  }
  // 提示胶囊（外观是快捷键提示，点击等同于按快捷键）
  function makeHint(label, title, onClick) {
    const b = document.createElement('div')
    b.title = title
    b.textContent = label
    Object.assign(b.style, {
      height: '18px', lineHeight: '17px', padding: '0 7px', textAlign: 'center',
      border: '1px solid rgba(47,129,247,.55)', borderRadius: '9px',
      background: 'rgba(47,129,247,.14)', color: '#7fb0ff', cursor: 'pointer',
      fontSize: '11px', userSelect: 'none', boxSizing: 'border-box',
      pointerEvents: 'auto', whiteSpace: 'nowrap',
    })
    b.addEventListener('mousedown', e => e.preventDefault())
    b.addEventListener('click', () => { onClick(); term.focus() })
    return b
  }
  overlay.appendChild(makeHint('Ctrl+↵ 直接执行', '接受补全并立即执行 (Ctrl+Enter)', acceptAndRun))
  overlay.appendChild(makeBtn('⌄', '切换下一条 (↓)', cycleGhost))
  term.element.appendChild(overlay)

  function metrics() {
    const scr = term.element.querySelector('.xterm-screen')
    return {
      w: parseFloat(scr.style.width) / term.cols,
      h: parseFloat(scr.style.height) / term.rows,
      fs: term.options.fontSize || 14,
    }
  }
  function positionOverlay() {
    if (!ghost) return
    const { w, h, fs } = metrics()
    gtext.textContent = ghost.rest
    overlay.style.fontSize = fs + 'px'
    overlay.style.height = h + 'px'
    overlay.style.lineHeight = h + 'px'
    overlay.style.left = `${ghost.x * w}px`
    overlay.style.top = `${ghost.y * h + 1}px`
    overlay.style.display = 'flex'
  }

  // ----- ghost 状态机 -----
  // 行文本判定优先级：可信镜像（buffer 行尾与镜像一致，任意提示符格式通吃）
  // → 脏行后从缓冲恢复（stripPrompt 兜底）→ 都不可靠则不联想
  function refreshGhost() {
    if (pending) return hideGhost()
    const { x, y, text } = readLine()
    let cmd
    if (!mirrorDirty && inputLine && text.endsWith(inputLine)) {
      cmd = inputLine // 可信镜像优先（任意提示符格式通吃）
    } else {
      // 镜像为空或与回显脱节（IME 整串提交、宽字符退格、卡片导致提示符位移等）：
      // 以终端缓冲重算输入行，而不是直接放弃联想——否则联想会一直失灵
      cmd = stripPrompt(text)
      inputLine = cmd
      mirrorDirty = false
    }
    if (!cmd) return hideGhost()
    if (ghost && ghost.cmd === cmd) { ghost.x = x; ghost.y = y; positionOverlay(); return }
    const candidates = findCompletions(cmd)
    const full = candidates[0]
    if (!full || !full.startsWith(cmd)) return hideGhost()
    const rest = full.substring(cmd.length)
    if (!rest || visualWidth(rest) > term.cols - x - 14) return hideGhost() // 14 ≈ 提示胶囊+按钮宽
    ghost = { cmd, x, y, rest, candidates, idx: 0 }
    positionOverlay()
  }
  function hideGhost() {
    ghost = null
    overlay.style.display = 'none'
  }
  // 按键发出前处理：镜像输入行、隐藏 ghost、拦截自然语言回车
  // 返回 false 表示本次数据被智能输入接管，不要发送到后端
  function beforeSend(data) {
    // 用户绕过卡片快捷键直接向 shell 输入 → 视为放弃当前卡片，避免 pending 永久挂起
    // （否则确认卡之后英文命令的 ghost 联想会被一直压制、旧卡链接也仍可点击）
    if (pending) pending = null
    hideGhost()
    if (data === '\r' || data === '\n') {
      const cmd = inputLine
      inputLine = ''
      mirrorDirty = false
      if (looksLikeNaturalLanguage(cmd)) { lastSentCmd = ''; handleNatural(cmd); return false }
      // 记录本次真正发往 shell 的输入行，供输出侧「命令未找到」检测使用
      lastSentCmd = cmd
      return true
    }
    if (data === '\x03') { inputLine = ''; mirrorDirty = false; return true } // Ctrl+C
    if (data === '\x7f' || data === '\x08') { inputLine = inputLine.slice(0, -1); return true }
    // Tab 到达此处说明当时无 ghost（有 ghost 会在按键层被 acceptGhost 拦截），
    // 交由 shell 自行补全：结果不可预知，标记脏行从缓冲恢复，避免镜像脱节导致联想失效
    if (data === '\t') {
      mirrorDirty = true
      setTimeout(() => { if (mirrorDirty) scheduleRefresh() }, 60)
      return true
    }
    // ESC 序列（↑↓历史、Home/End/Delete…）与 Ctrl 控制键（Ctrl+U/W/A/E…）无法逐字符镜像，
    // 标记脏行：ghost 刷新时改从终端缓冲恢复输入行，避免镜像脱节导致联想失效
    if (data.charCodeAt(0) === 0x1b || (data.length === 1 && data < ' ')) {
      mirrorDirty = true
      // 主动安排刷新：若该键不产生任何回显（如空历史的 ↑、Ctrl+U 清行），
      // afterWrite 不会触发，ghost 将永远不恢复
      setTimeout(() => { if (mirrorDirty) scheduleRefresh() }, 60)
      return true
    }
    if (data >= ' ') track(data)
    return true
  }
  // 跟踪粘贴/IME 整串文本到输入行镜像（含右键粘贴、多行粘贴）
  function track(data) {
    const clean = String(data).replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '')
    if (clean.includes('\r') || clean.includes('\n')) {
      const parts = clean.split(/\r\n|\r|\n/)
      inputLine = parts[parts.length - 1]
    } else {
      inputLine += clean
    }
  }
  function acceptGhost() {
    if (!ghost) return
    const cand = ghost.candidates[ghost.idx]
    const rest = cand.substring(ghost.cmd.length)
    inputLine += rest // 同步客户端输入行镜像
    hideGhost()
    sendRaw(rest)
  }
  // 接受补全并立即执行（Ctrl+Enter / 点击提示胶囊）
  function acceptAndRun() {
    if (!ghost) return
    const cand = ghost.candidates[ghost.idx]
    const rest = cand.substring(ghost.cmd.length)
    hideGhost()
    inputLine = '' // 补全部分 + 回车直接交给 shell 执行，执行后是新提示符
    lastSentCmd = cand // 直接执行，同样纳入「命令未找到」检测
    sendRaw(rest + '\r')
  }
  function cycleGhost() {
    if (!ghost) return
    const g = ghost
    g.idx = (g.idx + 1) % g.candidates.length
    g.rest = g.candidates[g.idx].substring(g.cmd.length)
    positionOverlay()
  }

  // ----- 卡片点击（xterm link provider，确认卡 + 澄清卡通用） -----
  // xterm 回调传入的 lineNum 是 1 基绝对缓冲行；自存的 absLine 为 0 基
  term.registerLinkProvider({
    provideLinks(lineNum, cb) {
      if (!pending) return
      if (pending.type === 'confirm') {
        if (lineNum !== pending.btnAbsLine + 1) return
        cb(pending.buttons.map(b => ({
          range: {
            start: { x: b.startColumn, y: lineNum },
            end: { x: b.endColumn, y: lineNum },
          },
          text: b.key,
          activate: () => doButton(b.key),
          hoverText: b.key === 'run' ? '执行命令' : b.key === 'edit' ? '修改命令' : '拒绝执行',
        })))
      } else {
        const hits = pending.links.filter(l => l.absLine + 1 === lineNum)
        if (!hits.length) return
        cb(hits.map(l => ({
          range: {
            start: { x: l.startColumn, y: lineNum },
            end: { x: l.endColumn, y: lineNum },
          },
          text: String(l.idx),
          activate: () => l.idx < 0 ? cancelAsk() : chooseAsk(l.idx),
          hoverText: l.idx < 0 ? '取消，重新描述' : '选择该操作',
        })))
      }
    },
  })

  // 弹出确认卡
  function openConfirm(intent) {
    const { btnAbsLine, buttons } = drawConfirmCard(term, intent.desc, intent.commands)
    pending = { type: 'confirm', commands: intent.commands, btnAbsLine, buttons }
  }
  // 弹出澄清提问卡（候选行附命令预览，用户选择后二次确认）
  function openAsk(text, options, opts) {
    const shown = options.map(o => ({
      ...o,
      desc: (o.desc || '').replace(/[：:]\s*$/, '') + ' → ' + o.commands[0].trim(),
    }))
    const { links } = drawAskCard(term, text, shown, opts)
    pending = { type: 'ask', options, links }
  }
  // 澄清卡：选中某个候选 → 转确认卡
  function chooseAsk(i) {
    if (!pending || pending.type !== 'ask') return
    const opt = pending.options[i]
    if (!opt) return
    pending = null
    openConfirm(opt)
  }
  // 澄清卡：放弃选择
  function cancelAsk() {
    if (!pending || pending.type !== 'ask') return
    pending = null
    term.writeln('\x1b[2;37m好的，请换个说法重新描述您的需求\x1b[0m')
  }

  function doButton(key) {
    if (!pending || pending.type !== 'confirm') return
    const cmds = pending.commands
    pending = null
    lastSentCmd = '' // 卡片命令由用户显式选择，不再做「未找到」相似推荐
    overlay.style.display = 'none'
    if (key === 'run') {
      inputLine = '' // 命令直接经 sendRaw 执行，镜像清空
      cmds.forEach(c => sendRaw(c + '\r'))
    } else if (key === 'edit') {
      inputLine = cmds[0] // 回填到镜像，用户可继续编辑后回车
      sendRaw(cmds[0]); term.focus()
    } else {
      inputLine = ''
      term.writeln('\x1b[2;37m已取消，未执行任何命令\x1b[0m')
    }
  }

  // ----- 自然语言 → 命令 -----
  function aiCallable() {
    return aiProvider.value === 'ollama' || !!aiKey.value.trim()
  }

  // 本地与 AI 都无法确定时：在终端内引导换说法（不静默失败）
  function guidance(text) {
    term.writeln('\x1b[2;37m我还没完全理解「' + text + '」，请换个说法，例如：查看磁盘空间、检查内存占用、查看监听端口、测试网络连通性\x1b[0m')
  }

  // ----- 命令未找到 → 相似命令建议 -----
  // 扫描 shell 输出（含跨帧滚动窗口），仅在确实命中「command not found」且能给出
  // 相似命令时弹出候选卡；命中一次后清空 lastSentCmd，避免重复提示。
  function scanOutput(data) {
    if (pending || !lastSentCmd) return
    const s = typeof data === 'string' ? data : String(data || '')
    outTail = (outTail + s).slice(-400)
    const token = detectCommandNotFound(outTail, lastSentCmd.split(/\s+/)[0])
    if (!token) return
    lastSentCmd = ''
    outTail = ''
    const opts = suggestSimilarCommands(token, 8)
    if (!opts.length) return
    openAsk(token, opts, {
      prefix: ' 命令「',
      suffix: '」未找到，您是想执行以下哪个命令？',
      cancelText: ' 0. 都不是，我重新输入',
    })
  }

  async function handleNatural(lineCmd) {
    const text = lineCmd.trim()
    // 1. 先用 Ctrl+C 放弃 shell 当前输入行（保留已输入文字可见），等其回显完成
    const p = waitNextWrite(400)
    sendRaw('\x03')
    await p
    // 2. 强意图库（多步命令配方，离线、即时）：命中直接出确认卡
    const intent = matchLocalIntent(text)
    if (intent) { openConfirm(intent); return }
    // 3. 语义检索（700+ 条「命令名+中文说明」全文匹配，离线、亚毫秒级）：
    //    反馈相关候选给用户选择，选中后再出确认卡做二次处理（给足候选，避免只给零星几条）
    const cands = searchCandidates(text, 10)
    if (cands.length) { openAsk(text, cands); return }
    // 4. 检索无果：AI 兜底（仅此时才有网络等待）
    if (aiCallable()) {
      term.writeln('\x1b[2;37m正在理解「' + text + '」，请稍候...\x1b[0m')
      try {
        const res = await aiApi.chat({
          provider: aiProvider.value,
          api_key: aiKey.value.trim(),
          model: aiModel.value.trim(),
          messages: buildAiIntentMessages(text),
        })
        const aiIntent = parseAiIntent(res.data.content)
        if (aiIntent) { openConfirm(aiIntent); return }
      } catch {
        // AI 不可用/超时统一走引导兜底
      }
    }
    guidance(text)
  }

  // ----- 键盘拦截 -----
  term.attachCustomKeyEventHandler((e) => {
    if (e.type !== 'keydown') return true
    // 澄清卡：数字选择 / 0 或 Esc 取消
    if (pending && pending.type === 'ask' && !e.ctrlKey && !e.altKey && !e.metaKey) {
      if (e.key === 'Escape') { e.preventDefault(); cancelAsk(); return false }
      if (/^[0-9]$/.test(e.key)) {
        const n = Number(e.key)
        if (n === 0) { e.preventDefault(); cancelAsk(); return false }
        if (n <= pending.options.length) { e.preventDefault(); chooseAsk(n - 1); return false }
      }
    }
    // 确认卡快捷键
    if (pending && pending.type === 'confirm' && e.ctrlKey) {
      const k = (e.key || '').toLowerCase()
      if (e.key === 'Enter') { e.preventDefault(); doButton('run'); return false }
      if (k === 'e') { e.preventDefault(); doButton('edit'); return false }
      if (k === 'x') { e.preventDefault(); doButton('cancel'); return false }
    }
    if (!e.repeat && ghost) {
      if (e.ctrlKey && e.key === 'Enter') { e.preventDefault(); acceptAndRun(); return false }
      if (e.key === 'Tab' || e.key === 'ArrowRight') { e.preventDefault(); acceptGhost(); return false }
      if (e.key === 'ArrowDown') { e.preventDefault(); cycleGhost(); return false }
    }
    // 自然语言回车的拦截统一在 onData/beforeSend 完成（兼容 IME 整串提交）
    return true
  })

  function dispose() {
    try { overlay.remove() } catch {}
    try { term.textarea && term.textarea.removeEventListener('compositionend', onImeCompositionEnd) } catch {}
    ghost = null; pending = null
  }
  // 外部（工具栏 Ctrl+C）打断当前输入时同步复位
  function reset() {
    inputLine = ''
    mirrorDirty = false
    ghost = null
    pending = null
    lastSentCmd = ''
    outTail = ''
    overlay.style.display = 'none'
  }

  return { beforeSend, afterWrite, dispose, track, reset, scanOutput }
}

// ===== AI 助手（免费供应商，操作系统级技术支持） =====
const showAiPanel = ref(false)
const aiSettingsOpen = ref(false)
const aiProvider = ref(localStorage.getItem('ai_provider') || 'zhipu')
const aiKey = ref(localStorage.getItem('ai_key') || '')
const aiModel = ref(localStorage.getItem('ai_model') || '')
const aiCtx = ref(localStorage.getItem('ai_ctx') !== '0')
const aiInput = ref('')
const aiMessages = ref([])
const aiLoading = ref(false)
const aiMsgBox = ref(null)

const aiModelPlaceholder = computed(() => ({
  zhipu: 'glm-4-flash',
  siliconflow: 'Qwen/Qwen2.5-7B-Instruct',
  ollama: 'qwen2.5:7b'
}[aiProvider.value] || ''))

watch([aiProvider, aiKey, aiModel, aiCtx], () => {
  localStorage.setItem('ai_provider', aiProvider.value)
  localStorage.setItem('ai_key', aiKey.value)
  localStorage.setItem('ai_model', aiModel.value)
  localStorage.setItem('ai_ctx', aiCtx.value ? '1' : '0')
})

function toggleAi() {
  showAiPanel.value = !showAiPanel.value
  nextTick(() => fitActive())
}

// 读取当前激活终端最后N行输出作为AI上下文
function getTerminalContext(lines = 40) {
  const inst = activeInst.value
  if (!inst) return ''
  const buf = inst.term.buffer.active
  const out = []
  for (let i = Math.max(0, buf.length - lines); i < buf.length; i++) {
    const line = buf.getLine(i)
    if (line) out.push(line.translateToString(true))
  }
  return out.filter(l => l.trim()).join('\n').trimEnd()
}

function escHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
// 轻量markdown渲染：代码块/行内代码/加粗/换行
function renderMd(s) {
  const pres = []
  let t = escHtml(s || '')
  t = t.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) => {
    pres.push(`<div class="ai-code"><div class="ai-code-lang">${lang || 'shell'}</div><pre><code>${code.replace(/\n$/, '')}</code></pre></div>`)
    return `\u0000${pres.length - 1}\u0000`
  })
  t = t.replace(/`([^`\n]+)`/g, '<code class="ai-inline">$1</code>')
  t = t.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>')
  t = t.replace(/\n/g, '<br>')
  return t.replace(/\u0000(\d+)\u0000/g, (_, i) => pres[Number(i)])
}

function scrollAi() {
  nextTick(() => { if (aiMsgBox.value) aiMsgBox.value.scrollTop = aiMsgBox.value.scrollHeight })
}

async function aiSend() {
  const q = aiInput.value.trim()
  if (!q || aiLoading.value) return
  if (aiProvider.value !== 'ollama' && !aiKey.value.trim()) {
    aiSettingsOpen.value = true
    ElMessage.warning('请先在面板右上角「设置」中填写 API Key（Ollama 本地模式无需 Key）')
    return
  }
  aiMessages.value.push({ role: 'user', content: q })
  aiInput.value = ''
  scrollAi()
  aiLoading.value = true
  try {
    // 携带最近10条历史作为对话上下文
    const history = aiMessages.value.slice(0, -1).filter(m => !m.error).slice(-10).map(m => ({ role: m.role, content: m.content }))
    const msgs = [...history]
    let finalQ = q
    if (aiCtx.value) {
      const ctx = getTerminalContext(40)
      if (ctx) finalQ = `以下是当前终端（${activeTab.value?.name || ''}）最近的输出，请结合它分析我的问题：\n\`\`\`\n${ctx}\n\`\`\`\n\n我的问题：${q}`
    }
    msgs.push({ role: 'user', content: finalQ })
    const res = await aiApi.chat({
      provider: aiProvider.value,
      api_key: aiKey.value.trim(),
      model: aiModel.value.trim(),
      messages: msgs
    })
    aiMessages.value.push({ role: 'assistant', content: res.data.content })
  } catch (e) {
    aiMessages.value.push({ role: 'assistant', content: e.message || '请求失败', error: true })
  } finally {
    aiLoading.value = false
    scrollAi()
  }
}

// 全屏
function toggleFullscreen() {
  if (!document.fullscreenElement) { document.documentElement.requestFullscreen().catch(() => {}) }
  else { document.exitFullscreen().catch(() => {}) }
}
function onFsChange() { isFullscreen.value = !!document.fullscreenElement; nextTick(() => fitActive()) }

function onWinResize() { fitActive() }

onMounted(async () => {
  loadServerPrefs()
  try { const res = await serverApi.list({}); servers.value = res.data || [] } finally { loading.value = false }
  window.addEventListener('resize', onWinResize)
  document.addEventListener('fullscreenchange', onFsChange)
  if (route.query.server_id) {
    openServer(Number(route.query.server_id))
    router.replace({ path: '/terminal' })
  }
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onWinResize)
  document.removeEventListener('fullscreenchange', onFsChange)
  for (const key of [...tabInstances.keys()]) closeTab(key)
})

// keep-alive：切回终端页时组件被重新激活（此时DOM重新挂回，需重算尺寸）
onActivated(() => {
  closePanels()
  // 从概览页点击服务器跳转：keep-alive 下 onMounted 不再触发，在此处理
  if (route.query.server_id) {
    openServer(Number(route.query.server_id))
    router.replace({ path: '/terminal' })
  }
  nextTick(() => fitActive())
})
// keep-alive：切走时仅关闭浮层面板，WebSocket/心跳/反空闲全部保留
onDeactivated(() => {
  closePanels()
})
</script>

<style scoped>
.terminal-page { display: flex; flex-direction: column; height: 100%; background: var(--bg-page); position: relative; }

/* ===== 标签+工具同一行 ===== */
.tab-bar {
  display: flex; align-items: center; justify-content: space-between;
  height: 40px;
  background: var(--bg-rail); border-bottom: 1px solid var(--border-soft); flex-shrink: 0;
}
.tab-list { display: flex; flex: 1; overflow-x: auto; scrollbar-width: none; }
.tab-list::-webkit-scrollbar { display: none; }
.tab-item {
  display: flex; align-items: center; gap: 7px;
  padding: 0 12px; height: 40px; cursor: pointer; font-size: calc(13px * var(--fs, 1));
  color: var(--text-2); user-select: none; max-width: 220px;
  border-right: 1px solid var(--border-soft); transition: all .15s; flex-shrink: 0;
  position: relative;
}
.tab-item:hover { color: var(--text-1); background: var(--hover-overlay); }
.tab-item.active {
  color: var(--accent); background: var(--bg-page);
  box-shadow: inset 0 -2px 0 var(--accent);
}
.tab-index {
  font-size: calc(11px * var(--fs, 1)); color: var(--accent);
  background: var(--active-overlay);
  border-radius: 4px; padding: 0 5px; line-height: 17px; min-width: 16px; text-align: center;
}
.tab-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tab-close { font-size: calc(11px * var(--fs, 1)); border-radius: 50%; padding: 1px; transition: .15s; }
.tab-close:hover { background: var(--hover-overlay); color: #f85149; }

.tab-actions { display: flex; align-items: center; gap: 3px; padding: 0 8px; flex-shrink: 0; }
.sbtn {
  width: 32px; height: 32px; border-radius: 7px;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; color: var(--text-2); transition: all .15s;
}
.sbtn:hover { color: var(--text-1); background: var(--hover-overlay); }
.sbtn.on { color: var(--accent); background: var(--active-overlay); }
.sbtn.danger:hover { color: #f85149; background: rgba(247,118,142,0.1); }
.sbtn[disabled] { opacity: .35; cursor: not-allowed; }
.sdivider { width: 1px; height: 18px; background: var(--border); margin: 0 6px; }

.dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; }
.dot.ok { background: #3fb950; }
.dot.err { background: #f85149; }

/* ===== 浮层 & 浮动面板 ===== */
.panel-overlay { position: absolute; inset: 0; z-index: 90; background: transparent; }
.float-panel {
  position: absolute; top: 44px; z-index: 100;
  background: var(--bg-elev); border: 1px solid var(--border);
  border-radius: 10px; box-shadow: 0 12px 40px rgba(0,0,0,0.5);
  overflow: hidden;
}
.server-panel { left: 10px; width: min(230px, calc(100% - 20px)); }
.settings-panel { left: 196px; width: 224px; max-width: calc(100% - 20px); }

.panel-header {
  padding: 10px 14px; font-size: calc(12px * var(--fs, 1)); color: var(--text-2);
  border-bottom: 1px solid var(--border); font-weight: 600;
}
.panel-list { max-height: min(320px, calc(100vh - 150px)); overflow-y: auto; padding: 5px; }
.panel-item {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 10px; border-radius: 7px; cursor: pointer;
  color: var(--text-2); transition: .15s;
}
.panel-item:hover { background: var(--hover-overlay); color: var(--text-1); }
.panel-item-info { flex: 1; min-width: 0; }
.panel-item-name { font-size: calc(13px * var(--fs, 1)); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.panel-item-ip { font-size: calc(11px * var(--fs, 1)); color: var(--text-3); }
.panel-item-badge {
  background: var(--accent); color: var(--bg-rail); font-size: calc(10px * var(--fs, 1)); font-weight: bold;
  min-width: 16px; height: 16px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center; padding: 0 4px;
}
.panel-empty { text-align: center; padding: 20px; color: var(--text-3); font-size: calc(12px * var(--fs, 1)); }

.panel-body { padding: 12px 14px; display: flex; flex-direction: column; gap: 11px; }
.panel-row { display: flex; align-items: center; gap: 8px; }
.panel-label { font-size: calc(12px * var(--fs, 1)); color: var(--text-2); min-width: 44px; }

/* ===== 终端区 ===== */
.term-area { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.term-body { flex: 1; position: relative; overflow: hidden; box-sizing: border-box; min-height: 0; }
.term-wrap { position: absolute; inset: 0; }
.xterm { height: 100%; }
.xterm :deep(.xterm-screen) { height: 100% !important; }
.empty-tip { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; color: var(--bg-hover); gap: 10px; font-size: calc(14px * var(--fs, 1)); }
.empty-tip .inline-icon { vertical-align: -3px; }
.empty-sub { font-size: calc(12px * var(--fs, 1)); color: var(--bg-hover); }

/* ===== 底部状态栏 ===== */
.status-bar {
  display: flex; align-items: center; justify-content: space-between;
  height: 28px; padding: 0 14px;
  background: var(--bg-rail); border-top: 1px solid var(--border-soft);
  font-size: calc(11px * var(--fs, 1)); color: var(--text-2); flex-shrink: 0;
}
.status-left, .status-right { display: flex; gap: 14px; }
.status-item { display: flex; align-items: center; gap: 5px; }
.status-item.dim { color: var(--text-3); }

/* 配色预览 */
.theme-swatch { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 16px; border-radius: 3px; border: 1px solid var(--bg-hover); margin-right: 8px; vertical-align: middle; }
.theme-swatch i { font-style: normal; font-size: calc(10px * var(--fs, 1)); font-weight: bold; line-height: 1; }
.theme-check { margin-left: 8px; color: #3fb950; }

/* ===== AI 助手面板 ===== */
.term-area.with-ai { margin-right: 360px; }
.ai-panel {
  position: absolute; top: 40px; right: 0; bottom: 0; width: 360px;
  background: var(--bg-panel); border-left: 1px solid var(--border-soft);
  display: flex; flex-direction: column; z-index: 80;
}
.ai-header {
  display: flex; align-items: center; gap: 8px;
  height: 42px; padding: 0 8px 0 14px; flex-shrink: 0;
  border-bottom: 1px solid var(--border-soft);
}
.ai-title { font-size: calc(14px * var(--fs, 1)); font-weight: 600; color: var(--text-1); }
.ai-sub { font-size: calc(11px * var(--fs, 1)); color: var(--text-3); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-header-actions { display: flex; align-items: center; gap: 2px; }
.sbtn.sm { width: 26px; height: 26px; border-radius: 6px; }

.ai-settings { padding: 12px 14px; border-bottom: 1px solid var(--border-soft); display: flex; flex-direction: column; gap: 10px; background: var(--bg-rail); }
.ai-settings-tip { font-size: calc(11px * var(--fs, 1)); color: var(--text-3); line-height: 1.5; }

.ai-messages { flex: 1; overflow-y: auto; padding: 14px; display: flex; flex-direction: column; gap: 12px; }
.ai-messages::-webkit-scrollbar { width: 6px; }
.ai-messages::-webkit-scrollbar-thumb { background: #2e2e2e; border-radius: 3px; }
.ai-empty { margin: auto; text-align: center; color: var(--text-3); display: flex; flex-direction: column; gap: 10px; padding: 0 20px; }
.ai-empty p { margin: 0; font-size: calc(14px * var(--fs, 1)); }
.ai-empty-sub { font-size: calc(12px * var(--fs, 1)) !important; color: var(--text-3); line-height: 1.6; }

.ai-msg { display: flex; }
.ai-msg.user { justify-content: flex-end; }
.ai-bubble {
  max-width: 92%; font-size: calc(13px * var(--fs, 1)); line-height: 1.65; word-break: break-word;
  padding: 9px 12px; border-radius: 10px; color: var(--text-1);
}
.ai-msg.user .ai-bubble { background: rgba(47,129,247,0.18); border: 1px solid rgba(47,129,247,0.25); border-top-right-radius: 3px; }
.ai-msg.assistant .ai-bubble { background: var(--bg-elev); border: 1px solid var(--border); border-top-left-radius: 3px; }
.ai-bubble.err { color: #f85149; border-color: rgba(248,81,73,0.3); }
.ai-bubble :deep(strong) { color: var(--text-1); }
.ai-bubble :deep(code.ai-inline) {
  background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px;
  padding: 1px 5px; font-size: calc(12px * var(--fs, 1)); color: #79c0ff;
  font-family: Consolas, "Courier New", monospace;
}
.ai-code { background: var(--bg-input); border: 1px solid var(--border); border-radius: 7px; margin: 8px 0; overflow: hidden; }
.ai-code-lang {
  font-size: calc(10px * var(--fs, 1)); color: var(--text-3); padding: 4px 10px;
  background: var(--bg-panel-2); border-bottom: 1px solid var(--border); letter-spacing: 1px;
}
.ai-code pre { margin: 0; padding: 10px 12px; overflow-x: auto; }
.ai-code :deep(code) { font-size: calc(12px * var(--fs, 1)); color: #a5d6ff; font-family: Consolas, "Courier New", monospace; line-height: 1.6; }

.ai-thinking { color: var(--text-2); }
.ai-dots { display: inline-flex; gap: 3px; margin-left: 6px; }
.ai-dots i { width: 4px; height: 4px; border-radius: 50%; background: var(--accent); animation: ai-blink 1.2s infinite; }
.ai-dots i:nth-child(2) { animation-delay: .2s; }
.ai-dots i:nth-child(3) { animation-delay: .4s; }
@keyframes ai-blink { 0%, 60%, 100% { opacity: .25; } 30% { opacity: 1; } }

.ai-input-area { flex-shrink: 0; border-top: 1px solid var(--border-soft); padding: 10px 12px 12px; display: flex; flex-direction: column; gap: 8px; }
.ai-ctx-row { display: flex; align-items: center; gap: 8px; }
.ai-ctx-label { font-size: calc(11px * var(--fs, 1)); color: var(--text-3); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-input-row { display: flex; align-items: flex-end; gap: 8px; }
.ai-input-row :deep(.el-textarea__inner) {
  background: var(--bg-input); border-color: var(--border); color: var(--text-1);
  font-size: calc(13px * var(--fs, 1)); border-radius: 8px;
}
.ai-input-row :deep(.el-textarea__inner:focus) { border-color: var(--accent); }
.ai-send { height: 54px; width: 44px; border-radius: 8px; font-size: calc(17px * var(--fs, 1)); }

/* ===== 窄屏适配：设置面板归位左侧；AI 面板改为整页覆盖 ===== */
@media (max-width: 768px) {
  .settings-panel { left: 10px; }
  .term-area.with-ai { margin-right: 0; }
  .ai-panel { width: 100%; border-left: none; }
}
</style>
