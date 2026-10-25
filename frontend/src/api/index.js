import request from '../utils/request'

export const serverApi = {
  list: (params) => request.get('/server/list', { params }),
  add: (data) => request.post('/server/add', data),
  edit: (data) => request.put('/server/edit', data),
  del: (id) => request.delete('/server/del', { params: { id } }),
  groupList: () => request.get('/server/group/list'),
  groupAdd: (data) => request.post('/server/group/add', data),
  groupDel: (id) => request.delete('/server/group/del', { params: { id } }),
  urlList: (server_id) => request.get('/server/url/list', { params: { server_id } }),
  urlSave: (data) => request.post('/server/url/save', data),
  grantUsers: (server_id) => request.get('/server/grant/users', { params: { server_id } }),
  grantSave: (data) => request.post('/server/grant/save', data)
}

// H5 远程桌面（Windows RDP）
export const rdpApi = {
  token: (server_id) => request.get('/rdp/token', { params: { server_id } })
}

export const monitorApi = {
  status: () => request.get('/monitor/status'),
  check: () => request.post('/monitor/check')
}

export const terminalApi = {
  connect: (server_id, config) => request.post('/terminal/connect', { server_id }, config),
  close: (session_id) => request.post('/terminal/close', { session_id }),
  history: (session_id) => request.get('/terminal/history', { params: { session_id } })
}

export const sftpApi = {
  list: (params) => request.get('/sftp/list', { params }),
  operate: (data) => request.post('/sftp/operate', data),
  upload: (formData) => request.post('/sftp/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } }),
  download: (params) => request.get('/sftp/download', { params, responseType: 'blob' })
}

export const userApi = {
  list: (params) => request.get('/user/list', { params }),
  add: (data) => request.post('/user/add', data),
  edit: (data) => request.put('/user/edit', data),
  del: (id) => request.delete('/user/del', { params: { id } }),
  servers: (user_id) => request.get('/user/servers', { params: { user_id } }),
  permission: (data) => request.post('/user/permission', data),
  prefsGet: () => request.get('/user/prefs'),
  prefsSave: (data) => request.put('/user/prefs', data)
}

export const logApi = {
  login: (params) => request.get('/log/login', { params }),
  command: (params) => request.get('/log/command', { params }),
  sftp: (params) => request.get('/log/sftp', { params }),
  export: (params) => request.get('/log/export', { params, responseType: 'blob' })
}

export const sessionApi = {
  online: () => request.get('/session/online'),
  kick: (session_id) => request.post('/session/kick', { session_id })
}

export const dashboardApi = {
  info: () => request.get('/dashboard')
}

export const aiApi = {
  chat: (data) => request.post('/ai/chat', data, { timeout: 120000 })
}

export const alertApi = {
  getSetting: () => request.get('/alert/setting'),
  saveSetting: (data) => request.put('/alert/setting', data),
  test: (data) => request.post('/alert/test', data)
}

export const licenseApi = {
  getInfo: () => request.get('/license/info'),
  activate: (data) => request.post('/license/activate', data)
}
