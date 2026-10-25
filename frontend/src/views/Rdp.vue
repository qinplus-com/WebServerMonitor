<template>
  <div class="rdp-page" :class="{ 'in-fs': isFullscreen }">
    <!-- 顶部悬浮工具条 -->
    <div class="rdp-toolbar">
      <span class="dot" :class="statusClass"></span>
      <span class="t-status">{{ statusText }}</span>
      <span class="t-spacer"></span>
      <button class="t-btn" @click="toggleFs"><el-icon><component :is="isFullscreen ? 'Aim' : 'FullScreen'" /></el-icon>{{ isFullscreen ? '退出全屏' : '全屏' }}</button>
      <button class="t-btn danger" @click="leave"><el-icon><SwitchButton /></el-icon>断开</button>
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
        <template v-else>
          <p class="err">{{ mask }}</p>
          <button class="t-btn back" @click="leave">返回</button>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import Guacamole from 'guacamole-common-js'

const route = useRoute()
const router = useRouter()
const displayEl = ref(null)
const status = ref('connecting') // connecting / connected / closed
const mask = ref('connecting') // connecting / 错误文案 / ''（无遮罩）
const isFullscreen = ref(false)

let client = null
let mouse = null
let keyboard = null
let onFsChange = null
let onWinResize = null
let onPaste = null

const statusText = computed(() =>
  ({ connecting: '连接中', connected: '已连接', closed: '已断开' }[status.value] || status.value))
const statusClass = computed(() =>
  ({ connecting: 'wait', connected: 'ok', closed: 'err' }[status.value] || ''))

function fitScale() {
  if (!client) return
  const display = client.getDisplay()
  const dw = display.getWidth()
  const dh = display.getHeight()
  if (!dw || !dh) return
  const scale = Math.min(displayEl.value.clientWidth / dw, displayEl.value.clientHeight / dh)
  display.scale(scale > 0 ? scale : 1)
}

function connect() {
  const token = route.query.token
  if (!token) {
    mask.value = '缺少连接凭证，请从服务器管理页重新打开'
    status.value = 'closed'
    return
  }
  const w = window.innerWidth
  const h = window.innerHeight
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const tunnel = new Guacamole.WebSocketTunnel(`${proto}://${location.host}/ws/rdp?token=${encodeURIComponent(token)}&w=${w}&h=${h}`)
  client = new Guacamole.Client(tunnel)

  tunnel.onerror = () => {
    if (status.value !== 'connected') {
      mask.value = '连接失败：无法建立 WebSocket 通道（请确认网关已启动）'
      status.value = 'closed'
    }
  }
  tunnel.onstatechange = (state) => {
    if (state === Guacamole.Tunnel.OPEN) {
      status.value = 'connected'
      mask.value = ''
    } else if (state === Guacamole.Tunnel.CLOSED && status.value === 'connected') {
      status.value = 'closed'
      mask.value = '远程桌面连接已断开'
    }
  }
  client.onerror = (err) => {
    const msg = (err && err.message) || '远程桌面发生错误'
    if (status.value !== 'connected') {
      mask.value = msg
      status.value = 'closed'
    } else {
      ElMessage.error(msg)
    }
  }
  // 远端剪贴板 -> 本地
  client.onclipboard = (stream, mimetype) => {
    if (mimetype !== 'text/plain') return
    const reader = new Guacamole.StringReader(stream)
    let text = ''
    reader.ontext = (t) => { text += t }
    reader.onend = () => {
      navigator.clipboard.writeText(text).catch(() => {})
    }
  }

  displayEl.value.appendChild(client.getDisplay().getElement())
  client.connect()

  // 鼠标
  mouse = new Guacamole.Mouse(client.getDisplay().getElement())
  mouse.onmousedown = mouse.onmouseup = mouse.onmousemove = (s) => client.sendMouseState(s)

  // 键盘（挂到 document，保证工具条聚焦时按键仍可达）
  keyboard = new Guacamole.Keyboard(document)
  keyboard.onkeydown = (k) => client.sendKeyEvent(1, k)
  keyboard.onkeyup = (k) => client.sendKeyEvent(0, k)

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

  // 远端分辨率变化 / 窗口变化 -> 重新适配缩放
  client.getDisplay().onresize = fitScale
  onWinResize = () => { fitScale(); client.sendSize(window.innerWidth, window.innerHeight) }
  window.addEventListener('resize', onWinResize)
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
  onFsChange = () => { isFullscreen.value = !!document.fullscreenElement; setTimeout(fitScale, 80) }
  document.addEventListener('fullscreenchange', onFsChange)
})

onBeforeUnmount(() => {
  if (client) client.disconnect()
  if (keyboard) keyboard.onkeydown = keyboard.onkeyup = null
  if (onFsChange) document.removeEventListener('fullscreenchange', onFsChange)
  if (onWinResize) window.removeEventListener('resize', onWinResize)
  if (onPaste) document.removeEventListener('paste', onPaste)
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
  display: flex;
  align-items: center;
  justify-content: center;
}
.rdp-display :deep(canvas) {
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
  transition: opacity .3s;
}
.rdp-toolbar:hover { opacity: 1; }
.dot { width: 8px; height: 8px; border-radius: 50%; }
.dot.ok { background: #34d399; box-shadow: 0 0 6px #34d399; }
.dot.wait { background: #fbbf24; }
.dot.err { background: #f87171; }
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
