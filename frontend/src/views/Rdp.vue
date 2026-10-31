<template>
  <div class="rdp-page" :class="{ 'in-fs': isFullscreen }" @dragover.prevent @drop.prevent="onPageDrop">
    <!-- 顶部悬浮工具条：3 秒无鼠标活动自动收起，顶部热区/移动鼠标唤醒 -->
    <!-- 隐藏后的唤出热区：顶部中央窄条常驻，悬停即唤出工具栏 -->
    <div v-if="!toolbarVisible" class="tb-strip" title="显示工具栏"></div>
    <div class="rdp-toolbar" :class="{ hide: !toolbarVisible }" @mouseenter="toolbarHover = true" @mouseleave="toolbarHover = false">
      <span class="dot" :class="statusClass"></span>
      <span class="t-status">{{ statusText }}</span>
      <el-select v-if="!isVnc" ref="resSelRef" v-model="resSel" size="small" class="t-res-sel" @change="onResChange" @visible-change="(v) => onSelVisible(v, resSelRef)" @keydown.stop>
        <el-option value="auto" label="自动" />
        <el-option v-for="o in RES_OPTIONS" :key="o" :value="o" :label="o" />
      </el-select>
      <!-- VNC：点击切换远端桌面分辨率（SSH 执行 xrandr，画面随 DesktopSize 自动跟随） -->
      <el-select v-else ref="vncResSelRef" v-model="vncResSel" size="small" class="t-res-sel" :loading="vncResLoading" :disabled="status !== 'connected'" placeholder="分辨率" @visible-change="(v) => onVncResOpen(v, vncResSelRef)" @change="onVncResChange" @keydown.stop>
        <el-option v-for="o in vncResOptions" :key="o" :value="o" :label="o" />
      </el-select>
      <span class="t-res">{{ resText }}</span>
      <!-- 画质/帧率档位（对齐商用远控"手动挡"）：变更后无缝重连会话生效 -->
      <el-select ref="qualitySelRef" v-model="qualitySel" size="small" class="t-q-sel" title="画质档位（变更后自动重连）" @change="onQualityChange" @visible-change="(v) => onSelVisible(v, qualitySelRef)" @keydown.stop>
        <el-option value="fluent" label="流畅" />
        <el-option value="normal" label="超清" />
        <el-option value="high" label="原画" />
      </el-select>
      <el-select ref="fpsSelRef" v-model="fpsSel" size="small" class="t-fps-sel" title="帧率上限（变更后自动重连）" @change="onFpsChange" @visible-change="(v) => onSelVisible(v, fpsSelRef)" @keydown.stop>
        <el-option :value="30" label="30帧" />
        <el-option :value="60" label="60帧" />
      </el-select>
      <span class="t-spacer"></span>
      <button v-if="!isVnc" class="t-btn" @mousedown.prevent @click="toggleFilesPanel"><el-icon><component :is="'FolderOpened'" /></el-icon>文件传输</button>
      <button class="t-btn" @mousedown.prevent @click="toggleFs"><el-icon><component :is="isFullscreen ? 'Aim' : 'FullScreen'" /></el-icon>{{ isFullscreen ? '退出全屏' : '全屏' }}</button>
      <button class="t-btn danger" @mousedown.prevent @click="leave"><el-icon><SwitchButton /></el-icon>断开</button>
    </div>

    <!-- 文件传输面板（Windows RDP）：经目标机 SMB 共享文件夹中转，实现本地与远程桌面文件互传 -->
    <div v-if="filesPanel" class="rdp-files" @mousedown.stop @click.stop @dragover.prevent @drop.prevent="onTransferDrop">
      <div class="f-head">
        <span>文件传输</span>
        <button class="t-btn" @mousedown.prevent @click="toggleFilesPanel"><el-icon><component :is="'Close'" /></el-icon></button>
      </div>
      <div class="f-upload" @click="transferInput && transferInput.click()">
        <el-icon class="f-upload-icon"><component :is="'Upload'" /></el-icon>
        <span>点击选择文件上传到远程桌面</span>
        <span class="f-prog" v-if="transferUploading">上传中 {{ transferDone }}/{{ transferTotal }}</span>
      </div>
      <div class="f-list" v-loading="transferLoading">
        <div v-for="f in transferFiles" :key="f.name" class="f-item">
          <el-icon :class="f.is_dir ? 'is-dir' : ''"><component :is="f.is_dir ? 'Folder' : 'Document'" /></el-icon>
          <span class="f-name" :title="f.name">{{ f.name }}</span>
          <span class="f-size">{{ f.is_dir ? '' : fmtSize(f.size) }}</span>
          <button v-if="!f.is_dir" class="t-btn" @mousedown.prevent @click="downloadTransfer(f)"><el-icon><component :is="'Download'" /></el-icon>下载</button>
        </div>
        <p v-if="!transferLoading && !transferFiles.length" class="f-empty">共享文件夹为空</p>
      </div>
      <p class="f-tip">文件经目标机共享文件夹中转：上传后在远程桌面内打开 \\localhost\共享名 即可使用；下载仅支持共享根目录文件，深层目录请用「文件」管理页。</p>
      <input ref="transferInput" type="file" multiple hidden @change="onTransferPicked" />
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
import { ElMessage, ElMessageBox } from 'element-plus'
import Guacamole from 'guacamole-common-js'
import { rdpApi, sftpApi } from '../api'

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
// VNC 会话（Linux 服务器）：画布跟随远端桌面尺寸，不支持分辨率切换重连
const isVnc = route.query.proto === 'vnc'
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
let onVisChange = null
let onTabGuard = null
let resizeTimer = null
let curW = window.innerWidth
let curH = window.innerHeight
let reconnecting = false
let rescalePending = false // 重连期间又收到分辨率调整请求：完成后补跑一次，避免竞态卡住

