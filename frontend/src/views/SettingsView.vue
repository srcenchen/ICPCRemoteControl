<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getSettings, updateSettings, updateCheckinConfig, updateSettingsPresets, changePassword } from '@/api'
import { useToast } from '@/composables/useToast'
import type { Settings, Preset, CheckinConfig } from '@/types'

const toast = useToast()

const settings = ref<Settings | null>(null)
const prefix = ref('')
const prefixPreview = computed(() => `${prefix.value || '?'}-1`)
const checkinCfg = ref<CheckinConfig>({
  welcome_text: '',
  warning_text: '',
  post_checkin_msg: '',
  post_checkout_cmd: '',
  post_checkout_msg: '',
})
const presets = ref<Preset[]>([])
const oldPwd = ref('')
const newPwd = ref('')

async function load() {
  try {
    const s = await getSettings()
    settings.value = s
    prefix.value = s.hostname_prefix
    const cfg = s.checkin_config || {}
    checkinCfg.value = {
      welcome_text: cfg.welcome_text || '',
      warning_text: cfg.warning_text || '',
      post_checkin_msg: cfg.post_checkin_msg || '',
      post_checkout_cmd: cfg.post_checkout_cmd || '',
      post_checkout_msg: cfg.post_checkout_msg || '',
    }
    presets.value = s.presets || []
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '加载失败')
  }
}
onMounted(load)

async function savePrefix() {
  if (!prefix.value.trim()) { toast.error('前缀不能为空'); return }
  try {
    const res = await updateSettings({ hostname_prefix: prefix.value.trim() })
    prefix.value = res.hostname_prefix
    toast.success('主机名前缀已保存')
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '保存失败') }
}

const defaultCheckin: CheckinConfig = {
  welcome_text: '欢迎参加XCPC竞赛',
  warning_text: '严禁场外答题，否则成绩无效！',
  post_checkin_msg: '签到成功',
  post_checkout_cmd: 'shutdown -h +1',
  post_checkout_msg: '签退成功，您的电脑将在1分钟后自动关机。',
}

function resetCheckinDefaults() {
  Object.assign(checkinCfg.value, defaultCheckin)
  toast.info('已恢复默认值，请点击保存')
}

async function saveCheckinCfg() {
  try {
    await updateCheckinConfig(checkinCfg.value)
    toast.success('签到配置已保存')
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '保存失败') }
}

const COLOR_OPTIONS = [
  { value: 'primary', label: '蓝' },
  { value: 'success', label: '绿' },
  { value: 'warning', label: '橙' },
  { value: 'danger', label: '红' },
  { value: 'info', label: '青' },
  { value: 'dark', label: '灰' },
]

function addPreset() {
  presets.value.push({ name: '', desc: '', command: '', color: 'primary' })
}

function removePreset(idx: number) {
  presets.value.splice(idx, 1)
}

function movePreset(idx: number, dir: -1 | 1) {
  const target = idx + dir
  if (target < 0 || target >= presets.value.length) return
  ;[presets.value[idx], presets.value[target]] = [presets.value[target], presets.value[idx]]
}

async function savePresets() {
  for (const p of presets.value) {
    if (!p.name) { toast.error('命令名称不能为空'); return }
    if (!p.command) { toast.error('命令内容不能为空'); return }
  }
  try {
    presets.value = await updateSettingsPresets(presets.value)
    toast.success('预设已保存')
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '保存失败') }
}

async function savePwd() {
  if (!oldPwd.value || !newPwd.value) { toast.error('旧密码和新密码不能为空'); return }
  try {
    await changePassword(oldPwd.value, newPwd.value)
    oldPwd.value = ''
    newPwd.value = ''
    toast.success('密码修改成功')
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '修改失败') }
}

const colorClass: Record<string, string> = {
  primary: 'bg-blue-600', success: 'bg-emerald-600',
  warning: 'bg-amber-500', danger: 'bg-red-500', info: 'bg-cyan-500', dark: 'bg-slate-700',
}
</script>

