<template>
  <div class="rdp-page" :class="{ 'in-fs': isFullscreen }">
    <!-- 顶部悬浮工具条：3 秒无鼠标活动自动收起，移动鼠标唤醒 -->
    <div class="rdp-toolbar" :class="{ hide: !toolbarVisible }" @mouseenter="toolbarHover = true" @mouseleave="toolbarHover = false">
      <span class="dot" :class="statusClass"></span>
      <span class="t-status">{{ statusText }}</span>
      <el-select v-model="resSel" size="small" class="t-res-sel" @change="onResChange" @visible-change="resSelectOpen = $event" @keydown.stop>
        <el-option value="auto" label="自动" />
        <el-option v-for="o in RES_OPTIONS" :key="o" :value="o" :label="o" />
      </el-select>
      <span class="t-res">{{ resText }}</span>
      <span class="t-spacer"></span>
      <button class="t-btn" @mousedown.prevent @click="toggleFs"><el-icon><component :is="isFullscreen ? 'Aim' : 'FullScreen'" /></el-icon>{{ isFullscreen ? '退出全屏' : '全屏' }}</button>
      <button class="t-btn danger" @mousedown.prevent @click="leave"><el-icon><SwitchButton /></el-icon>断开</button>
    </div>

    <!-- 远程画面 -->
    <div ref="displayEl" class="rdp-display"></div>

    <!-- 连接前 / 异常遮罩 -->
    <div v-if="mask" class="rdp-mask">
      <div class="mask-box">
        <template v-if="mask === 'connecting'">
          <div class="spin"></div>
          <p>正在连接远程桌面，请稍候...</p>
        </template>
        <template v-else-if="mask === 'rescale'">
          <div class="spin"></div>
          <p>正在按新窗口尺寸适配分辨率...</p>
        </template>
        <template v-else>
          <p class="err">{{ mask }}</p>
          <button class="t-btn back" @click="leave">返回</button>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import Guacamole from 'guacamole-common-js'
import { rdpApi } from '../api'

// 鼠标/触摸坐标换算补丁：guacamole-common-js 的 Guacamole.Position.fromClientPosition
// 仅按 offsetLeft/offsetTop 链换算，不感知 CSS transform 缩放；而 fitSize() 对 display
// 层整体 transform 等比缩放，直接用会导致鼠标坐标按缩放倍率偏移。这里在换算入口
// 改为基于 getBoundingClientRect（transform 后的真实渲染矩形）与元素布局尺寸的
// 比例换算，缩放与否均精确（缩放为 1 时与原实现等价）。
if (!Guacamole.__rectPositionPatched) {
  Guacamole.__rectPositionPatched = true
  const origFromClient = Guacamole.Position.fromClientPosition
  Guacamole.Position.fromClientPosition = function (element, clientX, clientY) {
    const rect = element.getBoundingClientRect()
    if (rect.width > 0 && rect.height > 0 && element.offsetWidth > 0 && element.offsetHeight > 0) {
      return new Guacamole.Position({
        x: Math.round((clientX - rect.left) * (element.offsetWidth / rect.width)),
        y: Math.round((clientY - rect.top) * (element.offsetHeight / rect.height))
      })
    }
    return origFromClient(element, clientX, clientY)
  }
}

const route = useRoute()
const router = useRouter()
const displayEl = ref(null)
const status = ref('connecting') // connecting / connected / closed
const mask = ref('connecting') // connecting / rescale / 错误文案 / ''（无遮罩）
const isFullscreen = ref(false)

let client = null
let mouse = null
let keyboard = null
let curEl = null // 当前正在展示的 display 元素（seamless 切换时新旧交替）
let onFsChange = null
let onWinResize = null
let onPaste = null
let resizeTimer = null
let curW = window.innerWidth
let curH = window.innerHeight
let reconnecting = false
let rescalePending = false // 重连期间又收到分辨率调整请求：完成后补跑一次，避免竞态卡住

const statusText = computed(() =>
  ({ connecting: '连接中', connected: '已连接', closed: '已断开' }[status.value] || status.value))