// ===== H.264 视频通道（WebCodecs）=====
// 检测通过后 WS URL 携带 codec=h264，rdpd 启用 ffmpeg 帧间压缩管线（h264.go），
// 以 h264 指令逐帧下发 Annex-B 码流，前端解码后直绘默认层画布——相比 tile 管线，
// 全屏连续变化（拖拽/滚动）场景带宽降低一个数量级，帧延迟更稳定。
// 任一环节不支持/失败均静默回退传统 tile 管线（rdpd 端 ffmpeg 缺失时同样回退）。
let h264OK = false
let h264State = null // 当前会话解码状态 { decoder, kf, close }
async function detectH264() {
  if (typeof VideoDecoder === 'undefined' || typeof EncodedVideoChunk === 'undefined') return false
  try {
    // baseline profile（avc1.42002a）：rdpd 端 x264 以 baseline/ultrafast 编码，兼容性最佳
    const sup = await VideoDecoder.isConfigSupported({ codec: 'avc1.42002a', optimizeForLatency: true })
    return !!sup.supported
  } catch (e) { return false }
}

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
// 工具栏下拉组件引用：下拉关闭时 blur 自身，防止焦点滞留劫持远端键盘输入
const resSelRef = ref(null)
const vncResSelRef = ref(null)
const qualitySelRef = ref(null)
const fpsSelRef = ref(null)
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

// 连接失败：弹出带解决方案的对话框（不再是转瞬即逝的 toast）。
// Linux VNC 与 Windows RDP 的排查步骤不同，按协议分别给出。
function showConnectHelp(msg) {
  const tips = isVnc
    ? [
        '1. 目标机安装并启动 VNC 服务：CentOS 执行 yum install tigervnc-server -y，运行 vncpasswd 设置密码（与平台登录密码一致）后执行 vncserver :1 启动',
        '2. 防火墙放行 VNC 端口：firewall-cmd --add-port=5901/tcp --permanent && firewall-cmd --reload',
        '3. 确认服务器编辑中的 VNC 端口与目标机一致（:1 对应 5901，:2 对应 5902）',
        '4. 详细部署步骤见平台文档《Linux远程桌面(VNC).md》'
      ]
    : [
        '1. 确认目标机已启用远程桌面（系统属性 → 远程 → 允许远程连接到此计算机）',
        '2. 目标机防火墙放行 3389 端口',
        '3. 确认登录账号密码正确（账号需有远程桌面权限）'
      ]
  ElMessageBox.alert(msg + '\n\n' + tips.join('\n'), '远程桌面连接失败', {
    confirmButtonText: '知道了',
    customClass: 'msgbox-pre'
  }).catch(() => {})
}

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
  if (h264State) { h264State.close(); h264State = null }
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

