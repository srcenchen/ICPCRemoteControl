<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { getDevices, executeCommand, getCommands, getCommand, cancelCommand, clearCommands, getPresets } from '@/api'
import { useToast } from '@/composables/useToast'
import { useFormat } from '@/composables/useFormat'
import DeviceSelector from '@/components/DeviceSelector.vue'
import CodeEditor from '@/components/CodeEditor.vue'
import type { Device, CommandLog, Preset } from '@/types'

const store = useAppStore()
const route = useRoute()
const toast = useToast()
const { statusLabel, statusColor } = useFormat()

const devices = ref<Device[]>([])
const selectedTargets = ref<number[]>([])
const cmdValue = ref('echo "Hello from ICPC!"')
const presets = ref<Preset[]>([])
const history = ref<CommandLog[]>([])
const historyLoading = ref(false)

// Result table
interface ResultRow {
  deviceId: number
  cmdId?: number
  status: string
  output: string
  durationMs?: number
}
const resultRows = ref<Map<string, ResultRow>>(new Map())
const activeKey = ref<string | null>(null)
const isBatchRunning = ref(false)
const activeCmdIds = new Set<number>()
const sessionTotal = ref(0)
const sessionDone = ref(0)

// History detail
const histDetailCmdId = ref<number | null>(null)
const histDetail = ref<CommandLog | null>(null)
const histDetailKey = ref<string | null>(null)

async function load() {
  devices.value = await getDevices()
  const targetParam = route.query.target
  if (targetParam) {
    selectedTargets.value = [Number(targetParam)]
  }
}
load()

async function loadPresets() {
  try { presets.value = await getPresets() } catch {}
}
loadPresets()

async function loadHistory() {
  historyLoading.value = true
  try { history.value = await getCommands(30) } catch {}
  historyLoading.value = false
}
loadHistory()

const presetColorMap: Record<string, string> = {
  primary: 'bg-blue-600 hover:bg-blue-700',
  success: 'bg-emerald-600 hover:bg-emerald-700',
  warning: 'bg-amber-500 hover:bg-amber-600',
  danger: 'bg-red-500 hover:bg-red-600',
  info: 'bg-cyan-500 hover:bg-cyan-600',
  dark: 'bg-slate-700 hover:bg-slate-800',
}

async function execute() {
  const cmd = cmdValue.value.trim()
  if (!cmd) { toast.error('请输入命令'); return }

  resultRows.value = new Map()
  activeKey.value = null
  activeCmdIds.clear()
  isBatchRunning.value = true
  sessionDone.value = 0

  if (selectedTargets.value.length === 0) {
    // Broadcast
    const online = devices.value.filter(d => d.connected)
    sessionTotal.value = online.length || 1
    try {
      const res = await executeCommand({ target_type: 'broadcast', command: cmd })
      activeCmdIds.add(res.id)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : '执行失败')
      isBatchRunning.value = false
    }
  } else {
    sessionTotal.value = selectedTargets.value.length
    for (const id of selectedTargets.value) {
      try {
        const res = await executeCommand({ target_type: 'single', target_id: id, command: cmd })
        activeCmdIds.add(res.id)
      } catch (e: unknown) {
        toast.error(`设备 #${id}: ${e instanceof Error ? e.message : '失败'}`)
      }
    }
  }

  await loadHistory()
}

function getOrCreateRow(deviceId: number, cmdId?: number): ResultRow {
  const key = `dev_${deviceId}`
  if (!resultRows.value.has(key)) {
    resultRows.value.set(key, { deviceId, cmdId, status: 'running', output: '' })
  }
  return resultRows.value.get(key)!
}

function handleCommandOutput(data: { command_id: number; device_id: number; line: string }) {
  if (!activeCmdIds.has(data.command_id)) return
  const row = getOrCreateRow(data.device_id, data.command_id)
  row.output += data.line + '\n'
  if (activeKey.value === `dev_${data.device_id}`) {
    // force reactivity
    resultRows.value = new Map(resultRows.value)
  }
}

function handleCommandResult(data: { command_id: number; device_id: number; status: string; error_output: string; duration_ms: number }) {
  if (!activeCmdIds.has(data.command_id)) return
  const row = getOrCreateRow(data.device_id, data.command_id)
  row.status = data.status
  row.durationMs = data.duration_ms
  if (data.error_output) row.output += data.error_output + '\n'
  sessionDone.value++
  resultRows.value = new Map(resultRows.value)
  if (sessionDone.value >= sessionTotal.value) {
    isBatchRunning.value = false
    loadHistory()
  }
}

async function showHistDetail(cmdId: number) {
  histDetailCmdId.value = cmdId
  histDetail.value = null
  histDetailKey.value = null
  try {
    histDetail.value = await getCommand(cmdId)
    if (histDetail.value?.command) cmdValue.value = histDetail.value.command
  } catch {}
}

async function cancelCmd(cmdId: number) {
  try {
    await cancelCommand(cmdId)
    toast.success('已取消')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '取消失败')
  }
}

