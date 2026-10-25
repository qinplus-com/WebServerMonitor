<template>
  <div class="about-page">
    <!-- 系统介绍 -->
    <el-card shadow="never" class="about-hero">
      <img src="/logo.png" alt="Logo" class="hero-logo" />
      <div class="hero-info">
        <h1>{{ appStore.appName }}</h1>
        <p>零客户端 · 浏览器即开即用 · 全功能 Linux 服务器运维网关</p>
      </div>
    </el-card>

    <!-- 功能特性 -->
    <el-card shadow="never" class="about-card">
      <template #header><span class="card-h">系统功能</span></template>
      <div class="feature-grid">
        <div v-for="(f, idx) in features" :key="f.name" class="feature-card" :class="{ flowing: flowIdx === idx }">
          <div class="fc-cover" :class="{ 'icon-cover': !f.image }">
            <img v-if="f.image" :src="f.image" :alt="f.name" loading="lazy" />
            <el-icon v-else :size="46"><component :is="f.icon" /></el-icon>
          </div>
          <div class="fc-body">
            <div class="feature-name">{{ f.name }}</div>
            <div class="feature-desc">{{ f.desc }}</div>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 关于公司 -->
    <el-card shadow="never" class="about-card">
      <template #header><span class="card-h card-h-long">{{ companyIntro }}</span></template>
      <div class="contact-grid">
        <div class="contact-item">
          <div class="ci-icon"><el-icon><OfficeBuilding /></el-icon></div>
          <div class="ci-text"><span>公司名称</span><b>{{ appStore.company }}</b></div>
        </div>
        <div class="contact-item">
          <div class="ci-icon"><el-icon><Link /></el-icon></div>
          <div class="ci-text">
            <span>官方网站</span>
            <b><a :href="appStore.website" target="_blank" rel="noopener" class="link">{{ siteHost }}</a></b>
          </div>
        </div>
        <div class="contact-item">
          <div class="ci-icon"><el-icon><Phone /></el-icon></div>
          <div class="ci-text">
            <span>联系电话</span>
            <b><a href="tel:18047115759" class="link">18047115759</a></b>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 版权声明：由后端品牌接口下发（加密存储+完整性校验），页面仅只读展示 -->
    <p v-if="appStore.copyright" class="copyright-text">{{ appStore.copyright }}</p>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Monitor, Platform, Folder, Connection, Document, DataLine, Cpu, Brush, OfficeBuilding, Link, Phone } from '@element-plus/icons-vue'
import { useAppStore } from '../store/app'

const appStore = useAppStore()

// 公司介绍（公司名来自品牌接口）
const companyIntro = computed(() =>
  appStore.company + '，致力于为企业提供全方位的数字化转型服务，帮助企业提升运营效率，实现可持续发展。'
)

// 网址显示：从完整 URL 提取主机名（如 qinplus.com）
const siteHost = computed(() => {
  try {
    return new URL(appStore.website).host
  } catch (e) {
    return appStore.website
  }
})

// 流光边框：每 2.5s 点亮下一张卡片（彗星沿边框跑一圈），循环逐个显示
const flowIdx = ref(-1)
let flowTimer = null
onMounted(() => {
  let i = 0
  flowTimer = setInterval(() => { flowIdx.value = i % 8; i++ }, 2500)
})
onBeforeUnmount(() => clearInterval(flowTimer))

const features = [
  { name: 'Web SSH 终端', desc: '浏览器直连 Linux 服务器，支持密码/密钥认证，选中复制、右键粘贴', icon: Monitor,
    image: '/features/01-ssh.svg' },
  { name: '多标签会话', desc: '同时连接多台服务器，同一服务器支持多连接并行操作', icon: Cpu,
    image: '/features/02-tabs.svg' },
  { name: 'SFTP 文件管理', desc: '在线浏览、上传、下载、编辑服务器文件', icon: Folder,
    image: '/features/03-sftp.svg' },
  { name: '网址可访问性监控', desc: '实时监控网站状态、延时、状态码及 SSL 证书有效期', icon: DataLine,
    image: '/features/04-monitor.svg' },
  { name: 'AI 智能助手', desc: '免费 AI 提供操作系统级技术支持，可结合终端输出分析问题', icon: Brush,
    image: '/features/05-ai.svg' },
  { name: '权限与审计', desc: '多角色分级授权，服务器访问与操作全程记录审计日志', icon: Document,
    image: '/features/06-audit.svg' },
  { name: '在线会话管理', desc: '实时查看与管理全部活跃终端连接', icon: Connection,
    image: '/features/07-session.svg' },
  { name: '服务器集中管理', desc: '服务器信息、分组、网址、授权统一维护', icon: Platform,
    image: '/features/08-dc.svg' }
]
</script>

<style scoped>
/* 全宽自适应 */
.about-page { width: 100%; }