// ===== 画质/帧率档位（对齐商用远控的"手动挡"）=====
// quality: fluent=流畅(CRF25,带宽敏感) / normal=超清(CRF21) / high=原画(CRF17,近无损)
// fps: 渲染帧率上限 30/60（60 帧拖拽/滚动更顺滑，CPU/带宽相应增加）
// 编码参数在会话建立时确定，变更后无缝重连（seamless：旧画面保留至新会话就绪）。
// 默认流畅档：带宽敏感的 Linux 服务器场景体验最优，需要更清晰可手动切超清/原画。
// 存储键升级 v2：旧键残留的旧默认"超清"会掩盖新默认，键名换代使新默认全员生效
const qualitySel = ref(localStorage.getItem('rdp_quality2') || 'fluent')
const fpsSel = ref(Number(localStorage.getItem('rdp_fps')) || 30)
function onQualityChange(v) {
  localStorage.setItem('rdp_quality2', v)
  reconnectForSettings()
}
function onFpsChange(v) {
  localStorage.setItem('rdp_fps', String(v))
  reconnectForSettings()
}
// 设置变更重连：与 requestRescale 同一套 seamless 流程，但不比较尺寸差异
function reconnectForSettings() {
  if (reconnecting) {
    rescalePending = true
    return
  }
  if (status.value !== 'connected') return
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
    ElMessage.error('设置应用失败，保持当前画面')
  })
}

// ===== VNC 分辨率下拉：查询/切换目标机 VNC 桌面分辨率（后端 SSH 执行 xrandr） =====
// 切换后 Xvnc 推送 DesktopSize，rdpd 同步前端画布尺寸，resText 随之更新，无需重连。
const vncResSel = ref('')
const vncResOptions = ref([])
const vncResLoading = ref(false)
let vncResLoaded = false
let vncResPrev = '' // 最近一次确认有效的分辨率（切换失败时回退显示）

async function loadVncResolutions() {
  vncResLoading.value = true
  try {
    const res = await rdpApi.vncResolutions(serverId)
    vncResOptions.value = res.data.modes || []
    const cur = (res.data.current || '').replace(/×/g, 'x')
    if (cur) {
      vncResSel.value = cur
      vncResPrev = cur
    }
    vncResLoaded = true
  } catch (e) {
    ElMessage.error(e.message || '获取分辨率列表失败')
  } finally {
    vncResLoading.value = false
  }
}

function onVncResOpen(open, sel) {
  onSelVisible(open, sel)
  if (open && !vncResLoaded && status.value === 'connected') loadVncResolutions()
}

// 工具栏下拉关闭即失焦：焦点滞留在 el-select 内时（组件有 @keydown.stop），
// 终端按键全部被本地吞掉——远端收不到字符（表现为命令未执行），且回车/
// 方向键会再次弹开下拉菜单、误触分辨率切换。关闭即 blur，键盘控制权交还远端。
function onSelVisible(v, sel) {
  resSelectOpen.value = v
  if (!v && sel && typeof sel.blur === 'function') sel.blur()
}

async function onVncResChange(v) {
  if (!v || v === vncResPrev) return
  const prev = vncResPrev
  vncResLoading.value = true
  try {
    await rdpApi.vncResolution({ server_id: serverId, resolution: v })
    vncResPrev = v
    ElMessage.success('分辨率已切换为 ' + v + '，画面即将自动适配')
  } catch (e) {
    ElMessage.error(e.message || '切换分辨率失败')
    vncResSel.value = prev
  } finally {
    vncResLoading.value = false
  }
}

// 画布尺寸变化（含切换分辨率后的 DesktopSize）同步到下拉当前值
watch(resText, (v) => {
  if (!v) return
  const norm = v.replace(/×/g, 'x')
  vncResSel.value = norm
  vncResPrev = norm
})