const resText = ref('') // 当前远端会话分辨率（取自画布位图，随 rdpd 动态同步更新）
const statusClass = computed(() =>
  ({ connecting: 'wait', connected: 'ok', closed: 'err' }[status.value] || ''))

// 工具条自动隐藏：3 秒无鼠标活动收起，任意移动鼠标唤醒。
// 悬停在工具条上、分辨率下拉展开中、或有遮罩（连接中/错误）时不收起。
const toolbarVisible = ref(true)
const toolbarHover = ref(false)
const resSelectOpen = ref(false)
let toolbarTimer = null
let onToolbarWake = null
function scheduleToolbarHide() {
  clearTimeout(toolbarTimer)
  toolbarTimer = setTimeout(() => {
    if (!toolbarHover.value && !resSelectOpen.value && !mask.value) toolbarVisible.value = false
  }, 3000)
}
function wakeToolbar() {
  toolbarVisible.value = true
  scheduleToolbarHide()
}
// 遮罩出现（连接失败等）时强制显示工具条并暂停自动隐藏
watch(mask, (v) => {
  if (v) {
    toolbarVisible.value = true
    clearTimeout(toolbarTimer)
  } else {
    scheduleToolbarHide()
  }
})

// 画面等比缩放填满容器（允许放大，避免比例失配时出现大片黑边）；
// 实现见 fitSize()：对 display 层整体 transform 缩放 + 绝对定位居中，
// 鼠标 getBoundingClientRect 坐标换算始终精确。
function fitSize() {
  if (!curEl || !displayEl.value) return
  // 必须定位"当前会话"的画布：seamless 切换期间新旧 display 元素并存，
  // 从容器全局查找会命中旧画布，导致新画面从未被适配
  const canvas = curEl.querySelector('canvas')
  if (!canvas) return
  // 位图尺寸直接取画布自身属性：不依赖 client.getDisplay()——
  // seamless 切换期间 client 与 curEl 新旧交替，跨引用读取会拿到旧会话尺寸，
  // 造成画面变形或裁切。主画布(第一个 canvas)的 width/height 即当前会话分辨率。
  const dw = canvas.width
  const dh = canvas.height
  if (!dw || !dh) return
  const cw = displayEl.value.clientWidth
  const ch = displayEl.value.clientHeight
  if (!cw || !ch) return
  const scale = Math.min(cw / dw, ch / dh)
  // 对 display 层整体 transform 等比缩放 + 绝对定位居中：
  // - 光标层等内部画布随层同步缩放，层间坐标始终一致；
  // - flex 布局在子元素溢出时会失去居中效果（大分辨率场景），绝对定位不受影响；
  // - 鼠标坐标换算基于 getBoundingClientRect（transform 后的矩形），依然精确。
  curEl.style.position = 'absolute'
  curEl.style.left = Math.floor((cw - dw * scale) / 2) + 'px'
  curEl.style.top = Math.floor((ch - dh * scale) / 2) + 'px'
  curEl.style.width = dw + 'px'
  curEl.style.height = dh + 'px'
  curEl.style.overflow = 'hidden'
  curEl.style.transformOrigin = '0 0'
  curEl.style.transform = 'scale(' + scale + ')'
  // 清理旧方案（逐 canvas 设 CSS 宽高）的残留，交由层级缩放统一控制
  canvas.style.maxWidth = ''
  canvas.style.maxHeight = ''
  canvas.style.width = ''
  canvas.style.height = ''
}

// 彻底清理会话并清空画面（仅页面离开时使用；分辨率重连走 seamless 切换，不走这里）
function cleanupSession() {
  if (client) client.disconnect()
  client = null
  if (keyboard) keyboard.onkeydown = keyboard.onkeyup = null
  keyboard = null
  mouse = null
  curEl = null
  if (displayEl.value) displayEl.value.innerHTML = ''
}