async function clearHistory() {
  if (!confirm('确定清空所有命令历史？')) return
  try {
    await clearCommands()
    history.value = []
    resultRows.value = new Map()
    toast.success('历史已清空')
  } catch {}
}

// WebSocket events
const unsubOutput = store.on('command_output', (data) => handleCommandOutput(data as Parameters<typeof handleCommandOutput>[0]))
const unsubResult = store.on('command_result', (data) => handleCommandResult(data as Parameters<typeof handleCommandResult>[0]))
onUnmounted(() => { unsubOutput(); unsubResult() })
</script>

<template>
  <div>
    <h1 class="text-xl font-bold text-slate-900 mb-5">命令执行</h1>

    <!-- Main layout -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-5 mb-6">
      <!-- Left: Command Panel -->
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 flex flex-col gap-4">
        <div>
          <div class="text-sm font-semibold text-slate-700 mb-2">目标设备</div>
          <DeviceSelector v-model="selectedTargets" :devices="devices" />
        </div>

        <div>
          <div class="text-sm font-semibold text-slate-700 mb-2">命令预设</div>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="p in presets"
              :key="p.name"
              :class="['px-3 py-1.5 text-xs font-semibold text-white rounded-lg', presetColorMap[p.color] || 'bg-blue-600 hover:bg-blue-700']"
              :title="p.desc"
              @click="cmdValue = p.command"
            >{{ p.name }}</button>
          </div>
        </div>

        <div>
          <div class="text-sm font-semibold text-slate-700 mb-2">命令</div>
          <CodeEditor v-model="cmdValue" height="200px" />
        </div>

        <button
          class="w-full py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-semibold text-sm flex items-center justify-center gap-2"
          @click="execute"
        >
          ▶ 执行命令
        </button>
      </div>

      <!-- Right: Result Panel -->
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 flex flex-col">
        <div class="flex items-center justify-between mb-3">
          <div class="text-sm font-semibold text-slate-700">执行结果</div>
          <div v-if="isBatchRunning" class="text-xs text-slate-500">
            {{ sessionDone }}/{{ sessionTotal }} 完成
          </div>
        </div>

        <!-- Batch result rows -->
        <div v-if="resultRows.size > 0" class="flex-1 flex flex-col min-h-0">
          <div class="overflow-y-auto border border-slate-100 rounded-lg">
            <table class="w-full text-sm">
              <thead class="sticky top-0 bg-slate-50 border-b border-slate-200">
                <tr>
                  <th class="px-3 py-2 text-left text-xs font-semibold text-slate-500">设备</th>
                  <th class="px-3 py-2 text-left text-xs font-semibold text-slate-500">状态</th>
                  <th class="px-3 py-2 text-left text-xs font-semibold text-slate-500">输出摘要</th>
                  <th class="px-3 py-2 text-left text-xs font-semibold text-slate-500">耗时</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="[key, row] in resultRows"
                  :key="key"
                  class="border-t border-slate-100 cursor-pointer"
                  :class="activeKey === key ? 'bg-blue-50' : 'hover:bg-slate-50'"
                  @click="activeKey = activeKey === key ? null : key"
                >
                  <td class="px-3 py-2 font-bold text-blue-600">#{{ row.deviceId }}</td>
                  <td class="px-3 py-2">
                    <span :class="['inline-flex px-2 py-0.5 rounded-full text-xs font-semibold', statusColor(row.status)]">
                      {{ statusLabel(row.status) }}
                    </span>
                  </td>
                  <td class="px-3 py-2 text-xs text-slate-600 max-w-xs truncate font-mono">
                    {{ row.output.replace(/\n/g, ' ').substring(0, 80) }}
                  </td>
                  <td class="px-3 py-2 text-xs text-slate-400">{{ row.durationMs ? row.durationMs + 'ms' : '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Detail output -->
          <Transition name="slide">
            <div v-if="activeKey && resultRows.get(activeKey)" class="mt-3">
              <div class="flex items-center justify-between mb-1">
                <span class="text-xs font-semibold text-blue-600">设备 #{{ resultRows.get(activeKey)!.deviceId }} 输出</span>
                <div class="flex gap-1">
                  <button
                    v-if="resultRows.get(activeKey)!.status === 'running' && resultRows.get(activeKey)!.cmdId"
                    class="px-2 py-1 text-xs bg-red-500 text-white rounded"
                    @click="cancelCmd(resultRows.get(activeKey)!.cmdId!)"
                  >⏹ 终止</button>
                  <button class="px-2 py-1 text-xs border border-slate-200 rounded text-slate-600" @click="activeKey = null">✕</button>
                </div>
              </div>
              <pre class="bg-slate-900 text-slate-100 rounded-lg p-3 text-xs font-mono max-h-48 overflow-auto whitespace-pre-wrap break-all">{{ resultRows.get(activeKey)!.output || '(等待输出...)' }}</pre>
            </div>
          </Transition>
        </div>

        <!-- History detail -->
        <div v-else-if="histDetail">
          <div class="flex items-center gap-2 mb-2">
            <span :class="['inline-flex px-2 py-0.5 rounded-full text-xs font-semibold', statusColor(histDetail.status)]">
              {{ statusLabel(histDetail.status) }}
            </span>
            <span v-if="histDetail.duration_ms" class="text-xs text-slate-400">{{ histDetail.duration_ms }}ms</span>
            <button
              v-if="['dispatched','running','pending'].includes(histDetail.status)"
              class="ml-auto px-2 py-1 text-xs bg-red-500 text-white rounded"
              @click="cancelCmd(histDetail.id)"
            >⏹ 终止</button>
          </div>
          <div v-if="histDetail.children?.length">
            <div class="text-xs text-slate-500 mb-1">{{ histDetail.children.length }} 台设备</div>
            <div class="overflow-y-auto max-h-64 border border-slate-100 rounded-lg">
              <table class="w-full text-xs">
                <thead class="sticky top-0 bg-slate-50 border-b border-slate-100">
                  <tr>
                    <th class="px-3 py-2 text-left font-semibold text-slate-500">设备</th>
                    <th class="px-3 py-2 text-left font-semibold text-slate-500">状态</th>
                    <th class="px-3 py-2 text-left font-semibold text-slate-500">输出</th>
                    <th class="px-3 py-2 text-left font-semibold text-slate-500">耗时</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="child in histDetail.children"
                    :key="child.id"
                    class="border-t border-slate-100 cursor-pointer"
                    :class="histDetailKey === String(child.target_id) ? 'bg-blue-50' : 'hover:bg-slate-50'"
                    @click="histDetailKey = histDetailKey === String(child.target_id) ? null : String(child.target_id)"
                  >
                    <td class="px-3 py-2 font-bold text-blue-600">#{{ child.target_id }}</td>
                    <td class="px-3 py-2">
                      <span :class="['inline-flex px-1.5 py-0.5 rounded-full text-xs font-semibold', statusColor(child.status)]">{{ statusLabel(child.status) }}</span>
                    </td>
                    <td class="px-3 py-2 text-slate-600 font-mono truncate max-w-xs">
                      {{ (child.output || child.error_output || '').replace(/\n/g,' ').substring(0,60) }}
                    </td>
                    <td class="px-3 py-2 text-slate-400">{{ child.duration_ms ? child.duration_ms + 'ms' : '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-if="histDetailKey">
              <pre class="bg-slate-900 text-slate-100 rounded-lg p-3 text-xs font-mono max-h-36 overflow-auto whitespace-pre-wrap mt-2">{{ histDetail.children.find(c => String(c.target_id) === histDetailKey)?.output || '(无输出)' }}</pre>
            </div>
          </div>
          <div v-else>
            <pre class="bg-slate-900 text-slate-100 rounded-lg p-3 text-xs font-mono max-h-64 overflow-auto whitespace-pre-wrap">{{ histDetail.output || histDetail.error_output || '(无输出)' }}</pre>
          </div>
        </div>

        <div v-else class="flex-1 flex items-center justify-center text-slate-400 text-sm py-16">
          选择目标设备并执行，或从历史记录中选择查看
        </div>
      </div>
    </div>

    <!-- Command History -->
    <div class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
      <div class="flex items-center justify-between px-5 py-4 border-b border-slate-100">
        <h2 class="font-semibold text-slate-800">命令历史</h2>
        <button class="px-3 py-1.5 text-xs bg-red-500 hover:bg-red-600 text-white rounded-lg font-medium" @click="clearHistory">清空历史</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="bg-slate-50 border-b border-slate-200 text-left">
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">ID</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">时间</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">目标</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">命令</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">状态</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">耗时</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!history.length">
              <td colspan="6" class="text-center py-10 text-slate-400">暂无命令记录</td>
            </tr>
            <tr
              v-for="cmd in history"
              :key="cmd.id"
              class="border-t border-slate-100 cursor-pointer hover:bg-blue-50/50"
              :class="histDetailCmdId === cmd.id ? 'bg-blue-50' : ''"
              @click="showHistDetail(cmd.id)"
            >
              <td class="px-4 py-3 text-slate-500">#{{ cmd.id }}</td>
              <td class="px-4 py-3 text-xs text-slate-500 whitespace-nowrap">{{ cmd.created_at }}</td>
              <td class="px-4 py-3 font-medium">{{ cmd.target_type === 'broadcast' ? '全部' : '#' + cmd.target_id }}</td>
              <td class="px-4 py-3 font-mono text-xs text-slate-700 max-w-xs truncate">{{ cmd.command.substring(0, 50) }}{{ cmd.command.length > 50 ? '...' : '' }}</td>
              <td class="px-4 py-3">
                <span :class="['inline-flex px-2 py-0.5 rounded-full text-xs font-semibold', statusColor(cmd.status)]">{{ statusLabel(cmd.status) }}</span>
              </td>
              <td class="px-4 py-3 text-xs text-slate-500">{{ cmd.duration_ms ? cmd.duration_ms + 'ms' : '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.slide-enter-active, .slide-leave-active { transition: all 0.2s ease; }
.slide-enter-from, .slide-leave-to { opacity: 0; transform: translateY(-8px); }
</style>
