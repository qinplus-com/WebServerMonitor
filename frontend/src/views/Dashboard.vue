<template>
  <div class="dashboard">
    <el-row :gutter="20">
      <el-col :xs="12" :sm="12" :md="6"><el-card shadow="hover" class="stat-card"><div class="stat-icon blue"><el-icon :size="32"><User /></el-icon></div><div class="stat-info"><div class="num">{{ data.user_count || 0 }}</div><div class="label">用户总数</div></div></el-card></el-col>
      <el-col :xs="12" :sm="12" :md="6"><el-card shadow="hover" class="stat-card"><div class="stat-icon green"><el-icon :size="32"><Platform /></el-icon></div><div class="stat-info"><div class="num">{{ data.server_count || 0 }}</div><div class="label">服务器总数</div></div></el-card></el-col>
      <el-col :xs="12" :sm="12" :md="6"><el-card shadow="hover" class="stat-card"><div class="stat-icon orange"><el-icon :size="32"><Connection /></el-icon></div><div class="stat-info"><div class="num">{{ data.online_session || 0 }}</div><div class="label">在线会话</div></div></el-card></el-col>
      <el-col :xs="12" :sm="12" :md="6"><el-card shadow="hover" class="stat-card"><div class="stat-icon purple"><el-icon :size="32"><Key /></el-icon></div><div class="stat-info"><div class="num">{{ data.today_login || 0 }}</div><div class="label">今日登录</div></div></el-card></el-col>
    </el-row>

    <!-- 快捷操作卡片 -->
    <el-card class="quick-card" shadow="never">
      <h3><el-icon><Grid /></el-icon> 快捷操作</h3>
      <div class="quick-grid">
        <div class="quick-item" @click="$router.push('/servers')">
          <div class="quick-icon green"><el-icon :size="24"><Platform /></el-icon></div>
          <span>服务器管理</span>
        </div>
        <div class="quick-item" @click="$router.push('/terminal')">
          <div class="quick-icon blue"><el-icon :size="24"><Monitor /></el-icon></div>
          <span>Web终端</span>
        </div>
        <div class="quick-item" @click="$router.push('/sftp')">
          <div class="quick-icon orange"><el-icon :size="24"><Folder /></el-icon></div>
          <span>文件管理</span>
        </div>
        <div v-if="isAdmin" class="quick-item" @click="$router.push('/users')">
          <div class="quick-icon purple"><el-icon :size="24"><User /></el-icon></div>
          <span>用户管理</span>
        </div>
        <div v-if="isAdmin" class="quick-item" @click="$router.push('/sessions')">
          <div class="quick-icon cyan"><el-icon :size="24"><Connection /></el-icon></div>
          <span>在线会话</span>
        </div>
        <div class="quick-item" @click="$router.push('/logs')">
          <div class="quick-icon red"><el-icon :size="24"><Document /></el-icon></div>
          <span>审计日志</span>
        </div>
      </div>
    </el-card>

    <!-- 网址监控面板：健康汇总 + 状态卡片网格 -->
    <el-card class="monitor-card" shadow="never">
      <div class="monitor-header">
        <h3><el-icon><DataLine /></el-icon> 网址可访问性监控</h3>
        <div class="monitor-actions">
          <el-button size="small" type="primary" :loading="checking" @click="checkNow">
            <el-icon><Refresh /></el-icon> 立即检测
          </el-button>
        </div>
      </div>

      <!-- 健康汇总条（紧凑，宽度与网格对齐） -->
      <div class="health-bar">
        <el-progress type="circle" :percentage="availability" :width="56" :stroke-width="6" :color="ringColor">
          <span class="hb-num">{{ availability }}<i>%</i></span>
        </el-progress>
        <div class="hb-divider"></div>
        <div class="hb-stats">
          <div class="hb-stat ok"><b>{{ summary.ok }}</b><span>正常</span></div>
          <div class="hb-stat warn"><b>{{ summary.warn }}</b><span>异常</span></div>
          <div class="hb-stat err"><b>{{ summary.err }}</b><span>不可访问</span></div>
          <div class="hb-stat idle"><b>{{ summary.idle }}</b><span>待检测</span></div>
        </div>
        <div class="hb-meta">
          <div class="hb-meta-item"><span>平均延时</span><b :class="{ 'lat-warn': avgLatency > 1000 }">{{ avgLatency }}<i>ms</i></b></div>
          <div class="hb-meta-item"><span>监控网址</span><b>{{ monitorList.length }}<i>个</i></b></div>
        </div>
      </div>

      <!-- 状态卡片网格（异常自动置顶；卡片有最大宽度，超宽屏不再被压窄） -->
      <div v-loading="monitorLoading" class="url-grid">
        <div v-for="row in sortedMonitorList" :key="row.id" class="url-card"
          :class="[cardLevel(row), flowType(row), { flowing: row.id === activeFlowKey }]">
          <div class="uc-head">
            <span class="uc-dot" :class="dotClass(row)"></span>
            <span class="uc-server" @click="gotoTerminal(row)" :title="row.server_ip ? `${row.server_name}（${row.server_ip}）` : row.server_name">{{ row.server_name }}</span>
            <span class="uc-proto" :class="row.protocol">{{ row.protocol }}</span>
          </div>

          <a class="uc-url" :href="urlHref(row)" target="_blank" rel="noopener">
            {{ domainText(row) }}
          </a>

          <div class="uc-metrics">
            <span v-if="row.status && row.status.accessible" class="uc-lat" :class="{ slow: row.status.latency_ms > 1000 }">
              {{ row.status.latency_ms }}<i>ms</i>
            </span>
            <span v-else class="uc-lat dim">—</span>
            <span v-if="row.status && row.status.status_code" class="uc-code"
              :class="row.status.status_code === 200 ? 'good' : 'bad'">{{ row.status.status_code }}</span>
          </div>

          <div v-if="row.protocol === 'https'" class="uc-cert">
            <span v-if="row.status && row.status.cert_expire_at" class="uc-cert-text"
              :class="certClass(row)" :title="certText(row)">{{ certText(row) }}</span>
            <span v-else class="uc-cert-text dim">证书未检测</span>
            <el-progress
              v-if="row.status && row.status.cert_expire_at"
              :percentage="certPercent(row)" :stroke-width="4" :show-text="false"
              :color="certBarColor(row)"
            />
          </div>

          <div v-if="row.status && row.status.error" class="uc-error" :title="row.status.error">
            <el-icon><WarningFilled /></el-icon><span>{{ row.status.error }}</span>
          </div>
        </div>

        <div v-if="!monitorLoading && monitorList.length === 0" class="grid-empty">
          <el-icon :size="40"><Connection /></el-icon>
          <p>暂无监控网址</p>
          <p class="grid-empty-sub">请在「服务器管理」编辑服务器时添加网站地址</p>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { dashboardApi, monitorApi } from '../api'
