<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useAppStore } from '@/stores/app'
import {
  getDevices, getDistributionStatus, getDistributionFiles,
  uploadDistributionFile, deleteDistributionFiles, clearDistributionFiles,
  startDistribution, stopDistribution, retryDeviceDistribution,
  precheckDistribution, resetDistributionTask,
} from '@/api'
import { useToast } from '@/composables/useToast'
import { useFormat } from '@/composables/useFormat'
import DeviceSelector from '@/components/DeviceSelector.vue'
import CodeEditor from '@/components/CodeEditor.vue'
import type { Device, DistributeFile, DistributeTask } from '@/types'

const store = useAppStore()
const toast = useToast()
const { formatBytes } = useFormat()

const devices = ref<Device[]>([])
const selectedTargets = ref<number[]>([])
const files = ref<DistributeFile[]>([])
const task = ref<DistributeTask | null>(null)
const selectedFiles = ref<Set<string>>(new Set())
const currentHostname = location.hostname
const saveDir = ref('/home/cwxu/Downloads')
const serverIP = ref(localStorage.getItem('distribute_server_ip') || location.hostname)
const postCmd = ref(localStorage.getItem('distribute_post_cmd') || '')
const uploadPct = ref(0)
const uploading = ref(false)
const precheckResults = ref<Array<{ device_id: number; success: boolean; error: string }>>([])
const precheckLoading = ref(false)

const isActiveTask = computed(() => task.value && ['running', 'completed', 'stopped'].includes(task.value.status))

async function load() {
  const [s, f, d] = await Promise.all([getDistributionStatus(), getDistributionFiles(), getDevices()])
  task.value = s
  files.value = f || []
  devices.value = d
}
load()

// Upload
const fileInputRef = ref<HTMLInputElement>()

async function handleUpload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  uploading.value = true
  uploadPct.value = 0
  try {
    await uploadDistributionFile(file, pct => { uploadPct.value = pct })
    await load()
    toast.success('上传成功')
  } catch (err: unknown) {
    toast.error(err instanceof Error ? err.message : '上传失败')
  } finally {
    uploading.value = false
    if (fileInputRef.value) fileInputRef.value.value = ''
  }
}

function toggleFile(name: string) {
  if (selectedFiles.value.has(name)) selectedFiles.value.delete(name)
  else selectedFiles.value.add(name)
}

function selectAllFiles(val: boolean) {
  selectedFiles.value = val ? new Set(files.value.map(f => f.name)) : new Set()
}

async function deleteSelected() {
  if (!selectedFiles.value.size) { toast.error('请先选择文件'); return }
  if (!confirm(`确定从服务器删除 ${selectedFiles.value.size} 个文件？`)) return
  try {
    await deleteDistributionFiles([...selectedFiles.value])
    selectedFiles.value.clear()
    await load()
    toast.success('已删除')
  } catch {}
}

async function clearAll() {
  if (!confirm('⚠️ 确定清空服务器上所有分发文件？')) return
  try {
    await clearDistributionFiles()
    selectedFiles.value.clear()
    await load()
    toast.success('已清空')
  } catch {}
}

async function precheck() {
  precheckLoading.value = true
  precheckResults.value = []
  localStorage.setItem('distribute_server_ip', serverIP.value)
  try {
    precheckResults.value = await precheckDistribution({ server_ip: serverIP.value, target_ids: selectedTargets.value })
    toast.info('连接测试完成')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '测试失败')
  } finally {
    precheckLoading.value = false
  }
}

async function startTask() {
  if (!selectedFiles.value.size) { toast.error('请选择要分发的文件'); return }
  if (!saveDir.value) { toast.error('请输入目标保存目录'); return }
  localStorage.setItem('distribute_server_ip', serverIP.value)
  localStorage.setItem('distribute_post_cmd', postCmd.value)
  const label = selectedTargets.value.length === 0 ? '所有在线设备' : `${selectedTargets.value.length} 台设备`
  if (!confirm(`确定分发 ${selectedFiles.value.size} 个文件到 ${label}？\n保存目录: ${saveDir.value}`)) return
  try {
    await startDistribution({ files: [...selectedFiles.value], save_dir: saveDir.value, target_ids: selectedTargets.value, server_ip: serverIP.value, post_cmd: postCmd.value })
    await load()
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '启动失败')
  }
}

async function stopTask() {
  if (!confirm('确定停止当前分发任务？')) return
  try {
    await stopDistribution()
    await load()
    toast.success('已停止')
  } catch {}
}

async function retryDevice(deviceId: number) {
  try {
    await retryDeviceDistribution(deviceId)
  } catch {}
}

async function clearTask() {
  try {
    await resetDistributionTask()
    task.value = null
    await load()
  } catch {}
}

// WebSocket events
const unsub = store.on('distribute_progress_update', (data) => { task.value = data as DistributeTask })
const unsub2 = store.on('distribute_task_finished', (data) => { task.value = data as DistributeTask })
onUnmounted(() => { unsub(); unsub2() })

function overallPct() {
  if (!task.value?.progresses) return 0
  const p = Object.values(task.value.progresses)
  if (!p.length) return 0
  return Math.round(p.filter(x => x.status === 'completed').length / p.length * 100)
}

