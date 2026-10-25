<template>
  <div class="sftp-page">
    <div class="toolbar">
      <el-select v-model="selectedServer" placeholder="选择服务器" filterable style="width: 260px" @change="loadList">
        <el-option v-for="s in servers" :key="s.id" :label="`${s.name} (${s.ip})`" :value="s.id" />
      </el-select>
      <el-button @click="goBack" :disabled="!canGoBack"><el-icon><ArrowLeft /></el-icon> 上级目录</el-button>
      <span class="current-path">{{ currentPath }}</span>
      <el-button type="success" @click="showMkdir = true" :disabled="!selectedServer"><el-icon><FolderAdd /></el-icon> 新建文件夹</el-button>
      <el-button type="primary" @click="pickFiles" :disabled="!selectedServer || uploading"><el-icon><Upload /></el-icon> 上传文件</el-button>
      <el-button type="primary" @click="pickFolder" :disabled="!selectedServer || uploading"><el-icon><FolderOpened /></el-icon> 上传文件夹</el-button>
      <input ref="fileInput" type="file" multiple style="display:none" @change="onFilesPicked" />
      <input ref="folderInput" type="file" webkitdirectory directory multiple style="display:none" @change="onFilesPicked" />
      <span v-if="uploading" class="upload-progress">上传中 {{ uploadDone }}/{{ uploadTotal }}</span>
    </div>

    <el-card v-loading="loading" shadow="never">
      <el-table :data="files" border stripe>
        <el-table-column label="名称" min-width="240">
          <template #default="{ row }">
            <el-icon :style="{ color: row.is_dir ? '#e6a23c' : '#909399' }"><Folder v-if="row.is_dir" /><Document v-else /></el-icon>
            <span class="file-name" @click="row.is_dir && enterDir(row)">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="size" label="大小" width="120">
          <template #default="{ row }">{{ row.is_dir ? '-' : formatSize(row.size) }}</template>
        </el-table-column>
        <el-table-column prop="perm" label="权限" width="120" />
        <el-table-column prop="mod_time" label="修改时间" width="180" />
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="downloadFile(row)" :disabled="row.is_dir">下载</el-button>
            <el-button size="small" @click="renameFile(row)">重命名</el-button>
            <el-button size="small" type="danger" @click="deleteFile(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showMkdir" title="新建文件夹" width="min(460px, 94%)">
      <el-input v-model="mkdirName" placeholder="文件夹名称" />
      <template #footer>
        <el-button @click="showMkdir = false">取消</el-button>
        <el-button type="primary" @click="mkdir">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showRename" title="重命名" width="min(460px, 94%)">
      <el-input v-model="newName" placeholder="新名称" />
      <template #footer>
        <el-button @click="showRename = false">取消</el-button>
        <el-button type="primary" @click="doRename">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { sftpApi, serverApi } from '../api'

const route = useRoute()
const servers = ref([])
const selectedServer = ref(null)
const currentPath = ref('/')
const pathHistory = ref(['/'])
const files = ref([])
const loading = ref(false)
const showMkdir = ref(false)
const showRename = ref(false)
const mkdirName = ref('')
const newName = ref('')
let renameTarget = null

const canGoBack = ref(false)

// 上传相关状态
const uploading = ref(false)
const uploadTotal = ref(0)
const uploadDone = ref(0)
const fileInput = ref(null)
const folderInput = ref(null)

function pickFiles() { fileInput.value && fileInput.value.click() }
function pickFolder() { folderInput.value && folderInput.value.click() }

