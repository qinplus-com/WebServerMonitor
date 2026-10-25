import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

import App from './App.vue'
import router from './router'
import { useAppStore } from './store/app'
import './assets/style.css'

// 挂载前恢复主题偏好（dark=默认 / light），避免闪变
if (localStorage.getItem('qj_theme') === 'light') {
  document.documentElement.classList.add('light')
}

const app = createApp(App)

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// 挂载前先获取软件名称（登录页标题、品牌区均依赖此数据）；失败不阻塞挂载
const appStore = useAppStore()
appStore.load().finally(() => app.mount('#app'))