// 分辨率选择：自动=非全屏 1920×1080、全屏本机屏幕分辨率；手动=按所选分辨率建会话
const resSel = ref('auto')
const RES_OPTIONS = ['1280×720', '1600×900', '1920×1080', '2560×1440', '3840×2160']
function onResChange() {
  requestRescale() // 当前会话已是所选分辨率时 requestRescale 会自动跳过，不重复重连
}

// 请求分辨率规则：
//   自动模式：跟随窗口实际大小（物理像素）——远端会话与显示区域 1:1 像素对应，
//             文字像素级清晰、画面充满窗口无黑边；全屏时为客户机屏幕物理分辨率。
//             窗口尺寸变化时由 requestRescale 无缝重连跟进。
//   手动模式：使用下拉所选分辨率（此时显示层等比缩放，清晰度受缩放采样影响）。
// Windows RDP 重连必然把会话改成请求的分辨率。
function desiredSize() {
  if (resSel.value !== 'auto') {
    const p = resSel.value.split('×')
    return [Number(p[0]), Number(p[1])]
  }
  const dpr = window.devicePixelRatio || 1
  if (isFullscreen.value) {
    return [Math.round(window.screen.width * dpr), Math.round(window.screen.height * dpr)]
  }
  return [Math.round(window.innerWidth * dpr), Math.round(window.innerHeight * dpr)]
}

// startSession 建立会话。
// seamless=false：首次连接，画面就绪前显示遮罩；
// seamless=true ：分辨率重连，旧画面保留继续显示，新会话在后台建立，
//                 远端新分辨率首帧就绪后瞬间替换画面——无遮罩无黑屏。
function startSession(token, seamless) {
  const oldClient = client
  const oldEl = curEl
  const req = desiredSize()
  curW = req[0]
  curH = req[1]
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const tunnel = new Guacamole.WebSocketTunnel(`${proto}://${location.host}/ws/rdp?token=${encodeURIComponent(token)}&w=${curW}&h=${curH}`)
  const newClient = new Guacamole.Client(tunnel)
  client = newClient

  tunnel.onerror = () => {
    if (client !== newClient) return // 旧会话退役，静默
    reconnecting = false
    if (seamless) {
      // 旧画面仍在，静默回退到原会话
      status.value = 'connected'
      ElMessage.error('分辨率适配失败，保持当前画面')
    } else {
      mask.value = '连接失败：无法建立 WebSocket 通道（请确认网关已启动）'
      status.value = 'closed'
    }
  }
  tunnel.onstatechange = (state) => {
    if (state === Guacamole.Tunnel.State.OPEN) {
      status.value = 'connected'
      mask.value = ''
      reconnecting = false
    } else if (state === Guacamole.Tunnel.State.CLOSED && client === newClient) {
      if (seamless) {
        // 新会话失败：移除残留的隐藏元素，旧画面继续使用
        el.remove()
        status.value = 'connected'
      } else {
        status.value = 'closed'
        mask.value = '远程桌面连接已断开'
      }
    }
  }
  newClient.onerror = (err) => {
    if (client !== newClient) return // 旧会话退役，静默
    const msg = (err && err.message) || '远程桌面发生错误'
    if (seamless) {
      reconnecting = false
      el.remove()
      status.value = 'connected'
      ElMessage.error('分辨率适配失败，保持当前画面')
      return
    }
    if (status.value !== 'connected') {
      mask.value = msg
      status.value = 'closed'
    } else {
      ElMessage.error(msg)
    }
  }
  // 远端剪贴板 -> 本地
  newClient.onclipboard = (stream, mimetype) => {
    if (mimetype !== 'text/plain') return
    const reader = new Guacamole.StringReader(stream)
    let text = ''
    reader.ontext = (t) => { text += t }
    reader.onend = () => {
      navigator.clipboard.writeText(text).catch(() => {})
    }
  }

  const el = newClient.getDisplay().getElement()
  // 远端分辨率就绪/变化 -> seamless 时完成画面替换，随后贴合容器
  newClient.getDisplay().onresize = () => {
    // 工具条实时显示远端会话真实分辨率（画布位图尺寸）
    const c = curEl && curEl.querySelector('canvas')
    if (c && c.width > 0) resText.value = c.width + '×' + c.height
    if (seamless && el.style.display === 'none') {
      if (oldClient) oldClient.disconnect()
      if (oldEl && oldEl.parentNode) oldEl.remove()
      el.style.display = ''
      curEl = el
      status.value = 'connected'
      reconnecting = false
      // 重连期间若又来了分辨率调整请求（如快速进出全屏），补跑一次
      if (rescalePending) {
        rescalePending = false
        setTimeout(requestRescale, 100)
      }
    }
    fitSize()
  }
  if (seamless) {
    el.style.display = 'none'
    displayEl.value.appendChild(el)
  } else {
    if (oldEl && oldEl.parentNode) oldEl.remove()
    curEl = el
    displayEl.value.appendChild(el)
  }
  newClient.connect()

  // 鼠标（绑定到当前 display 元素；seamless 切换后即用新元素）
  mouse = new Guacamole.Mouse(el)
  mouse.onmousedown = mouse.onmouseup = mouse.onmousemove = (s) => client.sendMouseState(s)

  // 键盘只挂一次（document 级），onkeydown 闭包读取全局 client，会话切换后自动指向新会话
  if (!keyboard) {
    keyboard = new Guacamole.Keyboard(document)
    keyboard.onkeydown = (k) => { if (client) client.sendKeyEvent(1, k) }
    keyboard.onkeyup = (k) => { if (client) client.sendKeyEvent(0, k) }
  }
}

