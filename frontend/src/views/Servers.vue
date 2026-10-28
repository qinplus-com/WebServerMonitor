<template>
  <div class="servers">
    <el-card shadow="never">
      <div class="toolbar">
        <el-input v-model="keyword" placeholder="搜索服务器名称/IP" clearable style="width: 260px" @keyup.enter="loadList" :prefix-icon="Search" />
        <el-select v-model="groupId" placeholder="全部分组" clearable style="width: 160px" @change="loadList">
          <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
        </el-select>
        <el-button type="primary" @click="loadList"><el-icon><Refresh /></el-icon> 刷新</el-button>
        <el-button v-if="isAdmin" type="success" @click="openEdit()"><el-icon><Plus /></el-icon> 新增服务器</el-button>
        <el-button v-if="isAdmin" @click="openGroup()"><el-icon><FolderAdd /></el-icon> 分组管理</el-button>
      </div>

      <el-table :data="list" border stripe v-loading="loading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="服务器名称" min-width="140" />
        <el-table-column label="类型" width="92">
          <template #default="{ row }"><el-tag :type="isWin(row) ? 'warning' : 'info'" size="small">{{ isWin(row) ? 'Windows' : 'Linux' }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="ip" label="IP地址" width="160" />
        <el-table-column prop="port" label="端口" width="80" />
        <el-table-column prop="username" label="登录账号" width="100" />
        <el-table-column prop="group_name" label="分组" width="120" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }"><el-tag :type="row.status ? 'success' : 'info'" size="small">{{ row.status ? '在线' : '离线' }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="140" show-overflow-tooltip />
        <el-table-column label="操作" :width="isAdmin ? 420 : 290" fixed="right">
          <template #default="{ row }">
            <div class="op-cell">
              <el-button v-if="!isWin(row)" link type="primary" @click="openTerminal(row)"><el-icon><Monitor /></el-icon> 终端</el-button>
              <el-button link type="warning" @click="openSftp(row)"><el-icon><Folder /></el-icon> 文件</el-button>
              <!-- Windows 始终可远程桌面；Linux 需已配置 VNC 端口才显示入口 -->
              <el-dropdown v-if="isWin(row) || row.vnc_port > 0" trigger="click" @command="cmd => openRemote(cmd, row)">
                <el-button link type="danger" class="rdp-btn"><el-icon><Connection /></el-icon> 远程桌面<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item v-if="isWin(row)" command="local"><el-icon><Launch /></el-icon> 本地RDP（mstsc）</el-dropdown-item>
                    <el-dropdown-item command="h5"><el-icon><Monitor /></el-icon> {{ isWin(row) ? 'H5 浏览器桌面' : 'H5 浏览器桌面（VNC）' }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <el-button v-if="isAdmin" link type="primary" @click="openEdit(row)"><el-icon><Edit /></el-icon> 编辑</el-button>
              <el-button v-if="isAdmin" link type="danger" @click="delServer(row)"><el-icon><Delete /></el-icon> 删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新增/编辑服务器 -->
    <el-dialog v-model="dialogVisible" :title="editForm.id ? '编辑服务器' : '新增服务器'" width="min(720px, 94%)">
      <el-form :model="editForm" label-width="100px">
        <el-form-item label="名称"><el-input v-model="editForm.name" /></el-form-item>
        <el-form-item label="服务器类型">
          <el-radio-group v-model="editForm.os_type" @change="onTypeChange">
            <el-radio value="linux">Linux</el-radio>
            <el-radio value="windows">Windows（支持远程桌面）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="IP"><el-input v-model="editForm.ip" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="editForm.port" :min="1" :max="65535" /></el-form-item>
        <el-form-item v-if="editForm.os_type === 'windows'" label="文件共享名">
          <div class="vnc-editor">
            <el-input v-model="editForm.smb_share" placeholder="目标机的共享文件夹名称，如 share" style="width: 240px" />
            <span class="vnc-label">SMB 端口</span>
            <el-input-number v-model="editForm.smb_port" :min="0" :max="65535" controls-position="right" style="width: 120px" placeholder="445" />
          </div>
          <div class="urls-tip"><span>Windows 文件管理通过 SMB 共享文件夹：先在目标机右键文件夹 → 属性 → 共享，给登录账号授予读写权限，再填共享名称。端口默认 445；跨公网访问时需在防火墙/云安全组放行（建议仅对平台服务器 IP 白名单开放），端口映射场景填映射后的公网端口</span></div>
        </el-form-item>
        <el-form-item v-if="editForm.os_type === 'linux'" label="远程桌面">
          <div class="vnc-editor">
            <el-switch :model-value="vncOn" @update:model-value="setVncOn" />
            <template v-if="vncOn">
              <span class="vnc-label">VNC 端口</span>
              <el-input-number v-model="editForm.vnc_port" :min="1" :max="65535" controls-position="right" style="width: 120px" />
            </template>
          </div>
          <div v-if="vncOn" class="urls-tip"><span>目标机需运行 VNC 服务（如 tigervnc :1 → 端口 5901），认证密码取服务器登录密码，部署步骤见《Linux远程桌面(VNC).md》文档</span></div>
        </el-form-item>
        <el-form-item label="登录账号"><el-input v-model="editForm.username" /></el-form-item>
        <el-form-item label="认证方式"><el-radio-group v-model="editForm.auth_type"><el-radio value="password">密码</el-radio><el-radio value="key">密钥</el-radio></el-radio-group></el-form-item>
        <el-form-item v-if="editForm.auth_type === 'password'" label="密码"><el-input v-model="editForm.password" type="password" show-password /></el-form-item>
        <el-form-item v-if="editForm.auth_type === 'key'" label="私钥"><el-input v-model="editForm.private_key" type="textarea" :rows="4" /></el-form-item>
        <el-form-item label="分组"><el-select v-model="editForm.group_id" clearable style="width:100%"><el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" /></el-select></el-form-item>
        <el-form-item label="网站地址">
          <div class="urls-editor">
            <div v-for="(u, idx) in editUrls" :key="idx" class="url-row">
              <el-select v-model="u.protocol" style="width:95px" @change="syncWwwProtocol(idx)">
                <el-option label="https" value="https" />
                <el-option label="http" value="http" />
              </el-select>
              <el-input v-model="u.domain" placeholder="域名，如 example.com" style="flex:1" @blur="autoWww(idx)" />
              <el-input-number v-model="u.port" :min="0" :max="65535" controls-position="right" style="width:110px" placeholder="端口" />
              <el-button type="danger" circle :icon="Delete" @click="editUrls.splice(idx, 1)" />
            </div>
            <div class="urls-tip">
              <el-button size="small" @click="addUrlRow"><el-icon><Plus /></el-icon> 添加网址</el-button>
              <span>端口留0用默认（https:443 / http:80）。输入主域名后自动补充 www 域名，可在概览页查看网址监控。</span>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="editForm.remark" /></el-form-item>
        <el-form-item label="授权用户">
          <el-select v-model="editGrantUserIds" multiple filterable collapse-tags collapse-tags-tooltip
            placeholder="不选则仅管理员可访问" style="width:100%">
            <el-option-group v-for="(users, role) in usersByRole" :key="role" :label="roleLabel(role)">
              <el-option v-for="u in users" :key="u.id" :label="u.username + (u.remark ? `（${u.remark}）` : '')" :value="u.id" />
            </el-option-group>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveServer">确定</el-button>
      </template>
    </el-dialog>

    <!-- 分组管理 -->
    <el-dialog v-model="groupVisible" title="分组管理" width="min(560px, 94%)">
      <div class="group-add">
        <el-input v-model="newGroupName" placeholder="新分组名称" style="width:200px" />
        <el-button type="primary" @click="addGroup">添加</el-button>
      </div>
      <el-table :data="groups" border style="margin-top:12px">
        <el-table-column prop="name" label="分组名称" />
        <el-table-column label="操作" width="100">
          <template #default="{ row }"><el-button size="small" type="danger" @click="delGroup(row)">删除</el-button></template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete } from '@element-plus/icons-vue'
import { serverApi, userApi, rdpApi } from '../api'
import { useUserStore } from '../store/user'

const router = useRouter()
const userStore = useUserStore()
const isAdmin = computed(() => userStore.userInfo.role === 'admin')
const list = ref([])
const groups = ref([])
const keyword = ref('')
const groupId = ref('')
const loading = ref(false)
const dialogVisible = ref(false)
const groupVisible = ref(false)
const newGroupName = ref('')
const editForm = reactive({ id: null, name: '', ip: '', port: 22, os_type: 'linux', vnc_port: 0, smb_share: '', smb_port: 0, username: '', auth_type: 'password', password: '', private_key: '', group_id: null, remark: '' })

// 是否 Windows 服务器（存量数据 os_type 为空按 Linux 处理）
function isWin(row) { return row.os_type === 'windows' }
// Linux VNC 远程桌面开关（vnc_port 0=关，开启默认 5901）
const vncOn = computed(() => editForm.vnc_port > 0)
function setVncOn(v) {
  if (v && !editForm.vnc_port) editForm.vnc_port = 5901
  if (!v) editForm.vnc_port = 0
}
// 类型切换时，端口处于另一类型的默认值则联动（22 ↔ 3389），用户自定义端口不动
function onTypeChange(os) {
  const d = { linux: 22, windows: 3389 }
  if (editForm.port === d[os === 'windows' ? 'linux' : 'windows'] || !editForm.port) {
    editForm.port = d[os]
  }
}
const editUrls = ref([])
const editGrantUserIds = ref([])
const allUsers = ref([])

// 按角色分组展示用户
const usersByRole = computed(() => {
  const g = {}
  for (const u of allUsers.value) {
    (g[u.role] = g[u.role] || []).push(u)
  }
  return g
})
function roleLabel(role) {
  return { admin: '管理员', operator: '操作员', user: '普通用户' }[role] || role
}

async function loadGroups() {
  const res = await serverApi.groupList()
  groups.value = res.data
}
async function loadList() {
  loading.value = true
  try {
    const res = await serverApi.list({ group_id: groupId.value, keyword: keyword.value })
    list.value = res.data
  } finally { loading.value = false }
}
function addUrlRow(protocol = 'https', domain = '', port = 0) {
  editUrls.value.push({ protocol, domain, port })
}
// 输入主域名失焦后自动补充 www 域名行（已存在则跳过）
function autoWww(idx) {
  const row = editUrls.value[idx]
  const d = (row.domain || '').trim().replace(/^https?:\/\//, '').split('/')[0]
  row.domain = d
  if (!d || d.startsWith('www.')) return
  const www = 'www.' + d
  if (editUrls.value.some(u => u.domain === www)) return
  editUrls.value.splice(idx + 1, 0, { protocol: row.protocol, domain: www, port: row.port || 0 })
}
// 主域名行改协议时，同步其紧邻的www行协议
function syncWwwProtocol(idx) {
  const row = editUrls.value[idx]
  if (!row.domain || row.domain.startsWith('www.')) return
  const www = 'www.' + row.domain
  const wwwRow = editUrls.value.find(u => u.domain === www)
  if (wwwRow) wwwRow.protocol = row.protocol
}
async function openEdit(row) {
  Object.assign(editForm, { id: null, name: '', ip: '', port: 22, os_type: 'linux', vnc_port: 0, smb_share: '', smb_port: 0, username: '', auth_type: 'password', password: '', private_key: '', group_id: null, remark: '' })
  editUrls.value = []
  editGrantUserIds.value = []
  // 加载全部用户（用于授权选择）
  if (isAdmin.value) {
    try {
      const res = await userApi.list({})
      allUsers.value = res.data || []
    } catch { allUsers.value = [] }
  }
  if (row) {
    Object.assign(editForm, row)
    // 存量数据 os_type 为空按 Linux
    if (editForm.os_type !== 'windows') editForm.os_type = 'linux'
    editForm.vnc_port = row.vnc_port || 0
    editForm.smb_share = row.smb_share || ''
    editForm.smb_port = row.smb_port || 0
    // 加载已保存的网址（协议白名单容错）
    try {
      const res = await serverApi.urlList(row.id)
      editUrls.value = (res.data || []).map(u => ({
        protocol: ['http', 'https'].includes(u.protocol) ? u.protocol : 'https',
        domain: u.domain || '',
        port: u.port || 0
      }))
    } catch { editUrls.value = [] }
    // 加载已授权用户
    try {
      const res = await serverApi.grantUsers(row.id)
      editGrantUserIds.value = res.data || []
    } catch { editGrantUserIds.value = [] }
  }
  dialogVisible.value = true
}
async function saveServer() {
  const fn = editForm.id ? serverApi.edit : serverApi.add
  const res = await fn(editForm)
  // 保存网址列表和用户授权（新服务器需先拿到id）
  const serverId = editForm.id || (res.data && res.data.id)
  if (serverId) {
    const urls = editUrls.value.filter(u => (u.domain || '').trim())
    await serverApi.urlSave({ server_id: serverId, urls })
    await serverApi.grantSave({ server_id: serverId, user_ids: editGrantUserIds.value })
  }
  ElMessage.success('保存成功')
  dialogVisible.value = false
  loadList()
}
async function delServer(row) {
  await ElMessageBox.confirm(`确认删除服务器【${row.name}】？`, '提示', { type: 'warning' })
  await serverApi.del(row.id)
  ElMessage.success('删除成功')
  loadList()
}
function openGroup() { loadGroups(); groupVisible.value = true }
async function addGroup() {
  if (!newGroupName.value) return
  await serverApi.groupAdd({ name: newGroupName.value })
  newGroupName.value = ''
  ElMessage.success('添加成功')
  loadGroups()
}
async function delGroup(row) {
  await serverApi.groupDel(row.id)
  ElMessage.success('删除成功')
  loadGroups()
}
function openTerminal(row) { router.push({ path: '/terminal', query: { server_id: row.id } }) }
function openSftp(row) { router.push({ path: '/sftp', query: { server_id: row.id } }) }

// ===== 远程桌面（Windows 服务器） =====
function openRemote(cmd, row) {
  if (cmd === 'local') return downloadRdp(row)
  // h5：先取一次性凭证，再进入全屏桌面页（Linux 走 VNC 协议）
  rdpApi.token(row.id).then(res => {
    const token = res.data && res.data.token
    if (!token) return
    router.push({ path: `/rdp/${row.id}`, query: { token, proto: res.data.proto || 'rdp' } })
  }).catch(() => {}) // 错误提示由 request 拦截器统一展示
}
// 生成 .rdp 文件下载，mstsc 打开后输入密码（.rdp 无法内嵌明文密码）
function downloadRdp(row) {
  const content = [
    'screen mode id:i:2',
    'use multimon:i:1',
    'desktopwidth:i:1280',
    'desktopheight:i:800',
    'session bpp:i:32',
    'winposstr:s:0,1,-4,-4,1284,804',
    'compression:i:1',
    'keyboardhook:i:2',
    'audiocapturemode:i:0',
    'videoplaybackmode:i:1',
    'connection type:i:7',
    'networkautodetect:i:1',
    'bandwidthautodetect:i:1',
    'displayconnectionbar:i:1',
    'disable wallpaper:i:0',
    'allow font smoothing:i:1',
    'allow desktop composition:i:1',
    'disable full window drag:i:1',
    'disable menu anims:i:1',
    'disable themes:i:0',
    'disable cursor setting:i:0',
    'bitmapcachepersistenable:i:1',
    `full address:s:${row.ip}:${row.port || 3389}`,
    'audiomode:i:0',
    'redirectprinters:i:0',
    'redirectcomports:i:0',
    'redirectsmartcards:i:0',
    'redirectclipboard:i:1',
    'redirectposdevices:i:0',
    'autoreconnection enabled:i:1',
    'authentication level:i:2',
    'prompt for credentials:i:0',
    'negotiate security layer:i:1',
    'remoteapplicationmode:i:0',
    'alternate shell:s:',
    'shell working directory:s:',
    'gatewayhostname:s:',
    'gatewayusagemethod:i:4',
    'gatewaycredentialssource:i:4',
    'gatewayprofileusagemethod:i:0',
    'promptcredentialonce:i:0',
    'drivestoredirect:s:',
    `username:s:${row.username || ''}`
  ].join('\r\n')
  const blob = new Blob([content], { type: 'application/octet-stream' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${row.name || row.ip}.rdp`
  a.click()
  URL.revokeObjectURL(a.href)
  ElMessage.success('已下载 .rdp 文件，双击打开后在弹窗中输入密码登录')
}

onMounted(() => { loadGroups(); loadList() })
</script>

<style scoped>
.toolbar { display: flex; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.group-add { display: flex; gap: 12px; }
.urls-editor { width: 100%; }
.url-row { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; flex-wrap: wrap; }
.urls-tip { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.urls-tip span { font-size: calc(12px * var(--fs, 1)); color: #909399; line-height: 1.5; }
.vnc-editor { display: flex; align-items: center; gap: 10px; }
.vnc-label { font-size: calc(13px * var(--fs, 1)); color: #606266; }

/* 操作列：紧凑单行不换行 */
.op-cell { display: flex; align-items: center; gap: 2px; white-space: nowrap; }
.op-cell .el-button { margin-left: 0; padding: 0 5px; }
.op-cell .el-button .el-icon { margin-right: 2px; }
.op-cell .el-dropdown { margin: 0 3px; }
.op-cell .rdp-btn .el-icon--right { margin-left: 0; }

/* ===== 窄屏：工具条控件占满整行堆叠 ===== */
@media (max-width: 640px) {
  .toolbar > .el-input, .toolbar > .el-select { width: 100% !important; }
  .toolbar > .el-button { flex: 1 1 auto; }
}
</style>