import { useUserStore } from '../store/user'

const router = useRouter()
const userStore = useUserStore()
const isAdmin = computed(() => userStore.userInfo.role === 'admin')
const data = ref({})
const monitorList = ref([])
const monitorLoading = ref(false)
const checking = ref(false)
let refreshTimer = null

const summary = computed(() => {
  const s = { ok: 0, warn: 0, err: 0, idle: 0 }
  for (const row of monitorList.value) {
    const cls = dotClass(row)
    if (cls === 'ok') s.ok++
    else if (cls === 'warn') s.warn++
    else if (cls === 'err') s.err++
    else s.idle++
  }
  return s
})

// 卡片健康等级：err 不可访问 / warn 异常或证书问题 / ok 正常 / idle 待检测
function cardLevel(row) {
  const cls = dotClass(row)
  if (cls === 'err') return 'lvl-err'
  if (cls === 'warn') return 'lvl-warn'
  if (cls === 'ok') {
    // 证书异常（含过期）/临期也降级为警告
    if (row.protocol === 'https' && row.status && row.status.cert_expire_at &&
      (row.status.cert_valid === false || row.status.cert_days_left <= 15)) return 'lvl-warn'
    return 'lvl-ok'
  }
  return 'lvl-idle'
}

// 排序：不可访问 → 警告 → 正常 → 待检测；同级慢的在前
const sortedMonitorList = computed(() => {
  const rank = { 'lvl-err': 0, 'lvl-warn': 1, 'lvl-ok': 2, 'lvl-idle': 3 }
  return [...monitorList.value].sort((a, b) => {
    const ra = rank[cardLevel(a)], rb = rank[cardLevel(b)]
    if (ra !== rb) return ra - rb
    const la = a.status?.accessible ? a.status.latency_ms : -1
    const lb = b.status?.accessible ? b.status.latency_ms : -1
    return lb - la
  })
})

