import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue') },
  // H5 远程桌面（全屏独立页，不属于主布局）
  { path: '/rdp/:id', name: 'Rdp', component: () => import('../views/Rdp.vue') },
  {
    path: '/',
    component: () => import('../layout/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '概览' } },
      { path: 'servers', name: 'Servers', component: () => import('../views/Servers.vue'), meta: { title: '服务器管理' } },
      { path: 'terminal', name: 'Terminal', component: () => import('../views/Terminal.vue'), meta: { title: 'Web终端' } },
      { path: 'sftp', name: 'Sftp', component: () => import('../views/Sftp.vue'), meta: { title: '文件管理' } },
      { path: 'users', name: 'Users', component: () => import('../views/Users.vue'), meta: { title: '用户管理' } },
      { path: 'logs', name: 'Logs', component: () => import('../views/Logs.vue'), meta: { title: '审计日志' } },
      { path: 'sessions', name: 'Sessions', component: () => import('../views/Sessions.vue'), meta: { title: '在线会话' } },
      { path: 'about', name: 'About', component: () => import('../views/About.vue'), meta: { title: '关于' } }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.path !== '/login' && !token) {
    next('/login')
  } else if (to.path === '/login' && token) {
    next('/')
  } else {
    next()
  }
})

export default router