<template>
  <div class="max-w-3xl">
    <h1 class="text-xl font-bold text-slate-900 mb-5">系统设置</h1>

    <div class="space-y-5">
      <!-- Hostname prefix -->
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
        <h2 class="font-semibold text-slate-800 mb-1">选手机主机名前缀</h2>
        <p class="text-xs text-slate-500 mb-3">客户端注册后会被重命名为 <code class="bg-slate-100 px-1 rounded">前缀-编号</code> 格式。</p>
        <div class="flex gap-2">
          <input v-model="prefix" type="text" placeholder="cwxu-icpc" maxlength="64" class="flex-1 px-3 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30" @keydown.enter="savePrefix" />
          <button class="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-semibold" @click="savePrefix">保存</button>
        </div>
        <p class="text-xs text-slate-400 mt-2">预览：<strong class="text-blue-600 font-mono">{{ prefixPreview }}</strong></p>
      </div>

      <!-- Checkin config -->
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
        <h2 class="font-semibold text-slate-800 mb-1">签到页面配置</h2>
        <p class="text-xs text-slate-500 mb-4">配置选手端签到页（<code class="bg-slate-100 px-1 rounded">:8090</code>）的显示内容和行为。</p>
        <div class="space-y-4">
          <div class="bg-slate-50 rounded-lg p-3">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="px-1.5 py-0.5 text-xs font-bold text-blue-700 bg-blue-100 rounded">签到前</span>
              <span class="text-sm font-medium text-slate-700">欢迎语</span>
            </div>
            <textarea v-model="checkinCfg.welcome_text" rows="2" class="w-full px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30 resize-y" placeholder="欢迎参加XCPC竞赛" />
          </div>
          <div class="bg-slate-50 rounded-lg p-3">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="px-1.5 py-0.5 text-xs font-bold text-red-700 bg-red-100 rounded">签到前</span>
              <span class="text-sm font-medium text-slate-700">警告提示</span>
            </div>
            <textarea v-model="checkinCfg.warning_text" rows="2" class="w-full px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30 resize-y" />
          </div>
          <div class="bg-slate-50 rounded-lg p-3">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="px-1.5 py-0.5 text-xs font-bold text-emerald-700 bg-emerald-100 rounded">签到后</span>
              <span class="text-sm font-medium text-slate-700">成功提示语</span>
            </div>
            <input v-model="checkinCfg.post_checkin_msg" type="text" class="w-full px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30" />
          </div>
          <div class="bg-slate-50 rounded-lg p-3">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="px-1.5 py-0.5 text-xs font-bold text-amber-700 bg-amber-100 rounded">签退后</span>
              <span class="text-sm font-medium text-slate-700">执行命令</span>
            </div>
            <p class="text-xs text-slate-400 mb-1.5">建议: <code class="bg-white px-1 rounded border border-slate-200">shutdown -h +1</code>（1分钟后关机）</p>
            <textarea v-model="checkinCfg.post_checkout_cmd" rows="2" class="w-full px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30 font-mono resize-y" />
          </div>
          <div class="bg-slate-50 rounded-lg p-3">
            <div class="flex items-center gap-2 mb-1.5">
              <span class="px-1.5 py-0.5 text-xs font-bold text-amber-700 bg-amber-100 rounded">签退后</span>
              <span class="text-sm font-medium text-slate-700">提示语</span>
            </div>
            <textarea v-model="checkinCfg.post_checkout_msg" rows="2" class="w-full px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30 resize-y" />
          </div>
          <div class="flex gap-2">
            <button class="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-semibold" @click="saveCheckinCfg">保存签到配置</button>
            <button class="px-4 py-2.5 border border-slate-200 hover:bg-slate-50 text-slate-700 rounded-lg text-sm font-medium" @click="resetCheckinDefaults">恢复默认</button>
          </div>
        </div>
      </div>

      <!-- Presets -->
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
        <h2 class="font-semibold text-slate-800 mb-1">预设命令管理</h2>
        <p class="text-xs text-slate-500 mb-4">在"命令执行"页面显示的快捷命令按钮。</p>

        <div v-if="!presets.length" class="text-center py-8 text-slate-400 text-sm">暂无预设，点击"添加"创建</div>
        <div v-else class="space-y-2 mb-3">
          <div class="hidden sm:grid grid-cols-[1fr_1fr_2fr_80px_100px] gap-2 px-2 py-1 text-xs font-semibold text-slate-500 uppercase tracking-wide bg-slate-50 rounded-lg">
            <span>名称</span><span>描述</span><span>命令</span><span>颜色</span><span class="text-center">操作</span>
          </div>
          <div v-for="(p, idx) in presets" :key="idx" class="grid sm:grid-cols-[1fr_1fr_2fr_80px_100px] grid-cols-1 gap-2 items-center p-2 rounded-lg border border-slate-200">
            <input v-model="p.name" type="text" placeholder="名称" class="px-2 py-1.5 text-sm border border-slate-200 rounded focus:outline-none focus:ring-1 focus:ring-blue-500" />
            <input v-model="p.desc" type="text" placeholder="描述" class="px-2 py-1.5 text-sm border border-slate-200 rounded focus:outline-none focus:ring-1 focus:ring-blue-500" />
            <input v-model="p.command" type="text" placeholder="shell 命令" class="px-2 py-1.5 text-sm border border-slate-200 rounded focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono" />
            <select v-model="p.color" class="px-2 py-1.5 text-sm border border-slate-200 rounded focus:outline-none focus:ring-1 focus:ring-blue-500 bg-white">
              <option v-for="c in COLOR_OPTIONS" :key="c.value" :value="c.value">{{ c.label }}</option>
            </select>
            <div class="flex gap-1 justify-end">
              <button class="px-2 py-1.5 text-xs border border-slate-200 rounded hover:bg-slate-50" @click="movePreset(idx, -1)" :disabled="idx === 0">▲</button>
              <button class="px-2 py-1.5 text-xs border border-slate-200 rounded hover:bg-slate-50" @click="movePreset(idx, 1)" :disabled="idx === presets.length - 1">▼</button>
              <button class="px-2 py-1.5 text-xs border border-red-200 text-red-500 rounded hover:bg-red-50" @click="removePreset(idx)">×</button>
            </div>
          </div>
        </div>

        <div class="flex gap-2">
          <button class="px-4 py-2.5 border border-dashed border-slate-300 hover:border-blue-400 hover:text-blue-600 text-slate-600 rounded-lg text-sm font-medium flex-1 sm:flex-none" @click="addPreset">+ 添加命令</button>
          <button class="px-4 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-semibold" @click="savePresets">保存预设</button>
        </div>

        <!-- Color preview -->
        <div v-if="presets.length" class="mt-4 flex flex-wrap gap-2">
          <button v-for="p in presets" :key="p.name" :class="['px-3 py-1.5 text-xs font-semibold text-white rounded-lg', colorClass[p.color] || 'bg-blue-600']" :title="p.desc">{{ p.name || '未命名' }}</button>
        </div>
      </div>

      <!-- Password -->
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
        <h2 class="font-semibold text-slate-800 mb-1">修改管理员密码</h2>
        <div class="max-w-xs space-y-3 mt-3">
          <div>
            <label class="block text-sm font-medium text-slate-700 mb-1.5">旧密码</label>
            <input v-model="oldPwd" type="password" class="w-full px-3 py-2.5 border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30" />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-700 mb-1.5">新密码</label>
            <input v-model="newPwd" type="password" class="w-full px-3 py-2.5 border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30" @keydown.enter="savePwd" />
          </div>
          <button class="w-full py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-semibold" @click="savePwd">修改密码</button>
        </div>
      </div>
    </div>
  </div>
</template>