// 可用率（仅统计已检测网址）
const availability = computed(() => {
  const checked = summary.value.ok + summary.value.warn + summary.value.err
  if (!checked) return 0
  return Math.round((summary.value.ok / checked) * 100)
})
const ringColor = computed(() => {
  const v = availability.value
  if (v >= 95) return '#3fb950'
  if (v >= 80) return '#e0af68'
  return '#f85149'
})

// 平均延时（仅可访问网址）
const avgLatency = computed(() => {
  let sum = 0, n = 0
  for (const row of monitorList.value) {
    if (row.status && row.status.accessible) { sum += row.status.latency_ms; n++ }
  }
  return n ? Math.round(sum / n) : 0
})

// 网址显示与跳转
function urlHref(row) {
  if (row.status && row.status.url) return row.status.url
  const port = row.port && row.port !== (row.protocol === 'http' ? 80 : 443) ? `:${row.port}` : ''
  return `${row.protocol}://${row.domain}${port}`
}
function domainText(row) {
  const port = row.port && row.port !== (row.protocol === 'http' ? 80 : 443) ? `:${row.port}` : ''
  return `${row.domain}${port}`
}

// 证书倒计时百分比（以90天为满格基准，过期/异常为0）
function certPercent(row) {
  if (!row.status || !row.status.cert_expire_at || row.status.cert_valid === false) return 0
  return Math.max(0, Math.min(100, Math.round((row.status.cert_days_left / 90) * 100)))
}
function certBarColor(row) {
  if (row.status.cert_valid === false) return '#f85149'
  const d = row.status.cert_days_left
  if (d < 0) return '#f85149'
  if (d <= 15) return '#e0af68'
  return '#2f81f7'
}