// 全屏切换 / 分辨率规则变化后的重连（seamless：旧画面保留，后台重建会话）。
// 非全屏固定 1920×1080，全屏用客户机屏幕分辨率；窗口拖动不改变会话分辨率，
// 仅 fitSize 等比缩放显示。目标尺寸与当前会话差异不足 80px 时不重连。
function requestRescale() {
  if (reconnecting) {
    rescalePending = true
    return
  }
  if (status.value !== 'connected') return
  const req = desiredSize()
  if (Math.abs(req[0] - curW) < 80 && Math.abs(req[1] - curH) < 80) return
  reconnecting = true
  status.value = 'connecting' // 仅工具条状态点提示，不遮画面
  rdpApi.token(route.params.id).then(res => {
    const token = res.data && res.data.token
    if (!token) {
      reconnecting = false
      status.value = 'connected'
      ElMessage.error('无法获取连接凭证，保持当前画面')
      return
    }
    startSession(token, true)
  }).catch(() => {
    reconnecting = false
    status.value = 'connected'
    ElMessage.error('分辨率适配失败，保持当前画面')
  })
}

function connect() {
  const token = route.query.token
  if (!token) {
    mask.value = '缺少连接凭证，请从服务器管理页重新打开'
    status.value = 'closed'
    return
  }
  startSession(token, false)

  // 本地 -> 远端剪贴板（粘贴时同步）
  onPaste = (e) => {
    const text = e.clipboardData && e.clipboardData.getData('text/plain')
    if (!text || !client) return
    const stream = client.createClipboardStream('text/plain')
    const writer = new Guacamole.StringWriter(stream)
    writer.sendText(text)
    writer.sendEnd()
  }
  document.addEventListener('paste', onPaste)

  onWinResize = () => {
    fitSize()
    clearTimeout(resizeTimer)
    resizeTimer = setTimeout(requestRescale, 1500)
  }
  window.addEventListener('resize', onWinResize)

  // 工具条鼠标唤醒：任意移动鼠标重新计时并显示
  onToolbarWake = wakeToolbar
  document.addEventListener('mousemove', onToolbarWake)
  scheduleToolbarHide()
}

function toggleFs() {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen().catch(() => ElMessage.error('进入全屏失败'))
  } else {
    document.exitFullscreen()
  }
}

function leave() {
  if (client) client.disconnect()
  router.push('/servers')
}

