<template>
  <div class="users">
    <el-card shadow="never">
      <div class="toolbar">
        <el-input v-model="keyword" placeholder="搜索用户名" clearable style="width:240px" @keyup.enter="loadList" />
        <el-button type="primary" @click="loadList">搜索</el-button>
        <el-button type="success" @click="openEdit()"><el-icon><Plus /></el-icon> 新增用户</el-button>
      </div>
      <el-table :data="list" border stripe>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="username" label="用户名" width="140" />
        <el-table-column label="角色" width="120">
          <template #default="{ row }"><el-tag :type="row.role === 'admin' ? 'danger' : row.role === 'operator' ? 'warning' : 'info'">{{ roleText(row.role) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }"><el-tag :type="row.status ? 'success' : 'danger'" size="small">{{ row.status ? '启用' : '禁用' }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="120" show-overflow-tooltip />
        <el-table-column label="可访问的服务器" min-width="200">
          <template #default="{ row }">
            <template v-if="row.server_names && row.server_names.length">
              <el-tag v-for="name in row.server_names.slice(0, 3)" :key="name" size="small" type="success" class="server-tag">{{ name }}</el-tag>
              <el-tooltip v-if="row.server_names.length > 3" :content="row.server_names.slice(3).join('、')" placement="top">
                <el-tag size="small" type="info">+{{ row.server_names.length - 3 }}</el-tag>
              </el-tooltip>
            </template>
            <span v-else class="no-server">{{ row.role === 'admin' ? '全部服务器' : '未授权' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="170" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openPermission(row)">分配权限</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="delUser(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editForm.id ? '编辑用户' : '新增用户'" width="min(560px, 94%)">
      <el-form :model="editForm" label-width="80px">
        <el-form-item label="用户名"><el-input v-model="editForm.username" :disabled="!!editForm.id" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="editForm.password" type="password" show-password :placeholder="editForm.id ? '不修改留空' : '请输入密码'" /></el-form-item>
        <el-form-item label="角色"><el-select v-model="editForm.role" style="width:100%"><el-option label="管理员" value="admin" /><el-option label="运维" value="operator" /><el-option label="普通用户" value="user" /></el-select></el-form-item>
        <el-form-item label="状态"><el-switch v-model="editForm.status" :active-value="1" :inactive-value="0" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="editForm.remark" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible=false">取消</el-button><el-button type="primary" @click="save">确定</el-button></template>
    </el-dialog>

    <el-dialog v-model="permVisible" title="分配服务器权限" width="min(880px, 94%)">
      <el-transfer
        v-model="permServerIds"
        :data="allServers"
        :titles="['未授权', '已授权']"
        :props="{ key: 'value', label: 'label' }"
        filterable
        filter-placeholder="搜索服务器名称"
        class="perm-transfer"
      />
      <template #footer><el-button @click="permVisible=false">取消</el-button><el-button type="primary" @click="savePermission">确定</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { userApi, serverApi } from '../api'

const list = ref([])
const keyword = ref('')
const dialogVisible = ref(false)
const permVisible = ref(false)
const permServerIds = ref([])
const allServers = ref([])
let currentUserId = null
const editForm = reactive({ id: null, username: '', password: '', role: 'user', status: 1, remark: '' })

function roleText(r) { return { admin: '管理员', operator: '运维', user: '普通用户' }[r] || r }

async function loadList() {
  const res = await userApi.list({ keyword: keyword.value })
  list.value = res.data
}
function openEdit(row) {
  Object.assign(editForm, { id: null, username: '', password: '', role: 'user', status: 1, remark: '' })
  if (row) Object.assign(editForm, row)
  dialogVisible.value = true
}
async function save() {
  if (!editForm.username) return ElMessage.warning('请输入用户名')
  if (!editForm.id && !editForm.password) return ElMessage.warning('请输入密码')
  if (editForm.id) { await userApi.edit(editForm) } else { await userApi.add(editForm) }
  ElMessage.success('保存成功')
  dialogVisible.value = false
  loadList()
}
async function delUser(row) {
  await ElMessageBox.confirm(`确认删除用户【${row.username}】？`, '提示', { type: 'warning' })
  await userApi.del(row.id)
  ElMessage.success('删除成功')
  loadList()
}
async function openPermission(row) {
  currentUserId = row.id
  const [sres, ures] = await Promise.all([serverApi.list({}), userApi.servers(row.id)])
  allServers.value = sres.data.map(s => ({ value: s.id, label: `${s.name} (${s.ip})` }))
  permServerIds.value = ures.data
  permVisible.value = true
}
async function savePermission() {
  await userApi.permission({ user_id: currentUserId, server_ids: permServerIds.value })
  ElMessage.success('权限分配成功')
  permVisible.value = false
}
onMounted(loadList)
</script>

<style scoped>
.toolbar { display: flex; gap: 12px; margin-bottom: 16px; }
.server-tag { margin-right: 4px; margin-bottom: 2px; }
.no-server { color: #909399; font-size: calc(13px * var(--fs, 1)); }
/* 分配权限穿梭框：两栏等宽，名称超长省略+悬浮显示完整名称，避免换行错乱 */
.perm-transfer { display: flex; justify-content: center; }
.perm-transfer :deep(.el-transfer-panel) { width: 280px; }
.perm-transfer :deep(.el-transfer-panel__body) { height: 360px; }
.perm-transfer :deep(.el-transfer-panel__item) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  height: auto;
  line-height: 2.4;
  padding-right: 12px;
}
</style>
