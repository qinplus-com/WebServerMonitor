import { defineStore } from 'pinia'
import request from '../utils/request'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    userInfo: JSON.parse(localStorage.getItem('userInfo') || '{}')
  }),
  actions: {
    async login(username, password) {
      const res = await request.post('/login', { username, password })
      this.token = res.data.token
      this.userInfo = { username: res.data.username, role: res.data.role }
      localStorage.setItem('token', res.data.token)
      localStorage.setItem('userInfo', JSON.stringify(this.userInfo))
      return res
    },
    logout() {
      this.token = ''
      this.userInfo = {}
      localStorage.removeItem('token')
      localStorage.removeItem('userInfo')
    }
  }
})