// 请求分辨率规则：
//   自动模式：跟随窗口实际大小；HiDPI（devicePixelRatio）不放大请求——会话像素量
//             直接决定服务端编码/网络传输/解码的每一环延迟，像素翻倍=延迟翻倍，
//             而高倍率下的清晰度收益边际递减；会话总像素超过 1920×1200（2.3MP）
//             时按比例收缩，显示层再等比适配窗口。
//   手动模式：使用下拉所选分辨率（显示层等比缩放适配窗口）。
// Windows RDP 重连必然把会话改成请求的分辨率。
function desiredSize() {
  if (resSel.value !== 'auto') {
    const p = resSel.value.split('×')
    return [Number(p[0]), Number(p[1])]
  }
  const dpr = window.devicePixelRatio || 1
  let w, h
  if (isFullscreen.value) {
    w = Math.round(window.screen.width * dpr)
    h = Math.round(window.screen.height * dpr)
  } else {
    w = Math.round(window.innerWidth * dpr)
    h = Math.round(window.innerHeight * dpr)
  }
  const cap = 1920 * 1200
  if (w * h > cap) {
    const s = Math.sqrt(cap / (w * h))
    w = Math.round(w * s)
    h = Math.round(h * s)
  }
  return [w, h]
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
  const tunnel = new Guacamole.WebSocketTunnel(`${proto}://${location.host}/ws/rdp?token=${encodeURIComponent(token)}&w=${curW}&h=${curH}` + (h264OK ? '&codec=h264' : '') + `&quality=${qualitySel.value}&fps=${fpsSel.value}`)
  const newClient = new Guacamole.Client(tunnel)
  client = newClient

  // ===== H.264 视频通道：解码直绘默认层画布 =====
  // 视频模式下 rdpd 不再发 png/jpeg tile，默认层画布无其他绘制者，直接绘入；
  // 光标层位于默认层之上，本地光标不受影响。
  const h264Prev = h264State // 旧会话解码状态（seamless 切换在画面替换时关闭）
  if (h264OK) {
    const canvas = newClient.getDisplay().getDefaultLayer().getCanvas()
    const st = { decoder: null, ts: 0, lastKf: 0, fresh: false }
    st.close = () => { try { if (st.decoder) st.decoder.close() } catch (e) { /* 已关闭 */ } st.decoder = null }
    st.kf = (force) => {
      // 请求关键帧：rdpd 重建编码器，新码流首帧必为 IDR。节流防连续触发；
      // 解码器重建/关键帧丢失场景 force 强制发送（节流会吞掉重同步请求，
      // 导致新解码器一直收 delta 帧、参考缺失输出绿屏）
      const now = Date.now()
      if (!force && now - st.lastKf < 3000) return
      st.lastKf = now
      try { tunnel.sendMessage(['keyframe']) } catch (e) { /* 通道已关闭 */ }
    }
    const setupDecoder = () => {
      st.fresh = true // 新解码器未建立参考链：必须先拿到关键帧
      st.decoder = new VideoDecoder({
        output: (vf) => {
          try { canvas.getContext('2d').drawImage(vf, 0, 0, vf.displayWidth, vf.displayHeight) } catch (e) { /* 画布瞬态未就绪，下一帧自愈 */ }
          vf.close()
        },
        error: () => {
          // 解码链损坏：重建解码器并强制请求关键帧重新同步
          st.close()
          setupDecoder()
          st.kf(true)
        }
      })
      st.decoder.configure({ codec: 'avc1.42002a', optimizeForLatency: true })
    }
    setupDecoder()
    h264State = st
    // 拦截 h264 指令（Client 构造时已挂载自身处理器，此处包装透传其余指令）
    const clientHandler = tunnel.oninstruction
    tunnel.oninstruction = (opcode, args) => {
      if (opcode === 'h264') {
        const b64 = args && args[0]
        if (b64 && st.decoder && st.decoder.state === 'configured') {
          const bin = atob(b64)
          const bytes = new Uint8Array(bin.length)
          for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
          // 首个 VCL NAL 为 IDR(5) 或携带 SPS(7) 判为关键帧（rdpd 按访问单元逐帧下发）
          let isKey = false
          for (let i = 0; i + 3 < bytes.length; i++) {
            if (bytes[i] === 0 && bytes[i + 1] === 0 && bytes[i + 2] === 1) {
              const t = bytes[i + 3] & 0x1F
              if (t === 5 || t === 7) { isKey = true; break }
            }
          }
          if (st.fresh && !isKey) {
            // 解码器新建/重建后首个到达帧不是关键帧：configure 异步窗口期的
            // 关键帧已被丢弃，直接解 delta 会因参考缺失输出绿帧——丢弃并
            // 强制请求关键帧重新同步
            st.kf(true)
            return
          }
          st.fresh = false
          try {
            st.decoder.decode(new EncodedVideoChunk({ type: isKey ? 'key' : 'delta', timestamp: st.ts, data: bytes }))
            st.ts += 33333
          } catch (e) { st.kf(true) }
        }
        return
      }
      clientHandler(opcode, args)
    }
  }

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
    showConnectHelp(msg)
    if (status.value !== 'connected') {
      mask.value = msg
      status.value = 'closed'
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
  // 远端复制的文件（MS-RDPECLIP FileGroupDescriptorW）-> 浏览器下载
  newClient.onfile = (stream, mimetype, name) => {
    if (isVnc) return
    const reader = new Guacamole.BlobReader(stream, mimetype)
    reader.onend = () => {
      clipFileBatch.push({ name, blob: reader.getBlob() })
      clearTimeout(clipBatchTimer)
      // 同一次远端复制的多个文件连续到达，静默 500ms 视为一批
      clipBatchTimer = setTimeout(flushClipFiles, 500)
    }
  }

  const el = newClient.getDisplay().getElement()
  // 远端分辨率就绪/变化 -> seamless 时完成画面替换，随后贴合容器
  newClient.getDisplay().onresize = () => {
    // 工具条实时显示远端会话真实分辨率：读 display 逻辑尺寸——
    // 画布物理尺寸会被 guacamole 对齐到 64 的倍数（如 1080→1088），不是真实分辨率
    const disp = newClient.getDisplay()
    if (disp.getWidth() > 0) resText.value = disp.getWidth() + '×' + disp.getHeight()
    if (seamless && el.style.display === 'none') {
      if (oldClient) oldClient.disconnect()
      if (h264Prev) h264Prev.close() // 旧会话画面已替换，关闭其解码器
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
  if (isVnc) return // VNC 画布跟随远端桌面，无需重连适配
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

async function connect() {
  const token = route.query.token
  if (!token) {
    mask.value = '缺少连接凭证，请从服务器管理页重新打开'
    status.value = 'closed'
    return
  }
  h264OK = await detectH264() // WebCodecs 检测：决定是否请求 h264 视频通道
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

  // 标签页重新可见时请求关键帧：隐藏期间浏览器可能暂停渲染/丢弃解码帧，
  // 恢复后以 IDR 重建参考链，避免花屏（tile 管线无此问题，请求无副作用）
  onVisChange = () => { if (!document.hidden && h264State) h264State.kf() }
  document.addEventListener('visibilitychange', onVisChange)

  onWinResize = () => {
    fitSize()
    clearTimeout(resizeTimer)
    resizeTimer = setTimeout(requestRescale, 1500)
  }
  window.addEventListener('resize', onWinResize)

  // 工具条鼠标唤醒：guacamole 画布对 mousemove 调用 stopPropagation，
  // 冒泡阶段挂在 document 上收不到画布内的事件（隐藏后唤不出来的根因），
  // 必须用捕获阶段监听。隐藏时仅顶部热区（≤48px）唤出，避免操作画面时误弹出；
  // 显示时任意移动重置 3 秒计时。
  onToolbarWake = (e) => {
    if (e.clientY <= 48) {
      wakeToolbar()
    } else if (toolbarVisible.value) {
      scheduleToolbarHide()
    }
  }
  document.addEventListener('mousemove', onToolbarWake, true)

  // Tab 守卫：远端终端的 Tab 补全不应把本地焦点移进工具栏（焦点一旦落进
  // el-select，按键被 @keydown.stop 吞掉且回车会弹开下拉）。仅阻止浏览器
  // 默认的焦点移动，事件仍冒泡至 Guacamole.Keyboard 正常下发远端；
  // 输入类控件（剪贴板/文件面板等）聚焦时不拦截。
  onTabGuard = (e) => {
    if (e.key !== 'Tab') return
    const el = document.activeElement
    if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)) return
    e.preventDefault()
  }
  document.addEventListener('keydown', onTabGuard)
  scheduleToolbarHide()
}

function toggleFs() {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen().catch(() => ElMessage.error('进入全屏失败'))
  } else {
    document.exitFullscreen()
  }
}

// ===== 文件传输面板（Windows RDP）：经 SMB 共享文件夹实现本地与远程桌面文件互传 =====
const serverId = Number(route.params.id) || 0
const filesPanel = ref(false)
const transferFiles = ref([])
const transferLoading = ref(false)
const transferUploading = ref(false)
const transferTotal = ref(0)
const transferDone = ref(0)
const transferInput = ref(null)

function toggleFilesPanel() {
  filesPanel.value = !filesPanel.value
  // 面板打开期间工具条保持常显（否则收起后只能靠面板自身按钮关闭）
  toolbarHover.value = filesPanel.value
  if (filesPanel.value) loadTransferFiles()
}

async function loadTransferFiles() {
  transferLoading.value = true
  try {
    // silent：错误不走全局 toast，用对话框完整展示解决方案（如未配置共享名）
    const res = await sftpApi.list({ server_id: serverId, path: '/' }, { silent: true })
    transferFiles.value = res.data.files || []
  } catch (e) {
    filesPanel.value = false
    toolbarHover.value = false
    ElMessageBox.alert(e.message || '读取共享文件夹失败', '文件传输', {
      confirmButtonText: '知道了',
      customClass: 'msgbox-pre'
    }).catch(() => {})
  } finally { transferLoading.value = false }
}

async function uploadTransferFiles(fileList) {
  const files = Array.from(fileList || [])
  if (!files.length || transferUploading.value) return
  transferUploading.value = true
  transferTotal.value = files.length
  transferDone.value = 0
  let failed = 0
  for (const f of files) {
    try {
      const fd = new FormData()
      fd.append('file', f)
      fd.append('server_id', serverId)
      fd.append('target_path', '/')
      await sftpApi.upload(fd)
    } catch (e) { failed++ }
    transferDone.value++
  }
  transferUploading.value = false
  if (failed === 0) ElMessage.success(`已上传 ${files.length} 个文件到共享文件夹`)
  else ElMessage.warning(`成功 ${files.length - failed} 个，失败 ${failed} 个`)
  loadTransferFiles()
}

function onTransferPicked(e) {
  uploadTransferFiles(e.target.files)
  e.target.value = ''
}

function onTransferDrop(e) {
  uploadTransferFiles(e.dataTransfer && e.dataTransfer.files)
}

function fmtSize(n) {
  if (n == null) return ''
  if (n < 1024) return n + ' B'
  if (n < 1048576) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB'
  return (n / 1073741824).toFixed(2) + ' GB'
}

async function downloadTransfer(f) {
  try {
    const res = await sftpApi.download({ server_id: serverId, file_path: '/' + f.name })
    const url = URL.createObjectURL(res.data)
    const a = document.createElement('a')
    a.href = url
    a.download = f.name
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    ElMessage.error(e.message || '下载失败')
  }
}

// ===== 远端剪贴板文件下载（MS-RDPECLIP） =====
let clipFileBatch = []
let clipBatchTimer = null

function flushClipFiles() {
  const files = clipFileBatch
  clipFileBatch = []
  if (!files.length) return
  const list = files.map(f => f.name).join('\n')
  ElMessageBox.confirm(
    `远程桌面复制了 ${files.length} 个文件到剪贴板：\n${list}\n\n是否下载到本地？`,
    '远程剪贴板文件',
    { confirmButtonText: '下载到本地', cancelButtonText: '忽略', customClass: 'msgbox-pre' }
  ).then(() => {
    for (const f of files) {
      const url = URL.createObjectURL(f.blob)
      const a = document.createElement('a')
      a.href = url
      a.download = (f.name || 'clipboard-file').replace(/[\\/:*?"<>|]/g, '_')
      a.click()
      setTimeout(() => URL.revokeObjectURL(url), 5000)
    }
  }).catch(() => {})
}

// 页面级拖拽：本地文件拖入桌面页 = 上传到远程（经共享文件夹中转）
function onPageDrop(e) {
  if (isVnc) return
  const dt = e.dataTransfer
  if (dt && dt.files && dt.files.length) {
    filesPanel.value = true
    toolbarHover.value = true
    loadTransferFiles()
    uploadTransferFiles(dt.files)
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
  if (onVisChange) document.removeEventListener('visibilitychange', onVisChange)
  if (onToolbarWake) document.removeEventListener('mousemove', onToolbarWake, true)
  if (onTabGuard) document.removeEventListener('keydown', onTabGuard)
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
/* 自动隐藏收起态：移出屏幕顶部并禁用交互，顶部热区/移动鼠标唤醒 */
.rdp-toolbar.hide {
  transform: translateX(-50%) translateY(-160%);
  opacity: 0;
  pointer-events: none;
}
/* 隐藏后的唤出热区：顶部中央常驻窄条，悬停唤出工具栏 */
.tb-strip {
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 140px;
  height: 10px;
  z-index: 29;
  border-radius: 0 0 10px 10px;
  background: rgba(255, 255, 255, .16);
  box-shadow: 0 1px 4px rgba(0, 0, 0, .35);
  cursor: pointer;
  transition: background .2s ease, height .2s ease;
}
.tb-strip:hover {
  background: rgba(255, 255, 255, .4);
  height: 16px;
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
.t-q-sel { width: 78px; }
.t-fps-sel { width: 82px; }
.t-q-sel :deep(.el-select__wrapper),
.t-fps-sel :deep(.el-select__wrapper) {
  min-height: 24px;
  padding: 0 8px;
  background: rgba(255, 255, 255, .06);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, .16) inset;
}
.t-q-sel :deep(.el-select__placeholder),
.t-q-sel :deep(.el-select__selected-item),
.t-fps-sel :deep(.el-select__placeholder),
.t-fps-sel :deep(.el-select__selected-item) { color: #e5edf5; font-size: 12px; }
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

/* 文件传输面板：右上角悬浮抽屉，风格与工具条一致 */
.rdp-files {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 40;
  width: 340px;
  max-height: calc(100% - 20px);
  display: flex;
  flex-direction: column;
  border-radius: 12px;
  background: rgba(15, 22, 32, .92);
  border: 1px solid rgba(255, 255, 255, .12);
  backdrop-filter: blur(10px);
  color: #cfd8e3;
  font-size: 13px;
  box-shadow: 0 4px 18px rgba(0, 0, 0, .45);
}
.f-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, .08);
  font-weight: 600;
}
.f-upload {
  margin: 12px 14px 0;
  padding: 16px 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  border: 1px dashed rgba(255, 255, 255, .22);
  border-radius: 8px;
  cursor: pointer;
  transition: all .2s;
  text-align: center;
}
.f-upload:hover { border-color: #4f9cff; background: rgba(79, 156, 255, .08); }
.f-upload-icon { font-size: 22px; color: #4f9cff; }
.f-prog { color: #fbbf24; font-size: 12px; }
.f-list {
  flex: 1;
  overflow-y: auto;
  margin: 10px 14px;
  min-height: 80px;
}
.f-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 4px;
  border-bottom: 1px solid rgba(255, 255, 255, .05);
}
.f-item .is-dir { color: #e6a23c; }
.f-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.f-size { color: #9fb3c8; font-size: 12px; flex-shrink: 0; }
.f-empty { text-align: center; color: #6b7c8f; padding: 18px 0; margin: 0; }
.f-tip {
  margin: 0 14px 12px;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(79, 156, 255, .08);
  color: #9fb3c8;
  font-size: 12px;
  line-height: 1.6;
}
</style>
