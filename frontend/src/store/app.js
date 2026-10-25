import { defineStore } from 'pinia'
import request from '../utils/request'

const DEFAULT_NAME = '运维管理平台'

export const useAppStore = defineStore('app', {
  state: () => ({
    appName: DEFAULT_NAME,
    company: '',
    website: '',
    copyright: '',
    loaded: false,
    theme: localStorage.getItem('qj_theme') === 'light' ? 'light' : 'dark'
  }),
  actions: {
    // 明暗模式切换：写入 localStorage 并同步 html.light 类
    toggleTheme() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark'
      localStorage.setItem('qj_theme', this.theme)
      document.documentElement.classList.toggle('light', this.theme === 'light')
    },
    // 挂载前调用：获取软件名称与品牌信息，并同步浏览器标题
    async load() {
      if (this.loaded) return
      try {
        const res = await request.get('/system/info')
        this.appName = res.data.app_name || DEFAULT_NAME
        this.company = res.data.company || ''
        this.website = res.data.website || ''
        this.copyright = res.data.copyright || ''
        this.loaded = true
        document.title = this.appName
      } catch (e) {
        // 接口异常时使用默认名称，不阻塞页面渲染
        document.title = this.appName
      }
    }
  }
})
