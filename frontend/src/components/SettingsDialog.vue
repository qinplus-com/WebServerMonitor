<template>
  <el-dialog v-model="visible" title="设置" width="min(880px, 94%)" :close-on-click-modal="false" top="6vh">
    <el-tabs v-model="activeTab">
      <el-tab-pane label="授权管理" name="license">
        <div class="license-info">
          <div class="lic-row">
            <span class="lic-label">当前版本</span>
            <el-tag :type="editionTagType(licenseInfo.edition)" size="large">
              {{ editionName(licenseInfo.edition) }}
            </el-tag>
          </div>
          <div class="lic-row">
            <span class="lic-label">服务器数量限制</span>
            <span class="lic-value">{{ licenseInfo.max_servers < 0 ? '不限' : licenseInfo.max_servers + ' 台' }}</span>
          </div>
          <div v-if="licenseInfo.is_trial" class="lic-row">
            <span class="lic-label">试用状态</span>
            <el-tag type="warning">剩余 {{ licenseInfo.days_left }} 天</el-tag>
          </div>
          <div v-else-if="licenseInfo.edition !== 'personal' && licenseInfo.edition !== 'community'" class="lic-row">
            <span class="lic-label">授权到期</span>
            <span class="lic-value">{{ licenseInfo.expire_at }}</span>
          </div>
          <div v-if="licenseInfo.issued_to" class="lic-row">
            <span class="lic-label">授权给</span>
            <span class="lic-value">{{ licenseInfo.issued_to }}</span>
          </div>

          <el-divider />

          <div class="lic-activate">
            <div class="lic-label" style="margin-bottom:8px">导入授权文件</div>
            <div class="lic-import-row">
              <el-input
                v-model="licenseCode"
                type="textarea"
                :rows="4"
                placeholder="请粘贴授权码，或上传授权文件自动提取"
              />
            </div>
            <div class="lic-actions">
              <el-upload
                action="#"
                :auto-upload="false"
                :show-file-list="false"
                :on-change="handleLicenseFile"
                accept=".txt,.license,.lic"
              >
                <el-button plain>上传授权文件</el-button>
              </el-upload>
              <el-button type="primary" :loading="activating" @click="onActivate">激活</el-button>
              <el-button @click="refreshLicense">刷新</el-button>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="告警设置" name="alert">
        <div class="alert-desc">支持企业微信、钉钉、邮件、短信等多种通知方式，可任意组合启用。</div>

        <div v-for="ch in channels" :key="ch.key" class="channel-block">
          <div class="channel-head">
            <div class="ch-title">
              <el-icon :size="18"><component :is="ch.icon" /></el-icon>
              <span>{{ ch.name }}</span>
            </div>
            <el-switch v-model="form[ch.key].enabled" />
          </div>

          <el-form v-if="form[ch.key].enabled" label-width="110px" size="default" class="ch-form">
            <el-form-item v-if="ch.modes" label="机器人类型">
              <el-radio-group v-model="form[ch.key].params.mode">
                <el-radio v-for="m in ch.modes" :key="m.value" :value="m.value">{{ m.label }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <div v-if="activeMode(ch)?.tip" class="mode-tip">{{ activeMode(ch).tip }}</div>
            <el-form-item v-for="f in visibleFields(ch)" :key="f.key" :label="f.label">
              <el-input
                v-if="f.type === 'password'"
                v-model="form[ch.key].params[f.key]"
                :placeholder="f.placeholder"
                show-password
              />
              <el-input
                v-else
                v-model="form[ch.key].params[f.key]"
                :placeholder="f.placeholder"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" plain size="small" @click="testChannel(ch.key)">
                <el-icon><Promotion /></el-icon> 发送测试
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { ChatDotRound, ChatLineRound, Message, Iphone, Promotion } from '@element-plus/icons-vue'
import { alertApi, licenseApi } from '../api'

const props = defineProps({ modelValue: Boolean })
const emit = defineEmits(['update:modelValue'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const activeTab = ref('license')
const saving = ref(false)
const testing = ref('')
const activating = ref(false)
const licenseCode = ref('')

const licenseInfo = reactive({
  edition: 'personal',
  max_servers: 3,
  expire_at: '',
  issued_to: '',
  is_trial: false,
  days_left: -1
})

// 版本代号 → 展示名/标签颜色：企业版（绿）、中小企业板（橙）、社区版（蓝）、个人版（灰蓝）
const editionName = (ed) => {
  if (ed === 'enterprise') return '企业版'
  if (ed === 'sme') return '中小企业板'
  if (ed === 'community') return '社区版（开源）'
  return '个人版'
}
const editionTagType = (ed) => {
  if (ed === 'enterprise') return 'success'
  if (ed === 'sme') return 'warning'
  if (ed === 'community') return 'primary'
  return 'info'
}

const channels = [
  {
    key: 'wecom', name: '企业微信', icon: ChatDotRound,
    fields: [
      { key: 'key', label: 'Webhook Key', type: 'password', placeholder: '机器人 Webhook 末尾的 key' }
    ]
  },
  {
    key: 'dingtalk', name: '钉钉', icon: ChatLineRound,
    modes: [
      {
        value: 'webhook', label: '自定义群机器人',
        tip: '在钉钉群「群设置 - 机器人 - 添加机器人 - 自定义」获取 Webhook，取其中的 access_token。',
        fields: [
          { key: 'token', label: 'Access Token', type: 'password', placeholder: '机器人 Webhook 的 access_token' }
        ]
      },
      {
        value: 'robot', label: '企业应用机器人（RobotCode）',
        tip: '适用于钉钉开放平台创建的企业内部应用机器人，需已开通「企业内机器人发送消息权限」并将机器人添加到群中。',
        fields: [
          { key: 'app_key', label: 'AppKey', placeholder: '应用的 Client ID（AppKey）' },
          { key: 'app_secret', label: 'AppSecret', type: 'password', placeholder: '应用的 Client Secret（AppSecret）' },
          { key: 'robot_code', label: 'RobotCode', placeholder: '机器人编码，如 dingxxxxxxxx' },
          { key: 'conversation_id', label: '群会话ID', placeholder: 'openConversationId，如 cidxxxx' }
        ]
      }
    ]
  },
  {
    key: 'email', name: '邮件', icon: Message,
    fields: [
      { key: 'host', label: 'SMTP 服务器', placeholder: '如 smtp.qq.com' },
      { key: 'port', label: '端口', placeholder: '25 / 465(SSL) / 587' },
      { key: 'user', label: '发件账号', placeholder: '发件邮箱地址' },
      { key: 'pass', label: '授权码/密码', type: 'password', placeholder: '留空表示不修改' },
      { key: 'to', label: '收件人', placeholder: '多个用逗号分隔' }
    ]
  },
  {
    key: 'sms', name: '短信', icon: Iphone,
    fields: [
      { key: 'url', label: '网关地址', placeholder: '短信服务商 HTTP 接口地址' },
      { key: 'mobile', label: '接收手机号', placeholder: '多个用逗号分隔' },
      { key: 'template', label: '模板编号', placeholder: '可选，服务商模板ID' }
    ]
  }
]

const makeChannel = () => ({ enabled: false, params: {} })

const form = reactive({
  wecom: makeChannel(),
  dingtalk: makeChannel(),
  email: makeChannel(),
  sms: makeChannel()
})

async function load() {
  try {
    const { data } = await alertApi.getSetting()
    for (const ch of channels) {
      const cfg = data[ch.key]
      if (cfg) {
        form[ch.key].enabled = !!cfg.enabled
        form[ch.key].params = cfg.params ? { ...cfg.params } : {}
        // 多模式通道：未保存过模式时默认第一个（钉钉默认 Webhook）
        if (ch.modes && !form[ch.key].params.mode) {
          form[ch.key].params.mode = ch.modes[0].value
        }
      }
    }
  } catch (e) {
    // 读取失败不弹窗，保持默认空表单
  }
  refreshLicense()
}

async function refreshLicense() {
  try {
    const { data } = await licenseApi.getInfo()
    Object.assign(licenseInfo, data)
  } catch (e) {
    // 忽略
  }
}

async function onActivate() {
  if (!licenseCode.value.trim()) {
    ElMessage.warning('请输入授权码')
    return
  }
  activating.value = true
  try {
    await licenseApi.activate({ content: licenseCode.value.trim() })
    ElMessage.success('授权激活成功')
    licenseCode.value = ''
    refreshLicense()
  } catch (e) {
    ElMessage.error('激活失败：' + (e?.message || '未知错误'))
  } finally {
    activating.value = false
  }
}

// 上传授权文件自动提取授权码
function handleLicenseFile(file) {
  const reader = new FileReader()
  reader.onload = (e) => {
    const content = e.target.result
    // 提取 LICENSE_CODE_BEGIN 和 LICENSE_CODE_END 之间的内容
    const match = content.match(/LICENSE_CODE_BEGIN\s*\n([\s\S]*?)\nLICENSE_CODE_END/)
    if (match && match[1]) {
      licenseCode.value = match[1].trim()
      ElMessage.success('已从授权文件中提取授权码')
    } else {
      // 如果没有标记，尝试直接使用整个文件内容（兼容旧格式）
      licenseCode.value = content.trim()
      ElMessage.info('未找到授权码标记，已使用文件全部内容')
    }
  }
  reader.readAsText(file.raw)
}

watch(visible, (v) => { if (v) load() })

// 多模式通道：当前选中的模式对象
function activeMode(ch) {
  if (!ch.modes) return null
  const mode = form[ch.key].params.mode || ch.modes[0].value
  return ch.modes.find(m => m.value === mode) || ch.modes[0]
}

// 当前需要渲染的字段列表：多模式通道取选中模式的字段，否则取通道字段
function visibleFields(ch) {
  return activeMode(ch)?.fields || ch.fields || []
}

async function testChannel(key) {
  testing.value = key
  try {
    await alertApi.test({ channel: key, params: form[key].params })
    ElMessage.success('测试消息已发送，请检查对应通道')
  } catch (e) {
    ElMessage.error('发送失败：' + (e?.message || '未知错误'))
  } finally {
    testing.value = ''
  }
}

async function onSave() {
  saving.value = true
  try {
    await alertApi.saveSetting({ ...form })
    ElMessage.success('已保存')
    visible.value = false
  } catch (e) {
    ElMessage.error('保存失败：' + (e?.message || '未知错误'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.license-info { padding: 8px 0; }
.lic-row {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 0; border-bottom: 1px solid var(--border-soft);
}
.lic-row:last-child { border-bottom: none; }
.lic-label {
  width: 120px; font-size: calc(13px * var(--fs, 1)); color: var(--text-2); flex-shrink: 0;
}
.lic-value { font-size: calc(14px * var(--fs, 1)); color: var(--text-1); }
.lic-activate { margin-top: 16px; }
.lic-import-row { margin-bottom: 12px; }
.lic-actions { margin-top: 12px; display: flex; gap: 8px; align-items: center; }

.alert-desc {
  font-size: calc(12.5px * var(--fs, 1)); color: var(--text-2); margin-bottom: 14px;
}
.channel-block {
  background: var(--bg-panel-2); border: 1px solid var(--border-soft); border-radius: 10px;
  padding: 12px 14px; margin-bottom: 12px;
}
.channel-head {
  display: flex; align-items: center; justify-content: space-between;
}
.ch-title {
  display: flex; align-items: center; gap: 8px;
  font-size: calc(14px * var(--fs, 1)); font-weight: 600; color: var(--text-1);
}
.ch-form { margin-top: 12px; margin-bottom: 4px; }
.ch-form :deep(.el-form-item) { margin-bottom: 12px; }
.mode-tip {
  margin: 0 0 12px 2px; padding: 8px 10px;
  font-size: calc(12px * var(--fs, 1)); line-height: 1.6; color: var(--text-2);
  background: var(--hover-overlay); border-left: 3px solid #409eff; border-radius: 2px;
}
</style>
