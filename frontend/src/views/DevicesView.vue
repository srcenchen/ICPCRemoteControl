<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { getDevices, getDevice, deleteDevice, resetDevices, exportDevices } from '@/api'
import { useToast } from '@/composables/useToast'
import { useFormat } from '@/composables/useFormat'
import Modal from '@/components/Modal.vue'
import TerminalModal from '@/components/TerminalModal.vue'
import type { Device } from '@/types'

const store = useAppStore()
const router = useRouter()
const toast = useToast()
const { formatBytes, formatUptime, formatDateTime } = useFormat()

const devices = ref<Device[]>([])
const loading = ref(true)
const selected = ref<Device | null>(null)
const terminalId = ref<number | null>(null)

async function load() {
  try {
    devices.value = await getDevices()
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '加载失败')
  } finally {
    loading.value = false
  }
}
load()

async function showDetail(id: number) {
  try {
    selected.value = await getDevice(id)
  } catch {}
}

async function handleDelete(id: number) {
  if (!confirm(`确定要移除设备 #${id} 吗？`)) return
  try {
    await deleteDevice(id)
    selected.value = null
    await load()
    toast.success('设备已移除')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  }
}

async function handleReset() {
  if (!confirm('这将删除所有设备记录并断开所有连接。\n客户端将以新 ID 重新连接（从 1 开始）。\n\n确定重置？')) return
  try {
    await resetDevices()
    selected.value = null
    await load()
    toast.success('所有设备已重置')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '重置失败')
  }
}

function goToCommand(id: number) {
  selected.value = null
  router.push({ path: '/commands', query: { target: id } })
}

function parseJson<T>(str: string, fallback: T): T {
  try { return JSON.parse(str) } catch { return fallback }
}

function sysInfoRows(d: Device) {
  return [
    ['主机名', d.hostname],
    ['用户名', d.username],
    ['操作系统', d.os_pretty_name],
    ['内核', `${d.kernel_release} (${d.kernel_arch})`],
    ['Shell', d.shell],
    ['终端', d.terminal],
    ['桌面环境', d.de_name],
    ['窗口管理器', d.wm_name],
    ['运行时间', formatUptime(d.uptime)],
    ['签到状态', checkinLabel(d.checkin_status).text],
  ] as [string, string][]
}

function gpuText(d: Device) {
  return parseJson<Array<{vendor:string;name:string}>>(d.gpu_info, []).map(g => `${g.vendor} ${g.name}`).join(', ') || '-'
}
function diskText(d: Device) {
  return parseJson<Array<{mountpoint:string;bytes:{total:number}}>>(d.disk_info, []).map(x => `${x.mountpoint} (${formatBytes(x.bytes?.total)})`).join(', ') || '-'
}
function ipText(d: Device) {
  return parseJson<Array<{name:string;ipv4:string}>>(d.local_ip, []).map(i => `${i.name}: ${i.ipv4}`).join(', ') || '-'
}

function checkinLabel(status: number) {
  if (status === 1) return { text: '已签到', cls: 'text-emerald-700 bg-emerald-100' }
  if (status === 2) return { text: '已签退', cls: 'text-slate-600 bg-slate-100' }
  return { text: '未签到', cls: 'text-red-600 bg-red-100' }
}

function memPct(used: number, total: number) {
  if (!total) return 0
  return Math.round((used / total) * 100)
}