function sortedProgress() {
  if (!task.value?.progresses) return []
  return Object.values(task.value.progresses).sort((a, b) => a.device_id - b.device_id)
}

const statusColor: Record<string, string> = {
  completed: 'text-emerald-700 bg-emerald-100',
  failed: 'text-red-700 bg-red-100',
  cancelled: 'text-red-700 bg-red-100',
  idle: 'text-slate-500 bg-slate-100',
  downloading: 'text-amber-700 bg-amber-100',
}
const statusLabel: Record<string, string> = {
  completed: '成功 ✓', failed: '失败 ✕', cancelled: '已取消', idle: '等待', downloading: '下载中',
}
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-5">
      <h1 class="text-xl font-bold text-slate-900">P2P 文件分发</h1>
      <button v-if="task?.status === 'running'" class="px-3 py-2 text-sm bg-red-500 hover:bg-red-600 text-white rounded-lg font-semibold" @click="stopTask">⏹ 停止分发</button>
      <button v-else-if="isActiveTask" class="px-3 py-2 text-sm border border-slate-200 rounded-lg hover:bg-slate-50 font-medium" @click="clearTask">← 返回配置</button>
    </div>

    <!-- Active task view -->
    <template v-if="isActiveTask && task">
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 mb-5">
        <h3 class="font-semibold text-slate-800 mb-1">
          正在分发：<code class="bg-slate-100 px-2 py-0.5 rounded text-blue-600">{{ task.active_file }}</code>
          <span class="text-sm font-normal text-slate-500 ml-2">({{ task.active_idx + 1 }}/{{ task.files.length }})</span>
        </h3>
        <p class="text-xs text-slate-500 mb-3">
          目标目录: <code class="bg-slate-100 px-1 rounded">{{ task.save_dir }}</code>
          <span v-if="task.server_ip"> · 服务器IP: <code class="bg-slate-100 px-1 rounded">{{ task.server_ip }}</code></span>
        </p>
        <div class="w-full h-5 bg-slate-100 rounded-full overflow-hidden relative">
          <div class="h-full bg-blue-500 rounded-full transition-all" :style="{ width: overallPct() + '%' }" />
          <span class="absolute inset-0 text-xs text-center font-semibold text-white flex items-center justify-center">
            {{ overallPct() }}% 完成 ({{ Object.values(task.progresses || {}).filter(p => p.status === 'completed').length }}/{{ Object.keys(task.progresses || {}).length }})
          </span>
        </div>
      </div>

      <div class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="bg-slate-50 border-b border-slate-200 text-left">
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase">设备</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase">主机名</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase min-w-40">进度</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase">速度</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase">状态</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase">错误</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in sortedProgress()" :key="p.device_id" class="border-t border-slate-100 hover:bg-slate-50">
                <td class="px-4 py-3 font-bold text-blue-600">#{{ p.device_id }}</td>
                <td class="px-4 py-3 text-slate-600">{{ p.hostname }}</td>
                <td class="px-4 py-3">
                  <div class="w-full h-4 bg-slate-100 rounded-full overflow-hidden relative">
                    <div
                      class="h-full rounded-full transition-all"
                      :class="p.status === 'failed' ? 'bg-red-500' : 'bg-blue-500'"
                      :style="{ width: Math.round(p.percentage || 0) + '%' }"
                    />
                    <span class="absolute inset-0 flex items-center justify-center text-xs font-semibold text-white">
                      {{ Math.round(p.percentage || 0) }}%{{ p.total_chunks ? ` (${p.downloaded}/${p.total_chunks})` : '' }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3 text-slate-600 text-xs">{{ p.speed_mbps > 0 ? p.speed_mbps + ' Mbps' : '-' }}</td>
                <td class="px-4 py-3">
                  <span :class="['inline-flex px-2 py-0.5 rounded-full text-xs font-semibold', statusColor[p.status] || 'text-slate-500 bg-slate-100']">
                    {{ statusLabel[p.status] || p.status }}
                  </span>
                </td>
                <td class="px-4 py-3 text-xs text-red-500 max-w-xs truncate">{{ p.error || '-' }}</td>
                <td class="px-4 py-3">
                  <button v-if="p.status === 'failed'" class="px-2 py-1 text-xs bg-blue-600 text-white rounded font-medium" @click="retryDevice(p.device_id)">重试</button>
                  <span v-else class="text-slate-400">-</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <!-- Setup view -->
    <template v-else>
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <!-- Left: File management -->
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 flex flex-col gap-4">
          <h2 class="font-semibold text-slate-800">1. 文件管理</h2>

          <!-- Upload -->
          <div class="border-2 border-dashed border-slate-200 rounded-xl p-4 text-center">
            <p class="text-sm text-slate-500 mb-3">向服务器上传文件（支持大文件）</p>
            <input ref="fileInputRef" type="file" class="hidden" @change="handleUpload" />
            <button class="px-4 py-2 text-sm border border-slate-200 rounded-lg hover:bg-slate-50 font-medium" @click="fileInputRef?.click()">+ 选择并上传文件</button>
            <div v-if="uploading" class="mt-3">
              <div class="w-full h-3 bg-slate-100 rounded-full overflow-hidden">
                <div class="h-full bg-blue-500 rounded-full transition-all" :style="{ width: uploadPct + '%' }" />
              </div>
              <div class="text-xs text-slate-500 mt-1">{{ uploadPct }}%</div>
            </div>
          </div>

          <!-- Files table -->
          <div class="border border-slate-200 rounded-lg overflow-hidden max-h-52 overflow-y-auto">
            <table class="w-full text-sm">
              <thead class="sticky top-0 bg-slate-50 border-b border-slate-200">
                <tr>
                  <th class="px-3 py-2 text-center w-8">
                    <input type="checkbox" :checked="files.length > 0 && selectedFiles.size === files.length" @change="selectAllFiles(($event.target as HTMLInputElement).checked)" class="rounded" />
                  </th>
                  <th class="px-3 py-2 text-left text-xs font-semibold text-slate-500 uppercase">文件名</th>
                  <th class="px-3 py-2 text-left text-xs font-semibold text-slate-500 uppercase">大小</th>
                  <th class="px-3 py-2 text-left text-xs font-semibold text-slate-500 uppercase">时间</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!files.length"><td colspan="4" class="text-center py-8 text-slate-400 text-sm">暂无文件</td></tr>
                <tr v-for="f in files" :key="f.name" class="border-t border-slate-100 hover:bg-slate-50 cursor-pointer" @click="toggleFile(f.name)">
                  <td class="px-3 py-2 text-center">
                    <input type="checkbox" :checked="selectedFiles.has(f.name)" class="rounded" @click.stop @change="toggleFile(f.name)" />
                  </td>
                  <td class="px-3 py-2 font-medium">{{ f.name }}</td>
                  <td class="px-3 py-2 text-slate-500">{{ formatBytes(f.size) }}</td>
                  <td class="px-3 py-2 text-slate-400 text-xs">{{ f.mod_time }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="flex gap-2 flex-wrap">
            <button class="px-3 py-1.5 text-xs border border-slate-200 rounded-lg hover:bg-slate-50" @click="selectAllFiles(true)">全选</button>
            <button class="px-3 py-1.5 text-xs border border-slate-200 rounded-lg hover:bg-slate-50" @click="selectAllFiles(false)">取消</button>
            <button class="px-3 py-1.5 text-xs bg-red-500 text-white rounded-lg font-medium" @click="deleteSelected">删除选中</button>
            <button class="px-3 py-1.5 text-xs bg-red-700 text-white rounded-lg font-medium" @click="clearAll">清空服务器</button>
          </div>

          <!-- Config -->
          <div class="space-y-3">
            <div>
              <label class="block text-sm font-semibold text-slate-700 mb-1.5">2. 客户端目标保存目录</label>
              <input v-model="saveDir" type="text" placeholder="/home/cwxu/Downloads" class="w-full px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30" />
              <p class="text-xs text-slate-400 mt-1">目录不存在时自动创建</p>
            </div>
            <div>
              <label class="block text-sm font-semibold text-slate-700 mb-1.5">3. 分发服务器 IP</label>
              <div class="flex gap-2">
                <input v-model="serverIP" type="text" :placeholder="currentHostname" class="flex-1 px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30" />
                <button
                  class="px-3 py-2 text-sm border border-slate-200 rounded-lg hover:bg-slate-50 font-medium whitespace-nowrap"
                  :disabled="precheckLoading"
                  @click="precheck"
                >{{ precheckLoading ? '测试中...' : '⚡ 测试连接' }}</button>
              </div>
              <!-- Precheck results -->
              <div v-if="precheckResults.length" class="mt-2 border border-slate-200 rounded-lg p-3 text-xs space-y-1 max-h-36 overflow-y-auto">
                <div class="font-semibold text-slate-700 mb-1">
                  测试完成 — 成功: {{ precheckResults.filter(r => r.success).length }} / 失败: {{ precheckResults.filter(r => !r.success).length }}
                </div>
                <div v-for="r in precheckResults" :key="r.device_id" :class="r.success ? 'text-emerald-700' : 'text-red-600'">
                  {{ r.success ? '✓' : '✗' }} 设备 #{{ r.device_id }}{{ r.error ? ' — ' + r.error : '' }}
                </div>
              </div>
            </div>
          </div>

          <button class="w-full py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-semibold text-sm mt-2" @click="startTask">▶ 开启 P2P 分发</button>
        </div>

        <!-- Right: Target & post command -->
        <div class="flex flex-col gap-4">
          <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
            <h2 class="font-semibold text-slate-800 mb-3">4. 目标接收设备</h2>
            <DeviceSelector v-model="selectedTargets" :devices="devices" />
          </div>
          <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
            <h2 class="font-semibold text-slate-800 mb-1">5. 分发后执行命令 (选填)</h2>
            <p class="text-xs text-slate-400 mb-2">文件下载成功后自动执行，工作目录为目标保存目录</p>
            <CodeEditor v-model="postCmd" height="120px" />
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