onMounted(() => {
  connect()
  onFsChange = () => {
    isFullscreen.value = !!document.fullscreenElement
    setTimeout(fitSize, 120) // 等全屏布局稳定后再贴合
    setTimeout(requestRescale, 400) // 全屏切换后按新规则切换会话分辨率（1920×1080 ↔ 客户机屏幕）
  }
  document.addEventListener('fullscreenchange', onFsChange)
})

onBeforeUnmount(() => {
  cleanupSession()
  clearTimeout(resizeTimer)
  clearTimeout(toolbarTimer)
  if (keyboard) keyboard.onkeydown = keyboard.onkeyup = null
  if (onFsChange) document.removeEventListener('fullscreenchange', onFsChange)
  if (onWinResize) window.removeEventListener('resize', onWinResize)
  if (onPaste) document.removeEventListener('paste', onPaste)
  if (onToolbarWake) document.removeEventListener('mousemove', onToolbarWake)
})
</script>

<style scoped>
.rdp-page {
  position: fixed;
  inset: 0;
  background: #0b0f14;
  overflow: hidden;
}
.rdp-display {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background: #0b0f14;
}
.rdp-display :deep(canvas) {
  /* 尺寸与缩放统一由 fitSize() 控制（对 display 层整体 transform 缩放），
     此处仅做外观；禁止 CSS 自动缩放以免与 JS 尺寸互相覆盖 */
  box-shadow: 0 0 40px rgba(0, 0, 0, .6);
  border-radius: 4px;
}

/* 工具条：玻璃拟态悬浮 */
.rdp-toolbar {
  position: absolute;
  top: 10px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 30;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 14px;
  border-radius: 18px;
  background: rgba(15, 22, 32, .78);
  border: 1px solid rgba(255, 255, 255, .12);
  backdrop-filter: blur(10px);
  color: #cfd8e3;
  font-size: 13px;
  box-shadow: 0 4px 18px rgba(0, 0, 0, .45);
  transition: transform .28s ease, opacity .28s ease;
}
/* 自动隐藏收起态：移出屏幕顶部并禁用交互，移动鼠标唤醒 */
.rdp-toolbar.hide {
  transform: translateX(-50%) translateY(-160%);
  opacity: 0;
  pointer-events: none;
}
.dot { width: 8px; height: 8px; border-radius: 50%; }
.dot.ok { background: #34d399; box-shadow: 0 0 6px #34d399; }
.dot.wait { background: #fbbf24; }
.dot.err { background: #f87171; }
.t-res-sel { width: 118px; }
.t-res-sel :deep(.el-select__wrapper) {
  min-height: 24px;
  padding: 0 8px;
  background: rgba(255, 255, 255, .06);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, .16) inset;
}
.t-res-sel :deep(.el-select__placeholder),
.t-res-sel :deep(.el-select__selected-item) { color: #e5edf5; font-size: 12px; }
.t-res { color: #9fb3c8; font-size: 12px; min-width: 72px; }
.t-spacer { flex: 0 0 6px; }
.t-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: 1px solid rgba(255, 255, 255, .16);
  background: rgba(255, 255, 255, .06);
  color: #e5edf5;
  border-radius: 12px;
  padding: 3px 10px;
  font-size: 12px;
  cursor: pointer;
  transition: all .2s;
}
.t-btn:hover { background: rgba(255, 255, 255, .14); }
.t-btn.danger { border-color: rgba(248, 113, 113, .4); color: #fca5a5; }
.t-btn.danger:hover { background: rgba(248, 113, 113, .18); }

/* 遮罩 */
.rdp-mask {
  position: absolute;
  inset: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(8, 11, 16, .82);
  backdrop-filter: blur(4px);
}
.mask-box { text-align: center; color: #cfd8e3; font-size: 15px; }
.mask-box .err { color: #fca5a5; margin-bottom: 16px; }
.t-btn.back { padding: 6px 18px; font-size: 13px; }
.spin {
  width: 34px;
  height: 34px;
  margin: 0 auto 14px;
  border-radius: 50%;
  border: 3px solid rgba(255, 255, 255, .12);
  border-top-color: #4f9cff;
  animation: spin .9s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
