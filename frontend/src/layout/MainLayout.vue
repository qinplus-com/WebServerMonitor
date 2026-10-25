<template>
  <el-container class="main-layout">
    <!-- 纯图标导航栏 -->
    <el-aside width="58px" class="icon-rail">
      <!-- 中部导航：大图标 -->
      <div class="rail-menus">
        <el-tooltip v-for="m in menus" :key="m.path" :content="m.title" placement="right" :show-after="200" :popper-options="{ modifiers: [{ name: 'computeStyles', options: { adaptive: false } }] }">
          <div class="rail-item" :class="{ active: $route.path === m.path }" @click="$router.push(m.path)">
            <el-icon :size="22"><component :is="m.icon" /></el-icon>
          </div>
        </el-tooltip>
      </div>

      <!-- 底部：关于入口 -->
      <el-tooltip content="关于本平台" placement="right" :show-after="200">
        <div class="rail-about" @click="$router.push('/about')">
          <el-icon :size="20"><InfoFilled /></el-icon>
        </div>
      </el-tooltip>
    </el-aside>

    <!-- 内容区 -->
    <el-main class="content">
      <!-- 顶部条：居中品牌 + 右侧用户 -->
      <div class="top-user-bar">
        <div class="brand">
          <img src="/logo.png" alt="Logo" class="brand-logo" />
          <span class="brand-title">{{ appStore.appName }}</span>
        </div>
        <el-tooltip :content="appStore.theme === 'dark' ? '切换到明亮模式' : '切换到暗黑模式'" placement="bottom" :show-after="200">
          <div class="theme-toggle" @click="appStore.toggleTheme()">
            <el-icon :size="19"><Sunny v-if="appStore.theme === 'dark'" /><Moon v-else /></el-icon>
          </div>
        </el-tooltip>
        <el-dropdown trigger="click" placement="bottom-end" @command="handleCommand">
          <div class="user-trigger">
            <el-icon :size="20"><UserFilled /></el-icon>
            <span class="user-name">{{ userInfo.username }}</span>
            <el-icon :size="12"><ArrowDown /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item disabled>{{ roleText }}</el-dropdown-item>
              <el-dropdown-item v-if="isAdmin" command="settings">设置</el-dropdown-item>
              <el-dropdown-item divided command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>

      <div class="page-host">
        <router-view v-slot="{ Component }">
          <keep-alive include="Terminal">
            <component :is="Component" />
          </keep-alive>
        </router-view>
      </div>

      <!-- 设置对话框 -->
      <SettingsDialog v-if="isAdmin" v-model="settingsVisible" />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  Platform, Cpu, UserFilled, ArrowDown, Odometer, Monitor, Folder, User, Connection, Document, InfoFilled,
  Sunny, Moon
} from '@element-plus/icons-vue'
import { useUserStore } from '../store/user'
import { useAppStore } from '../store/app'
import SettingsDialog from '../components/SettingsDialog.vue'

const router = useRouter()
const userStore = useUserStore()
const appStore = useAppStore()
const userInfo = computed(() => userStore.userInfo)
const isAdmin = computed(() => userStore.userInfo.role === 'admin')
const roleText = computed(() => ({ admin: '管理员', operator: '运维', user: '普通用户' }[userStore.userInfo.role] || '用户'))

const settingsVisible = ref(false)

const menus = computed(() => [
  { path: '/dashboard', title: '概览', icon: Odometer },
  { path: '/servers', title: '服务器管理', icon: Platform },
  { path: '/terminal', title: 'Web终端', icon: Monitor },
  { path: '/sftp', title: '文件管理', icon: Folder },
  ...(isAdmin.value ? [
    { path: '/users', title: '用户管理', icon: User },
    { path: '/sessions', title: '在线会话', icon: Connection }
  ] : []),
  { path: '/logs', title: '审计日志', icon: Document }
])

function handleCommand(cmd) {
  if (cmd === 'logout') {
    userStore.logout()
    router.push('/login')
  } else if (cmd === 'settings') {
    settingsVisible.value = true
  }
}
</script>

<style scoped>
.main-layout { height: 100vh; background: var(--bg-page); }

/* ===== 图标导航栏 ===== */
.icon-rail {
  width: 58px; background: var(--bg-rail);
  border-right: 1px solid var(--border-soft);
  display: flex; flex-direction: column; align-items: center;
  padding: 10px 0;
}

.rail-menus { display: flex; flex-direction: column; gap: 6px; flex: 1; }
.rail-item {
  width: 42px; height: 42px;
  display: flex; align-items: center; justify-content: center;
  border-radius: 10px; cursor: pointer;
  color: var(--text-2);
  transition: all .15s;
}
.rail-item:hover { color: var(--text-1); background: var(--hover-overlay); }
.rail-item.active { color: var(--accent); background: var(--active-overlay); }

.rail-about {
  width: 40px; height: 40px; border-radius: 9px; margin-top: 8px;
  display: flex; align-items: center; justify-content: center;
  color: var(--text-2); cursor: pointer;
  transition: all .15s;
}
.rail-about:hover { color: var(--accent); background: var(--active-overlay); }

/* ===== 顶部条：加高加大气，居中品牌 + 右侧用户 ===== */
.top-user-bar {
  height: 58px; flex-shrink: 0;
  display: flex; align-items: center; justify-content: flex-end;
  padding: 0 20px;
  background:
    linear-gradient(180deg, rgba(255,255,255,.035), rgba(0,0,0,.06)),
    var(--bg-bar);
  border-bottom: 1px solid var(--border-soft);
  box-shadow: 0 2px 14px rgba(0,0,0,.18);
  position: relative; z-index: 5;
}
.brand {
  position: absolute; left: 50%; top: 50%; transform: translate(-50%, -50%);
  display: flex; align-items: center; gap: 12px; user-select: none; white-space: nowrap;
}
.brand-logo { width: 30px; height: 30px; object-fit: contain; border-radius: 7px; }
.brand-title { color: var(--text-1); font-size: calc(18px * var(--fs, 1)); font-weight: 700; letter-spacing: 3px; }
.user-trigger {
  display: flex; align-items: center; gap: 7px; cursor: pointer;
  color: var(--text-2); font-size: calc(14px * var(--fs, 1));
  padding: 7px 14px; border-radius: 8px; transition: all .15s;
}
.user-trigger:hover { color: var(--text-1); background: var(--hover-overlay); }
.user-name { font-size: calc(14px * var(--fs, 1)); }

/* ===== 页面容器 ===== */
.page-host { flex: 1; min-height: 0; position: relative; overflow-y: auto; overflow-x: hidden; }

/* ===== 移动端 ===== */
@media (max-width: 640px) {
  .top-user-bar { height: 48px; padding: 0 10px; }
  .brand { left: 12px; transform: translateY(-50%); gap: 6px; }
  .brand-title { display: none; }
  .user-name { display: none; }
}

/* ===== 内容区 ===== */
.content { background: var(--bg-page); padding: 0; overflow: hidden; position: relative; display: flex; flex-direction: column; }
</style>