async function onFilesPicked(e) {
  const files = Array.from(e.target.files || [])
  if (!files.length) return
  uploading.value = true
  uploadTotal.value = files.length
  uploadDone.value = 0
  let failed = 0
  try {
    for (const f of files) {
      // webkitRelativePath 形如 "myfolder/sub/file.txt"（文件夹上传时）；
      // 单纯多选文件时该值为空，回退到 file.name
      const rel = f.webkitRelativePath || f.name
      try {
        const formData = new FormData()
        formData.append('file', f)
        formData.append('server_id', selectedServer.value)
        formData.append('target_path', currentPath.value)
        if (rel && rel !== f.name) formData.append('relative_path', rel)
        await sftpApi.upload(formData)
      } catch (err) {
        failed++
      }
      uploadDone.value++
    }
  } finally {
    uploading.value = false
    const ok = files.length - failed
    if (failed === 0) ElMessage.success(`上传完成，共 ${files.length} 个文件`)
    else ElMessage.warning(`成功 ${ok} 个，失败 ${failed} 个`)
    e.target.value = ''
    loadList()
  }
}

onMounted(async () => {
  const res = await serverApi.list({})
  servers.value = res.data
  if (route.query.server_id) {
    selectedServer.value = Number(route.query.server_id)
    loadList()
  }
})

async function loadList() {
  if (!selectedServer.value) return
  loading.value = true
  try {
    const res = await sftpApi.list({ server_id: selectedServer.value, path: currentPath.value })
    files.value = res.data.files
    canGoBack.value = currentPath.value !== '/'
  } finally { loading.value = false }
}

function enterDir(row) {
  const newPath = currentPath.value === '/' ? '/' + row.name : currentPath.value + '/' + row.name
  pathHistory.value.push(currentPath.value)
  currentPath.value = newPath
  loadList()
}

function goBack() {
  if (pathHistory.value.length > 1) {
    pathHistory.value.pop()
    currentPath.value = pathHistory.value[pathHistory.value.length - 1]
    loadList()
  } else {
    currentPath.value = '/'
    pathHistory.value = ['/']
    loadList()
  }
}

async function downloadFile(row) {
  const res = await sftpApi.download({ server_id: selectedServer.value, file_path: currentPath.value === '/' ? '/' + row.name : currentPath.value + '/' + row.name })
  const url = URL.createObjectURL(res.data)
  const a = document.createElement('a')
  a.href = url
  a.download = row.name
  a.click()
  URL.revokeObjectURL(url)
}

async function deleteFile(row) {
  await ElMessageBox.confirm(`确认删除【${row.name}】？`, '提示', { type: 'warning' })
  const path = currentPath.value === '/' ? '/' + row.name : currentPath.value + '/' + row.name
  await sftpApi.operate({ server_id: selectedServer.value, path, operate_type: 'delete' })
  ElMessage.success('删除成功')
  loadList()
}

function renameFile(row) {
  renameTarget = row
  newName.value = row.name
  showRename.value = true
}

async function doRename() {
  const path = currentPath.value === '/' ? '/' + renameTarget.name : currentPath.value + '/' + renameTarget.name
  await sftpApi.operate({ server_id: selectedServer.value, path, operate_type: 'rename', new_name: newName.value })
  ElMessage.success('重命名成功')
  showRename.value = false
  loadList()
}

async function mkdir() {
  if (!mkdirName.value) return
  const path = currentPath.value === '/' ? '/' + mkdirName.value : currentPath.value + '/' + mkdirName.value
  await sftpApi.operate({ server_id: selectedServer.value, path, operate_type: 'mkdir' })
  ElMessage.success('创建成功')
  showMkdir.value = false
  mkdirName.value = ''
  loadList()
}

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}
</script>

<style scoped>
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
.current-path { color: var(--text-2); font-size: calc(13px * var(--fs, 1)); flex: 1; }
.file-name { cursor: pointer; margin-left: 8px; }
.file-name:hover { color: #409eff; }
.upload-progress { color: #67c23a; font-size: calc(13px * var(--fs, 1)); }

/* ===== 窄屏：服务器选择与上传占满整行，路径独占一行 ===== */
@media (max-width: 640px) {
  .toolbar > .el-select { width: 100% !important; }
  .current-path { flex-basis: 100%; }
  .upload-progress { flex-basis: 100%; }
}
</style>