.about-hero :deep(.el-card__body) {
  display: flex; align-items: center; gap: 16px; padding: 14px 16px;
}
.hero-logo { width: 60px; height: 60px; border-radius: 12px; }
.hero-info h1 { margin: 0 0 6px; font-size: calc(20px * var(--fs, 1)); color: var(--text-1); }
.hero-info p { margin: 0; font-size: calc(12.5px * var(--fs, 1)); color: var(--text-2); }

.about-card { margin-top: 12px; }
.about-card :deep(.el-card__header) { padding: 11px 16px; }
.about-card :deep(.el-card__body) { padding: 13px 16px 15px; }
.card-h { font-size: calc(15px * var(--fs, 1)); font-weight: 600; color: var(--text-1); }
.card-h-long { font-size: calc(13px * var(--fs, 1)); font-weight: 500; line-height: 1.6; color: var(--text-1); display: block; }

/* 功能卡片：每行最多4列，按页面宽度逐级减少（4→3→2→1） */
.feature-grid {
  display: grid; grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
.feature-card {
  position: relative;
  background: var(--bg-panel-2); border: 1px solid var(--border-soft); border-radius: 12px;
  overflow: hidden; display: flex; flex-direction: column;
  transition: transform .18s, border-color .18s, box-shadow .18s;
}
.feature-card:hover {
  transform: translateY(-3px); border-color: rgba(47,129,247,.45);
  box-shadow: 0 12px 28px rgba(0,0,0,.4);
}

/* ===== 逐个点亮的流光边框（伪元素+mask镂空，仅2px描边） ===== */
@property --flow-angle { syntax: '<angle>'; initial-value: 0deg; inherits: false; }
.feature-card::after {
  content: ''; position: absolute; inset: 0; border-radius: 12px; padding: 2px;
  pointer-events: none; opacity: 0;
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
          mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
          mask-composite: exclude;
}
.feature-card.flowing::after {
  opacity: 1;
  background: conic-gradient(from var(--flow-angle),
    rgba(47,129,247,0) 0deg, rgba(47,129,247,0) 205deg,
    rgba(47,129,247,.3) 268deg, var(--accent) 312deg, #c3e0ff 336deg,
    rgba(47,129,247,0) 360deg);
  animation: comet-run 2.4s linear 1;
}
@keyframes comet-run { to { --flow-angle: 360deg; } }
.fc-cover {
  aspect-ratio: 16 / 10; overflow: hidden; background: #1c1c1c; position: relative;
}
.fc-cover img {
  width: 100%; height: 100%; object-fit: cover; display: block;
  transition: transform .35s ease;
}
.feature-card:hover .fc-cover img { transform: scale(1.05); }
/* 暂无配图时：深色科技渐变 + 大图标封面 */
.fc-cover.icon-cover {
  display: flex; align-items: center; justify-content: center;
  color: rgba(121,192,255,.85);
  background:
    radial-gradient(circle at 28% 24%, rgba(47,129,247,.28), transparent 58%),
    linear-gradient(135deg, #16315c, #102344 55%, #0b1a33);
}
.fc-cover.icon-cover::before {
  content: ''; position: absolute; inset: 0;
  background-image:
    linear-gradient(rgba(121,192,255,.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(121,192,255,.07) 1px, transparent 1px);
  background-size: 26px 26px;
}
.fc-cover.icon-cover .el-icon { position: relative; filter: drop-shadow(0 0 10px rgba(47,129,247,.6)); }
.fc-body { padding: 11px 14px 13px; }
.feature-name { font-size: calc(14px * var(--fs, 1)); font-weight: 600; color: var(--text-1); margin-bottom: 4px; }
.feature-desc { font-size: calc(12px * var(--fs, 1)); color: var(--text-3); line-height: 1.55; }

.link { color: var(--accent); text-decoration: none; }
.link:hover { color: #79c0ff; text-decoration: underline; }

/* 联系信息卡片（替代表格） */
.contact-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: 12px; margin-bottom: 0;
}
.contact-item {
  display: flex; align-items: center; gap: 12px;
  background: var(--bg-panel-2); border: 1px solid var(--border-soft); border-radius: 12px;
  padding: 11px 15px;
  transition: border-color .15s;
}
.contact-item:hover { border-color: rgba(47,129,247,.35); }
.ci-icon {
  width: 38px; height: 38px; border-radius: 10px; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  color: var(--accent); background: rgba(47,129,247,.12);
}
.ci-text { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.ci-text span { font-size: calc(11px * var(--fs, 1)); color: var(--text-3); }
.ci-text b { font-size: calc(14px * var(--fs, 1)); font-weight: 600; color: var(--text-1); }

.copyright-text {
  text-align: center; font-size: calc(12px * var(--fs, 1)); color: var(--text-3); margin: 14px 0 2px;
}

@media (max-width: 1090px) {
  .feature-grid { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 820px) {
  .feature-grid { grid-template-columns: repeat(2, 1fr); }
}
@media (max-width: 560px) {
  .feature-grid { grid-template-columns: 1fr; }
  .about-hero :deep(.el-card__body) { flex-direction: column; text-align: center; }
}
</style>
