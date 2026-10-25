<template>
  <div class="sessions">
    <el-card shadow="never">
      <div class="toolbar">
        <el-button type="primary" @click="loadList"><el-icon><Refresh /></el-icon> 刷新</el-button>
      </div>
      <el-table :data="list" border stripe v-loading="loading">
        <el-table-column prop="session_id" label="会话ID" width="280" />
        <el-table-column prop="user_id" label="用户ID" width="80" />
        <el-table-column prop="server_name" label="服务器" width="160" />
        <el-table-column prop="created_at" label="连接时间" width="200" />
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="kick(row)">强制下线</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { sessionApi } from '../api'

const list = ref([])
const loading = ref(false)

async function loadList() {
  loading.value = true
  try {
    const res = await sessionApi.online()
    list.value = res.data
  } finally { loading.value = false }
}

async function kick(row) {
  await ElMessageBox.confirm(`确认强制下线会话【${row.session_id.slice(0,8)}...】？`, '提示', { type: 'warning' })
  await sessionApi.kick(row.session_id)
  ElMessage.success('已强制下线')
  loadList()
}

onMounted(loadList)
</script>

<style scoped>.toolbar { margin-bottom: 16px; }</style>
