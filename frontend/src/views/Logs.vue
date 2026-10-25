<template>
  <div class="logs">
    <el-card shadow="never">
      <el-tabs v-model="activeTab" @tab-change="loadList">
        <el-tab-pane label="登录日志" name="login" />
        <el-tab-pane label="命令日志" name="command" />
        <el-tab-pane label="文件操作日志" name="sftp" />
      </el-tabs>
      <div class="toolbar">
        <el-date-picker v-model="dateRange" type="daterange" range-separator="至" start-placeholder="开始日期" end-placeholder="结束日期" value-format="YYYY-MM-DD" />
        <el-button @click="loadList">查询</el-button>
        <el-button type="primary" @click="exportLogs"><el-icon><Download /></el-icon> 导出CSV</el-button>
      </div>

      <el-table :data="list" border stripe v-loading="loading">
        <template v-if="activeTab === 'login'">
          <el-table-column prop="username" label="用户名" width="120" />
          <el-table-column prop="ip" label="登录IP" width="140" />
          <el-table-column prop="user_agent" label="设备信息" min-width="200" show-overflow-tooltip />
          <el-table-column label="状态" width="80"><template #default="{ row }"><el-tag :type="row.status ? 'success' : 'danger'" size="small">{{ row.status ? '成功' : '失败' }}</el-tag></template></el-table-column>
          <el-table-column prop="message" label="说明" width="120" />
          <el-table-column prop="created_at" label="登录时间" width="180" />
        </template>
        <template v-else-if="activeTab === 'command'">
          <el-table-column prop="username" label="用户名" width="120" />
          <el-table-column prop="server_name" label="服务器" width="140" />
          <el-table-column prop="command" label="执行命令" min-width="300" show-overflow-tooltip />
          <el-table-column prop="session_id" label="会话ID" width="200" show-overflow-tooltip />
          <el-table-column prop="created_at" label="执行时间" width="180" />
        </template>
        <template v-else>
          <el-table-column prop="username" label="用户名" width="120" />
          <el-table-column prop="server_name" label="服务器" width="140" />
          <el-table-column label="操作" width="100"><template #default="{ row }"><el-tag size="small">{{ row.operate }}</el-tag></template></el-table-column>
          <el-table-column prop="path" label="文件路径" min-width="300" show-overflow-tooltip />
          <el-table-column prop="created_at" label="操作时间" width="180" />
        </template>
      </el-table>
      <el-pagination style="margin-top:16px;text-align:right" background layout="total, prev, pager, next" :total="total" :page-size="20" @current-change="(p) => { page = p; loadList() }" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { logApi } from '../api'

const activeTab = ref('login')
const list = ref([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const dateRange = ref([])

async function loadList() {
  loading.value = true
  try {
    const params = { page: page.value, limit: 20 }
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_time = dateRange.value[0] + ' 00:00:00'
      params.end_time = dateRange.value[1] + ' 23:59:59'
    }
    const fn = activeTab.value === 'login' ? logApi.login : activeTab.value === 'command' ? logApi.command : logApi.sftp
    const res = await fn(params)
    list.value = res.data.list
    total.value = res.data.total
  } finally { loading.value = false }
}

async function exportLogs() {
  const params = { log_type: activeTab.value }
  if (dateRange.value && dateRange.value.length === 2) {
    params.start_time = dateRange.value[0] + ' 00:00:00'
    params.end_time = dateRange.value[1] + ' 23:59:59'
  }
  const res = await logApi.export(params)
  const url = URL.createObjectURL(res.data)
  const a = document.createElement('a')
  a.href = url
  a.download = `${activeTab.value}_logs.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(loadList)
</script>

<style scoped>.toolbar { display: flex; gap: 12px; margin: 16px 0; flex-wrap: wrap; }</style>
