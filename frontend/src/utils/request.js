import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000
})

request.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers['Authorization'] = 'Bearer ' + token
  }
  return config
})

request.interceptors.response.use(
  async response => {
    // blob 响应（文件下载/日志导出）：直接返回整个 response，不走业务 code 判断
    if (response.data instanceof Blob) {
      // 后端业务错误（如文件不存在/权限不足）也会被 responseType:blob 包成 Blob，
      // 且其 type 多为 application/json —— 还原为业务错误提示，避免下载到含错误信息的文本文件
      if (response.data.type && response.data.type.includes('application/json')) {
        try {
          const text = await response.data.text()
          const errRes = JSON.parse(text)
          if (errRes && errRes.code === 401) {
            localStorage.removeItem('token')
            localStorage.removeItem('userInfo')
            router.push('/login')
          }
          if (!response.config?.silent) {
            ElMessage.error(errRes?.msg || '请求失败')
          }
          return Promise.reject(new Error(errRes?.msg || '请求失败'))
        } catch (_) {
          // 非 JSON，按正常 blob 返回
        }
      }
      return response
    }
    const res = response.data
    if (res.code === 200) {
      return res
    }
    if (res.code === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('userInfo')
      router.push('/login')
    }
    // 带 silent 标记的请求由调用方自行展示错误（如 SSH 连接失败用对话框）
    if (!response.config?.silent) {
      // 演示模式的禁用操作统一用 warning 提示（非真实错误）
      if (typeof res.msg === 'string' && res.msg.startsWith('演示模式：')) {
        ElMessage.warning(res.msg)
      } else {
        ElMessage.error(res.msg || '请求失败')
      }
    }
    return Promise.reject(new Error(res.msg))
  },
  error => {
    if (!error.config?.silent) {
      ElMessage.error(error.message || '网络错误')
    }
    return Promise.reject(error)
  }
)

export default request
