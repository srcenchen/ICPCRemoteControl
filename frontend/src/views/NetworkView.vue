<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getDevices, getNetworkRules, updateNetworkRules, applyNetwork, removeNetwork } from '@/api'
import { useToast } from '@/composables/useToast'
import DeviceSelector from '@/components/DeviceSelector.vue'
import type { Device, NetworkRule } from '@/types'

const toast = useToast()

const devices = ref<Device[]>([])
const selectedTargets = ref<number[]>([])
const rules = ref<NetworkRule[]>([])
const logs = ref<string[]>([])

const RULE_TYPES = [
  { value: 'DOMAIN-SUFFIX', label: '域名后缀' },
  { value: 'DOMAIN-KEYWORD', label: '域名关键字' },
  { value: 'DOMAIN', label: '完整域名' },
]

async function load() {
  const [d, r] = await Promise.all([getDevices(), getNetworkRules()])
  devices.value = d
  rules.value = r
}
load()

function addRule() {
  rules.value.push({ type: 'DOMAIN-SUFFIX', value: '' })
}

function deleteRule(idx: number) {
  rules.value.splice(idx, 1)
  saveRules()
}

async function saveRules() {
  try {
    await updateNetworkRules(rules.value.filter(r => r.value.trim()))
    toast.success('规则已保存')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '保存失败')
  }
}

async function doAction(type: 'apply' | 'remove') {
  const label = type === 'apply' ? '应用网络限制' : '解除网络限制'
  if (!confirm(`确定${label}？`)) return

  const isBroadcast = selectedTargets.value.length === 0
  const fn = type === 'apply' ? applyNetwork : removeNetwork

  if (!isBroadcast && selectedTargets.value.length > 1) {
    const results: string[] = []
    for (const id of selectedTargets.value) {
      try {
        const res = await fn({ target_type: 'single', target_id: id })
        results.push(`[设备 #${id}] 已派发 (ID:${res.id})`)
      } catch (e: unknown) {
        results.push(`[设备 #${id}] 失败: ${e instanceof Error ? e.message : '未知'}`)
      }
    }
    logs.value = results
    toast.success(`${label} 派发完成`)
  } else {
    try {
      const body = isBroadcast
        ? { target_type: 'broadcast' }
        : { target_type: 'single', target_id: selectedTargets.value[0] }
      const res = await fn(body)
      logs.value = [`${label} 已派发 (命令ID: ${res.id})\n请在"命令执行"页面查看结果。`]
      toast.success(`${label} 已派发`)
    } catch (e: unknown) {
      logs.value = [`${label} 失败: ${e instanceof Error ? e.message : '未知'}`]
      toast.error(e instanceof Error ? e.message : '操作失败')
    }
  }
}
</script>

<template>
  <div>
    <h1 class="text-xl font-bold text-slate-900 mb-5">网络屏蔽管理</h1>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-5">
      <!-- Rules -->
      <div class="lg:col-span-2 space-y-4">
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
          <h2 class="font-semibold text-slate-800 mb-1">白名单规则</h2>
          <p class="text-xs text-slate-500 mb-4">只有匹配以下规则的域名才能访问外网，其余全部拦截。</p>

          <div v-if="!rules.length" class="text-center py-8 text-slate-400 text-sm">
            暂无规则，点击"添加规则"创建
          </div>

          <div v-else class="space-y-2 mb-3">
            <div class="grid grid-cols-[120px_1fr_auto] gap-2 items-center px-2 py-1 text-xs font-semibold text-slate-500 uppercase tracking-wide bg-slate-50 rounded-lg">
              <span>类型</span><span>值</span><span></span>
            </div>
            <div
              v-for="(rule, idx) in rules"
              :key="idx"
              class="grid grid-cols-[120px_1fr_auto] gap-2 items-center"
            >
              <select
                v-model="rule.type"
                class="px-2 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                @change="saveRules"
              >
                <option v-for="t in RULE_TYPES" :key="t.value" :value="t.value">{{ t.label }}</option>
              </select>
              <input
                v-model="rule.value"
                type="text"
                placeholder="如: baidu.com"
                class="px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                @blur="saveRules"
              />
              <button class="w-8 h-8 text-slate-400 hover:text-red-500 hover:bg-red-50 rounded-lg flex items-center justify-center text-lg" @click="deleteRule(idx)">×</button>
            </div>
          </div>

          <button class="px-4 py-2 text-sm border border-dashed border-slate-300 text-slate-600 rounded-lg hover:border-blue-400 hover:text-blue-600 w-full" @click="addRule">+ 添加规则</button>
        </div>

        <!-- Log area -->
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
          <h2 class="font-semibold text-slate-800 mb-3">执行日志</h2>
          <pre class="bg-slate-900 text-slate-300 rounded-lg p-4 text-xs font-mono min-h-24 max-h-48 overflow-auto whitespace-pre-wrap">{{ logs.join('\n') || '等待操作...' }}</pre>
        </div>
      </div>

      <!-- Right: Device selector + Actions -->
      <div class="space-y-4">
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
          <h2 class="font-semibold text-slate-800 mb-3">目标设备</h2>
          <DeviceSelector v-model="selectedTargets" :devices="devices" />
        </div>

        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 space-y-3">
          <h2 class="font-semibold text-slate-800 mb-1">操作</h2>
          <button
            class="w-full py-2.5 bg-red-500 hover:bg-red-600 text-white rounded-lg font-semibold text-sm"
            @click="doAction('apply')"
          >🚫 应用网络限制</button>
          <button
            class="w-full py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white rounded-lg font-semibold text-sm"
            @click="doAction('remove')"
          >✅ 解除网络限制</button>
        </div>
      </div>
    </div>
  </div>
</template>