const unsub = store.on('device_connected', load)
const unsub2 = store.on('device_disconnected', load)
const unsub3 = store.on('device_updated', load)
onUnmounted(() => { unsub(); unsub2(); unsub3() })
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-5">
      <h1 class="text-xl font-bold text-slate-900">设备管理</h1>
      <div class="flex gap-2">
        <a
          :href="exportDevices()"
          download
          class="px-3 py-2 text-sm border border-slate-200 rounded-lg hover:bg-slate-50 font-medium text-slate-700 inline-flex items-center gap-1"
        >导出 Excel</a>
        <button
          class="px-3 py-2 text-sm bg-red-500 hover:bg-red-600 text-white rounded-lg font-medium"
          @click="handleReset"
        >↺ 重置所有设备</button>
      </div>
    </div>

    <!-- Table -->
    <div class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="bg-slate-50 text-left border-b border-slate-200">
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">ID</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">主机名</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">用户</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">操作系统</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">CPU</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide min-w-36">内存</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">签到</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">学生</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td colspan="9" class="text-center py-12 text-slate-400">加载中...</td>
            </tr>
            <tr v-else-if="!devices.length">
              <td colspan="9" class="text-center py-12 text-slate-400">暂无已注册设备</td>
            </tr>
            <tr
              v-for="d in devices"
              :key="d.assigned_id"
              class="border-t border-slate-100 hover:bg-blue-50/50 cursor-pointer"
              @click="showDetail(d.assigned_id)"
            >
              <td class="px-4 py-3 font-bold text-blue-600">#{{ d.assigned_id }}</td>
              <td class="px-4 py-3 font-medium">{{ d.hostname }}</td>
              <td class="px-4 py-3 text-slate-600">{{ d.username }}</td>
              <td class="px-4 py-3 text-slate-600 max-w-xs truncate">{{ d.os_name }}</td>
              <td class="px-4 py-3 text-slate-600 max-w-xs truncate text-xs">{{ d.cpu_model }}</td>
              <td class="px-4 py-3">
                <div v-if="d.memory_total" class="w-full bg-slate-100 rounded-full h-4 relative overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="memPct(d.memory_used, d.memory_total) > 90 ? 'bg-red-500' : memPct(d.memory_used, d.memory_total) > 70 ? 'bg-amber-500' : 'bg-blue-500'"
                    :style="{ width: memPct(d.memory_used, d.memory_total) + '%' }"
                  />
                  <span class="absolute inset-0 flex items-center justify-center text-xs font-semibold text-white mix-blend-difference">
                    {{ formatBytes(d.memory_used) }} / {{ formatBytes(d.memory_total) }}
                  </span>
                </div>
                <span v-else class="text-slate-400 text-xs">-</span>
              </td>
              <td class="px-4 py-3">
                <span :class="['inline-flex px-2 py-0.5 rounded-full text-xs font-semibold', checkinLabel(d.checkin_status).cls]">
                  {{ checkinLabel(d.checkin_status).text }}
                </span>
              </td>
              <td class="px-4 py-3 text-sm">
                <span v-if="d.student_name" class="font-medium">{{ d.student_name }}</span>
                <span v-if="d.student_num" class="text-slate-400 text-xs ml-1">{{ d.student_num }}</span>
                <span v-if="!d.student_name" class="text-slate-400">-</span>
              </td>
              <td class="px-4 py-3">
                <span :class="['inline-flex px-2 py-0.5 rounded-full text-xs font-semibold', d.connected ? 'text-emerald-700 bg-emerald-100' : 'text-red-700 bg-red-100']">
                  {{ d.connected ? '在线' : '离线' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Detail Modal -->
    <Modal v-if="selected" :title="`设备 #${selected.assigned_id} 详情`" max-width="720px" @close="selected = null">
      <div class="p-6">
        <div class="flex items-center gap-3 mb-5">
          <span :class="['inline-flex px-2.5 py-1 rounded-full text-xs font-bold', selected.connected ? 'text-emerald-700 bg-emerald-100' : 'text-red-700 bg-red-100']">
            {{ selected.connected ? '在线' : '离线' }}
          </span>
          <span v-if="selected.student_name" class="text-sm text-slate-600">
            {{ selected.student_name }} ({{ selected.student_num }})
          </span>
        </div>

        <section class="mb-5">
          <h3 class="text-xs font-bold uppercase tracking-widest text-blue-600 mb-3">系统信息</h3>
          <div class="grid grid-cols-2 gap-3">
            <div v-for="[label, value] in sysInfoRows(selected)" :key="label" class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">{{ label }}</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ value || '-' }}</div>
            </div>
          </div>
        </section>

        <section class="mb-5">
          <h3 class="text-xs font-bold uppercase tracking-widest text-blue-600 mb-3">硬件信息</h3>
          <div class="grid grid-cols-2 gap-3">
            <div class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">CPU</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ selected.cpu_model }}</div>
            </div>
            <div class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">核心数</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ selected.cpu_physical_cores }} 物理 / {{ selected.cpu_logical_cores }} 逻辑</div>
            </div>
            <div class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">GPU</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ gpuText(selected) }}</div>
            </div>
            <div class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">内存</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ formatBytes(selected.memory_used) }} / {{ formatBytes(selected.memory_total) }}</div>
            </div>
          </div>
        </section>

        <section class="mb-5">
          <h3 class="text-xs font-bold uppercase tracking-widest text-blue-600 mb-3">存储与网络</h3>
          <div class="grid grid-cols-2 gap-3">
            <div class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">磁盘</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ diskText(selected) }}</div>
            </div>
            <div class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">网络 IP</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ ipText(selected) }}</div>
            </div>
            <div class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">首次上线</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ formatDateTime(selected.first_seen) }}</div>
            </div>
            <div class="bg-slate-50 rounded-lg px-3 py-2">
              <div class="text-xs text-slate-500 uppercase tracking-wide">最后在线</div>
              <div class="text-sm font-medium text-slate-800 mt-0.5">{{ formatDateTime(selected.last_seen) }}</div>
            </div>
          </div>
        </section>
      </div>

      <template #footer>
        <div class="flex gap-2">
          <button
            class="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium"
            @click="goToCommand(selected!.assigned_id)"
          >在此设备执行命令</button>
          <button
            class="px-4 py-2 text-sm bg-slate-700 hover:bg-slate-800 text-white rounded-lg font-medium"
            @click="terminalId = selected!.assigned_id; selected = null"
          >🖥 终端</button>
          <button
            class="px-4 py-2 text-sm bg-red-500 hover:bg-red-600 text-white rounded-lg font-medium ml-auto"
            @click="handleDelete(selected!.assigned_id)"
          >移除设备</button>
        </div>
      </template>
    </Modal>

    <!-- Terminal Modal -->
    <TerminalModal v-if="terminalId" :device-id="terminalId" @close="terminalId = null" />
  </div>
</template>
