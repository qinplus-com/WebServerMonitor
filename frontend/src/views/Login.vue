﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿﻿<template>
  <div class="login-container">
    <!-- 背景视频：上下各留160px，静音自动播放；源视频已裁掉四角「豆包AI生成」水印，object-fit:cover 自然居中 -->
    <video class="bg-video" autoplay muted playsinline preload="auto" src="/bgm.mp4"></video>
    <!-- 暗色遮罩：与视频同范围，压暗视频、提升表单对比度 -->
    <div class="bg-overlay"></div>

    <div class="login-box">
      <!-- 演示版角标 -->
      <div v-if="isDemo" class="demo-badge">演示版</div>
      <div class="login-header">
        <img src="/logo.png" alt="Logo" class="login-logo" />
        <h1 ref="titleEl">{{ appStore.appName }}</h1>
        <p class="subtitle" ref="subtitleEl">零客户端 · 类VPN穿透 · 全功能Linux运维网关</p>
      </div>
      <el-form :model="form" :rules="rules" ref="formRef" class="login-form" @keyup.enter="handleLogin">
        <el-form-item prop="username">
          <el-input v-model="form.username" name="username" autocomplete="username" placeholder="用户名" :prefix-icon="User" size="large" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" name="password" autocomplete="current-password" placeholder="密码" :prefix-icon="Lock" size="large" show-password />
        </el-form-item>
        <el-button type="primary" size="large" class="login-btn" :loading="loading" @click="handleLogin">登 录</el-button>
      </el-form>
      <!-- 演示账号提示 -->
      <div v-if="isDemo" class="login-tip demo-tip">演示账号：demo / demo123（不可修改）</div>
      <!-- 版权声明：由后端品牌接口下发（加密存储+完整性校验），前端不内置明文 -->
      <div v-if="appStore.copyright" ref="copyrightEl" class="login-tip copyright">{{ appStore.copyright }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { useUserStore } from '../store/user'
import { useAppStore } from '../store/app'

const appStore = useAppStore()

// ===== 标题/副标题单行自适应：任何自定义名称、任何窄屏都不换行 =====
const titleEl = ref(null)
const subtitleEl = ref(null)
const copyrightEl = ref(null)

// 从样式表基准字号开始，按 0.5px 递减直到文本宽度不超过可用宽度
function fitOneLine(el, minPx) {
  if (!el) return
  el.style.fontSize = '' // 先还原为样式表（calc(var(--fs))）基准字号
  const maxW = el.parentElement.clientWidth
  let size = parseFloat(getComputedStyle(el).fontSize)
  while (el.scrollWidth > maxW && size > minPx) {
    size -= 0.5
    el.style.fontSize = size + 'px'
  }
}

let fitRaf = null
function fitHeader() {
  cancelAnimationFrame(fitRaf)
  fitRaf = requestAnimationFrame(() => {
    fitOneLine(titleEl.value, 10)
    fitOneLine(subtitleEl.value, 9)
    fitOneLine(copyrightEl.value, 8)
  })
}

function onResize() {
  fitHeader()
}

onMounted(() => {
  window.addEventListener('resize', onResize)
  // 标题自适应：立即 + 延迟（等字体加载）+ 字体就绪后
  fitHeader()
  setTimeout(fitHeader, 300)
  document.fonts?.ready.then(fitHeader)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  cancelAnimationFrame(fitRaf)
})

const router = useRouter()
const userStore = useUserStore()
const formRef = ref()
const loading = ref(false)

const form = reactive({ username: '', password: '' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

// 演示版判定：品牌接口下发的平台名称含「演示版」
// appName 为异步下发，需 watch 以便在拿到名称后再预填账号
const isDemo = computed(() => (appStore.appName || '').includes('演示版'))
watch(isDemo, v => {
  if (v) { form.username = 'demo'; form.password = 'demo123' }
}, { immediate: true })
// 平台名称异步下发：到达后重新做单行自适应
watch(() => appStore.appName, () => fitHeader())

async function handleLogin() {
  await formRef.value.validate()
  loading.value = true
  try {
    await userStore.login(form.username, form.password)
    ElMessage.success('登录成功')
    router.push('/')
  } catch (e) {
    // 错误已在拦截器处理
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  height: 100vh; position: relative; overflow: hidden;
  display: flex; align-items: center; justify-content: center;
  background: #0a0a0a;
}

/* ===== 视频层：上下高度各减小160px =====
   video是替换元素，不能用 height:auto + top/bottom（会按原始比例算高、忽略bottom），
   必须显式指定高度，object-fit:cover 负责裁切与居中（默认 50% 50%） */
.bg-video {
  position: absolute; top: 160px; left: 0;
  width: 100vw; height: calc(100vh - 320px);
  object-fit: cover; z-index: 0;
}
.bg-overlay {
  position: absolute; top: 160px; bottom: 160px; left: 0; right: 0; z-index: 1;
  background:
    radial-gradient(ellipse at 50% 42%, rgba(10,10,10,0.32) 0%, rgba(8,8,8,0.68) 100%),
    linear-gradient(180deg, rgba(0,0,0,0.45) 0%, rgba(0,0,0,0.3) 40%, rgba(0,0,0,0.6) 100%);
}

/* ===== 玻璃拟态登录卡片 =====
   宽度随 --fs 字号系数缩放但封顶 640px：兼顾 4K 下标题不换行，
   又避免输入框/登录按钮被拉得过宽显得松散（长标题由 fitHeader 单行自适应兜底） */
.login-box {
  position: relative; z-index: 2;
  width: min(calc(560px * var(--fs, 1)), 640px);
  padding: calc(46px * var(--fs, 1)) calc(50px * var(--fs, 1)) calc(40px * var(--fs, 1));
  background: rgba(18, 20, 26, 0.55);
  border: 1px solid rgba(47, 129, 247, 0.25);
  border-radius: 16px;
  backdrop-filter: blur(22px) saturate(150%);
  -webkit-backdrop-filter: blur(22px) saturate(150%);
  box-shadow:
    0 24px 70px rgba(0, 0, 0, 0.55),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
  animation: box-in .6s cubic-bezier(.22,.8,.32,1) both;
}
/* 顶部科技感光带 */
.login-box::before {
  content: ''; position: absolute; top: 0; left: 22%; right: 22%; height: 1px;
  background: linear-gradient(90deg, transparent, rgba(47,129,247,.85), transparent);
}

/* ===== 流动边框：一条高亮光带沿边框循环流动 =====
   conic渐变提供光带，@property驱动角度，mask镂空只留2px边框 */
@property --border-angle {
  syntax: '<angle>'; initial-value: 0deg; inherits: false;
}
.login-box::after {
  content: ''; position: absolute; inset: 0; border-radius: 16px;
  padding: 2px; pointer-events: none;
  background: conic-gradient(
    from var(--border-angle),
    rgba(47,129,247,0.25) 0deg,
    rgba(47,129,247,0.25) 200deg,
    var(--accent) 285deg,
    #bfe3ff 320deg,
    rgba(47,129,247,0.25) 360deg
  );
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
          mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
          mask-composite: exclude;
  animation: border-flow 3.2s linear infinite;
}
@keyframes border-flow { to { --border-angle: 360deg; } }

@keyframes box-in {
  from { opacity: 0; transform: translateY(18px) scale(.985); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

.login-header { text-align: center; margin-bottom: 32px; }
.login-logo {
  width: 78px; height: 78px; border-radius: 18px; object-fit: contain;
  filter: drop-shadow(0 4px 14px rgba(47,129,247,0.35));
}
.login-header h1 {
  font-size: calc(22px * var(--fs, 1)); margin: 16px 0 8px; color: #f2f5fa;
  letter-spacing: 1px; text-shadow: 0 2px 12px rgba(0,0,0,0.5);
  line-height: 1.35;
  /* 强制单行：长自定义名称由 JS fitHeader 自动缩字号，禁止换行 */
  white-space: nowrap;
}
.subtitle {
  color: var(--text-2); font-size: calc(13px * var(--fs, 1)); letter-spacing: .5px;
  line-height: 1.8; white-space: nowrap;
}

.login-form { margin-top: 22px; color-scheme: dark; }

/* 输入框：不透明深色底，前缀区/输入区/自动填充三色完全统一
   （prefix 是 wrapper 的 flex 兄弟，半透明底会让背景视频亮块从图标区透出来）
   注意：App.vue 全局有 .el-input__wrapper{background:var(--bg-input)!important}，
   这里必须用更高特异性（.login-form[data-v] …）的 !important 才能胜出 */
.login-form :deep(.el-input__wrapper) {
  background: #161b25 !important;
  box-shadow: 0 0 0 1px rgba(255,255,255,0.14) inset !important;
  border-radius: 10px; height: 52px; padding: 0 16px;
  transition: box-shadow .2s, background .2s;
}
.login-form :deep(.el-input__wrapper:hover) {
  background: #1a202d !important;
  box-shadow: 0 0 0 1px rgba(255,255,255,0.22) inset !important;
}
.login-form :deep(.el-input__wrapper.is-focus) {
  background: #161b25 !important;
  box-shadow: 0 0 0 1px var(--accent) inset, 0 0 0 3px rgba(47,129,247,0.18) !important;
}
/* 前缀容器透明，显露出 wrapper 自身的不透明深色 */
.login-form :deep(.el-input__prefix) { background: transparent; }
/* inner 透明，让 wrapper 的 #161b25 贯通整个输入框（全局 !important 把 inner 涂成 #0d0d0d，需反覆盖） */
.login-form :deep(.el-input__inner) { color: #f2f5fa !important; height: 52px; background: transparent !important; font-size: calc(15px * var(--fs, 1)); }
.login-form :deep(.el-input__inner::placeholder) { color: var(--text-3); }
.login-form :deep(.el-input__prefix-inner) { margin-right: 12px; }
.login-form :deep(.el-input__prefix-inner .el-icon) { color: var(--text-2); font-size: calc(19px * var(--fs, 1)); }
.login-form :deep(.el-input__wrapper.is-focus .el-input__prefix-inner .el-icon) { color: var(--accent); }

/* 强制覆盖浏览器自动填充的灰色高亮（保持深色玻璃风）
   关键级联事实（CSS Cascade L4，优先级低→高）：
     UA 普通(1) < 作者普通(3) < 作者 !important(5) < UA !important(7)
   Chromium/Edge 的 UA 样式表（html.css）对填充框设：
     input:-internal-autofill-previewed,
     input:-internal-autofill-selected {
       appearance: menulist-button;
       background-color: light-dark(#E8F0FE, rgba(70,90,126,.4)) !important;
     }
   => background-color 是 UA !important，作者任何规则都无法直接改填充底色；
   => 但 UA 未锁定 box-shadow：大扩散半径 inset 阴影按标准绘制顺序
      画在元素背景层之上，是唯一可靠的覆盖手段；
   => 显式 appearance:none 双保险，杜绝一切 native theme 绘制路径；
   => .login-form 上 color-scheme:dark 让 UA 走 dark 分支（light 分支浅蓝更突兀）。
   触发时机：会话过期自动跳回 /login 等非整页场景，浏览器静默填充时。 */
.login-form :deep(.el-input__inner:-webkit-autofill),
.login-form :deep(.el-input__inner:-webkit-autofill:hover),
.login-form :deep(.el-input__inner:-webkit-autofill:focus),
.login-form :deep(.el-input__inner:-webkit-autofill:active),
.login-form :deep(.el-input__inner:autofill),
.login-form :deep(.el-input__inner:-internal-autofill-previewed),
.login-form :deep(.el-input__inner:-internal-autofill-selected) {
  -webkit-appearance: none !important;
  appearance: none !important;
  -webkit-text-fill-color: #f2f5fa !important;
  -webkit-box-shadow: 0 0 0 1000px #161b25 inset !important;
  box-shadow: 0 0 0 1000px #161b25 inset !important;
  caret-color: #f2f5fa !important;
  transition: background-color 9999s ease-in-out 0s !important;
}

.login-btn {
  width: 100%; margin-top: 14px; height: 46px;
  font-size: calc(15px * var(--fs, 1)); letter-spacing: 4px; border-radius: 10px;
  background: linear-gradient(135deg, var(--accent) 0%, #1f6fe0 100%);
  border: none;
  box-shadow: 0 6px 20px rgba(47,129,247,0.4), inset 0 1px 0 rgba(255,255,255,0.18);
  transition: transform .15s, box-shadow .2s, filter .2s;
}
.login-btn:hover {
  transform: translateY(-1px); filter: brightness(1.08);
  box-shadow: 0 10px 26px rgba(47,129,247,0.5), inset 0 1px 0 rgba(255,255,255,0.2);
}
.login-btn:active { transform: translateY(0); }

.login-tip {
  text-align: center; color: var(--text-3);
  font-size: calc(12px * var(--fs, 1)); margin-top: 18px; letter-spacing: .3px;
  line-height: 1.8;
}
.copyright { letter-spacing: .3px; color: var(--text-2); opacity: .8; white-space: nowrap; }

/* ===== 演示版角标与提示 ===== */
.demo-badge {
  position: absolute; top: 14px; right: 14px;
  padding: 3px 10px; border-radius: 999px;
  font-size: calc(12px * var(--fs, 1)); letter-spacing: 1px;
  color: #bfe3ff; background: rgba(47,129,247,.18);
  border: 1px solid rgba(47,129,247,.45);
  pointer-events: none;
}
.demo-tip { color: var(--accent); opacity: .9; }

@media (max-width: 640px) {
  .login-box { width: calc(100vw - 48px); padding: 36px 28px 30px; }
  .login-header { margin-bottom: 26px; }
  .login-header h1 { font-size: calc(17px * var(--fs, 1)); letter-spacing: .3px; }
  .subtitle { font-size: calc(12px * var(--fs, 1)); letter-spacing: .2px; }
  .copyright { font-size: calc(11.5px * var(--fs, 1)); }
}
</style>