function dotClass(row) {
  if (!row.status) return 'idle'
  if (!row.status.accessible) return 'err'
  if (row.status.status_code >= 400) return 'warn'
  if (row.protocol === 'https' && row.status.cert_expire_at && row.status.cert_valid === false) return 'warn'
  return 'ok'
}
function statusText(row) {
  if (!row.status) return '待检测'
  if (!row.status.accessible) return '不可访问'
  if (row.status.status_code >= 500) return '服务错误'
  if (row.status.status_code >= 400) return '访问异常'
  if (row.protocol === 'https' && row.status.cert_expire_at && row.status.cert_valid === false) return '证书异常'
  return '正常'
}
function certText(row) {
  const s = row.status
  if (s.cert_valid === false) return s.cert_error || '证书异常'
  const d = s.cert_days_left
  return d < 0 ? '已过期' : `剩余 ${d} 天`
}
function certClass(row) {
  const s = row.status
  if (s.cert_valid === false) return 'invalid'
  const d = s.cert_days_left
  if (d < 0) return 'expired'
  if (d <= 15) return 'expiring'
  return 'valid'
}
function gotoTerminal(row) {
  router.push({ path: '/terminal', query: { server_id: row.server_id } })
}
function fmtTime(t) {
  if (!t) return ''
  const d = new Date(t)
  const p = n => String(n).padStart(2, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

async function loadMonitor() {
  monitorLoading.value = true
  try {
    const res = await monitorApi.status()
    monitorList.value = res.data || []
  } finally { monitorLoading.value = false }
}

async function checkNow() {
  checking.value = true
  try {
    const res = await monitorApi.check()
    monitorList.value = res.data || []
    ElMessage.success('检测完成')
  } finally { checking.value = false }
}

/* ===== 边框流光：按状态/证书分类，逐张顺序播放（同一时刻只有一张发光） ===== */
const activeFlowKey = ref(null)
let flowTimer = null
const FLOW_WINDOW = 2500

// 颜色优先级：证书异常→红黄蓝闪烁；故障+证书≤30天→闪烁；4xx/5xx→红；200+证书≤30→黄；200+证书>30→绿
function flowType(row) {
  const s = row.status
  if (!s) return 'f-idle'
  const d = s.cert_days_left
  const isErr = !s.accessible || (s.status_code && s.status_code >= 400)
  const is200 = s.accessible && s.status_code >= 200 && s.status_code < 300
  const certBad = s.cert_expire_at && s.cert_valid === false
  const certShort = d !== null && d <= 30
  if (certBad) return 'f-flash'                 // 证书异常（域名不匹配/链不受信任/过期等）
  if (isErr && certShort) return 'f-flash'      // 服务故障 且 证书≤30天
  if (isErr) return 'f-red'
  if (is200) return certShort ? 'f-yellow' : 'f-green'
  return 'f-idle'
}

// 推进到下一张（每次读取最新列表，数据刷新不打断节奏）
function advanceFlow() {
  const list = sortedMonitorList.value.filter(r => r.status)
  if (!list.length) { activeFlowKey.value = null; return }
  const i = list.findIndex(r => r.id === activeFlowKey.value)
  activeFlowKey.value = list[(i + 1) % list.length].id
}
// 数据刷新后：当前发光卡片若已消失才归位，否则保持原节奏
watch(sortedMonitorList, list => {
  const eligible = list.filter(r => r.status)
  if (!eligible.some(r => r.id === activeFlowKey.value)) {
    activeFlowKey.value = eligible.length ? eligible[0].id : null
  }
})

onMounted(async () => {
  const res = await dashboardApi.info()
  data.value = res.data
  loadMonitor()
  // 登录后自动触发一轮网址可访问性检测
  checkNow().catch(() => {})
  refreshTimer = setInterval(loadMonitor, 60000)
  // 流光轮播只启动一次；数据就绪后首张立即发光
  setTimeout(() => {
    const first = sortedMonitorList.value.filter(r => r.status)[0]
    if (first) activeFlowKey.value = first.id
    flowTimer = setInterval(advanceFlow, FLOW_WINDOW)
  }, 1200)
})
onBeforeUnmount(() => { clearInterval(refreshTimer); clearInterval(flowTimer) })
</script>

<style scoped>
.stat-card { display: flex; }
.stat-card :deep(.el-card__body) { display: flex; align-items: center; gap: 16px; }
.stat-icon { width: 64px; height: 64px; border-radius: 12px; display: flex; align-items: center; justify-content: center; color: #fff; }
.stat-icon.blue { background: #409eff; }
.stat-icon.green { background: #67c23a; }
.stat-icon.orange { background: #e6a23c; }
.stat-icon.purple { background: #909399; }
.stat-info .num { font-size: calc(28px * var(--fs, 1)); font-weight: bold; color: var(--text-1); }
.stat-info .label { color: var(--text-3); font-size: calc(14px * var(--fs, 1)); margin-top: 4px; }

/* ===== 快捷操作卡片 ===== */
.quick-card { margin-top: 20px; }
.quick-card h3 { display: flex; align-items: center; gap: 6px; color: var(--text-1); margin: 0 0 16px; font-size: calc(16px * var(--fs, 1)); }
.quick-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 16px; }
.quick-item {
  display: flex; flex-direction: column; align-items: center; gap: 10px;
  padding: 16px 10px; border-radius: 10px; cursor: pointer;
  background: var(--bg-panel-2); border: 1px solid var(--border);
  transition: all .15s;
}
.quick-item:hover { background: rgba(47,129,247,0.08); border-color: var(--accent); transform: translateY(-2px); }
.quick-item span { font-size: calc(13px * var(--fs, 1)); color: var(--text-2); }
.quick-item:hover span { color: var(--text-1); }
.quick-icon { width: 48px; height: 48px; border-radius: 12px; display: flex; align-items: center; justify-content: center; color: #fff; }
.quick-icon.green { background: #67c23a; }
.quick-icon.blue { background: #409eff; }
.quick-icon.orange { background: #e6a23c; }
.quick-icon.purple { background: #909399; }
.quick-icon.cyan { background: #00b7c3; }
.quick-icon.red { background: #f56c6c; }

.monitor-card { margin-top: 20px; }
.monitor-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
.monitor-header h3 { display: flex; align-items: center; gap: 8px; font-size: calc(15px * var(--fs, 1)); color: var(--text-1); margin: 0; }
.monitor-actions { display: flex; align-items: center; gap: 12px; }

/* ===== 健康汇总条（紧凑） ===== */
.health-bar {
  display: flex; align-items: center; gap: 20px;
  background: var(--bg-rail); border: 1px solid var(--border-soft); border-radius: 12px;
  padding: 12px 20px; margin-bottom: 16px;
}
.hb-num { font-size: calc(15px * var(--fs, 1)); font-weight: 700; color: var(--text-1); line-height: 1; }
.hb-num i { font-size: calc(9px * var(--fs, 1)); font-style: normal; font-weight: 500; color: var(--text-3); margin-left: 1px; }
.hb-divider { width: 1px; height: 32px; background: var(--border); flex-shrink: 0; }
.hb-stats { display: flex; gap: 24px; flex-wrap: wrap; }
.hb-stat { display: flex; align-items: baseline; gap: 6px; line-height: 1; }
.hb-stat b { font-size: calc(18px * var(--fs, 1)); font-weight: 700; color: var(--text-1); }
.hb-stat span { font-size: calc(12px * var(--fs, 1)); color: var(--text-3); }
.hb-stat.ok b { color: #3fb950; }
.hb-stat.warn b { color: #e0af68; }
.hb-stat.err b { color: #f85149; }
.hb-stat.idle b { color: #888888; }
.hb-meta { margin-left: auto; display: flex; gap: 26px; flex-shrink: 0; }
.hb-meta-item { display: flex; align-items: baseline; gap: 8px; line-height: 1; }
.hb-meta-item span { font-size: calc(12px * var(--fs, 1)); color: var(--text-3); }
.hb-meta-item b { font-size: calc(15px * var(--fs, 1)); color: var(--text-1); font-weight: 700; }
.hb-meta-item b i { font-size: calc(10px * var(--fs, 1)); font-style: normal; font-weight: 400; color: var(--text-3); margin-left: 2px; }
.hb-meta-item b.lat-warn { color: #e0af68; }

/* ===== 状态卡片网格（卡片270~350px，超宽屏不再被压成窄条） ===== */
.url-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(270px, 100%), 350px));
  justify-content: flex-start;
  gap: 14px; min-height: 70px;
}
.url-card {
  position: relative; background: var(--bg-panel-2); border: 1px solid var(--border-soft); border-radius: 12px;
  padding: 14px 16px 13px; display: flex; flex-direction: column; gap: 10px;
  transition: transform .15s, border-color .15s, box-shadow .15s; overflow: hidden;
}
.url-card:hover { transform: translateY(-2px); border-color: var(--bg-hover); box-shadow: 0 8px 22px rgba(0,0,0,.35); }
.lvl-ok:hover { border-color: rgba(63,185,80,.4); }
.lvl-warn { border-color: rgba(224,175,104,.3); }
.lvl-err { border-color: rgba(248,81,73,.35); background: linear-gradient(180deg, rgba(248,81,73,.06), var(--bg-panel-2) 60%); }

/* ===== 流光边框层（伪元素+mask镂空，仅2px描边，不遮挡内容） ===== */
@property --flow-angle { syntax: '<angle>'; initial-value: 0deg; inherits: false; }
.url-card::after {
  content: ''; position: absolute; inset: -1px; border-radius: 13px; padding: 2px;
  pointer-events: none; opacity: 0; transition: opacity .2s;
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
          mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
          mask-composite: exclude;
}
.url-card.flowing::after { opacity: 1; }

/* 彗星：一条带尾迹的高亮光带，2.4s沿边框跑完一整圈（只播放一次） */
.url-card.f-green.flowing::after {
  background: conic-gradient(from var(--flow-angle),
    rgba(63,185,80,0) 0deg, rgba(63,185,80,0) 205deg,
    rgba(63,185,80,.3) 268deg, #3fb950 312deg, #c3ffd2 336deg,
    rgba(63,185,80,0) 360deg);
  animation: comet-run 2.4s linear 1;
}
.url-card.f-yellow.flowing::after {
  background: conic-gradient(from var(--flow-angle),
    rgba(224,175,104,0) 0deg, rgba(224,175,104,0) 205deg,
    rgba(224,175,104,.3) 268deg, #e0af68 312deg, #ffe9bd 336deg,
    rgba(224,175,104,0) 360deg);
  animation: comet-run 2.4s linear 1;
}
.url-card.f-red.flowing::after {
  background: conic-gradient(from var(--flow-angle),
    rgba(248,81,73,0) 0deg, rgba(248,81,73,0) 205deg,
    rgba(248,81,73,.3) 268deg, #f85149 312deg, #ffc8c4 336deg,
    rgba(248,81,73,0) 360deg);
  animation: comet-run 2.4s linear 1;
}
@keyframes comet-run { to { --flow-angle: 360deg; } }

/* 告警闪烁：整框在 红→黄→蓝 之间循环变色（2.4s窗口内约闪烁3次） */
.url-card.f-flash.flowing::after { animation: flash-ryb .8s linear infinite; }
@keyframes flash-ryb {
  0%, 32%   { background: conic-gradient(from var(--flow-angle), #f85149 0deg, #f85149 360deg); }
  33%, 65%  { background: conic-gradient(from var(--flow-angle), #e0af68 0deg, #e0af68 360deg); }
  66%, 100% { background: conic-gradient(from var(--flow-angle), var(--accent) 0deg, var(--accent) 360deg); }
}

.uc-head { display: flex; align-items: center; gap: 8px; min-width: 0; }
.uc-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.uc-dot.ok { background: #3fb950; box-shadow: 0 0 5px rgba(63,185,80,.5); }
.uc-dot.warn { background: #e0af68; }
.uc-dot.err { background: #f85149; }
.uc-dot.idle { background: var(--bg-hover); }
.uc-server {
  font-size: calc(13.5px * var(--fs, 1)); font-weight: 600; color: var(--text-1); cursor: pointer; flex: 1; min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.uc-server:hover { color: var(--accent); }
.uc-proto {
  font-size: calc(10px * var(--fs, 1)); font-weight: 700; padding: 2px 6px; border-radius: 4px;
  letter-spacing: .5px; flex-shrink: 0;
}
.uc-proto.https { color: #79c0ff; background: rgba(47,129,247,.13); }
.uc-proto.http { color: #e0af68; background: rgba(224,175,104,.1); }

.uc-url {
  font-size: calc(13px * var(--fs, 1)); color: var(--accent); text-decoration: none; min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.uc-url:hover { color: #9ab6ff; text-decoration: underline; }

/* 指标行：大延时数字 + 状态码胶囊，无文字标签 */
.uc-metrics { display: flex; align-items: center; justify-content: space-between; }
.uc-lat { font-size: calc(21px * var(--fs, 1)); font-weight: 700; color: var(--text-1); line-height: 1; }
.uc-lat i { font-size: calc(11px * var(--fs, 1)); font-style: normal; font-weight: 500; color: var(--text-3); margin-left: 2px; }
.uc-lat.slow { color: #e0af68; }
.uc-lat.dim { color: var(--text-3); font-weight: 400; font-size: calc(15px * var(--fs, 1)); }
.uc-code {
  font-size: calc(12px * var(--fs, 1)); font-weight: 700; padding: 3px 10px; border-radius: 20px; line-height: 1.4;
}
.uc-code.good { color: #3fb950; background: rgba(63,185,80,.12); }
.uc-code.bad { color: #f85149; background: rgba(248,81,73,.12); }

/* 证书：小字 + 4px细进度条（正常为蓝色系，收敛绿色） */
.uc-cert { display: flex; flex-direction: column; gap: 6px; }
.uc-cert-text {
  font-size: calc(11px * var(--fs, 1)); color: var(--text-2); line-height: 1;
  display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.uc-cert-text.expiring { color: #e0af68; }
.uc-cert-text.expired, .uc-cert-text.invalid { color: #f85149; }
.uc-cert-text.dim { color: var(--text-3); }
.uc-cert :deep(.el-progress-bar__outer) { background: var(--bg-input); }

.uc-error {
  display: flex; align-items: center; gap: 6px; font-size: calc(11px * var(--fs, 1)); color: #f85149;
  background: rgba(248,81,73,.08); border-radius: 6px; padding: 6px 9px;
}
.uc-error span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.uc-error .el-icon { flex-shrink: 0; }

.grid-empty {
  grid-column: 1 / -1; text-align: center; padding: 42px 0;
  color: var(--text-3); display: flex; flex-direction: column; align-items: center; gap: 10px;
}
.grid-empty p { margin: 0; font-size: calc(13px * var(--fs, 1)); }
.grid-empty-sub { color: var(--text-3) !important; font-size: calc(12px * var(--fs, 1)) !important; }

@media (max-width: 1090px) {
  .quick-grid { grid-template-columns: repeat(4, 1fr); }
}
@media (max-width: 820px) {
  .quick-grid { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 620px) {
  .health-bar { flex-wrap: wrap; gap: 16px; }
  .hb-meta { margin-left: 0; width: 100%; justify-content: space-between; }
  .monitor-header { flex-wrap: wrap; gap: 8px; }
  .url-grid { grid-template-columns: 1fr; }
}
@media (max-width: 560px) {
  .quick-grid { grid-template-columns: repeat(2, 1fr); }
}

.welcome-card { margin-top: 20px; }
.welcome-card h2 { margin-bottom: 12px; color: var(--text-1); }
.welcome-card p { color: var(--text-2); margin-bottom: 20px; line-height: 1.6; }
.quick-actions { margin-top: 24px; display: flex; gap: 12px; }
</style>
