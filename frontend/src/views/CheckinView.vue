<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '@/stores/app'
import {
  getCheckinList, getCheckinStats, doCheckin, doCheckout,
  restoreCheckout, resetCheckin, swapCheckin, resetAllCheckin, exportCheckin
} from '@/api'
import { useToast } from '@/composables/useToast'
import Modal from '@/components/Modal.vue'
import type { Device } from '@/types'

const store = useAppStore()
const toast = useToast()

const devices = ref<Device[]>([])
const stats = ref({ total: 0, checked_in: 0, checked_out: 0, not_checked: 0 })
const loading = ref(true)

// Checkin modal
const checkinModal = ref(false)
const checkinId = ref(0)
const checkinName = ref('')
const checkinNum = ref('')

// Swap modal
const swapModal = ref(false)
const swapFromId = ref(0)
const swapToId = ref(0)
const swapCandidates = ref<Device[]>([])

async function load() {
  loading.value = true
  try {
    const [s, d] = await Promise.all([getCheckinStats(), getCheckinList()])
    stats.value = s
    devices.value = d
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '加载失败')
  } finally {
    loading.value = false
  }
}
load()

async function confirmCheckin() {
  if (!checkinName.value || !checkinNum.value) { toast.error('请填写姓名和学号'); return }
  try {
    await doCheckin(checkinId.value, checkinName.value, checkinNum.value)
    checkinModal.value = false
    await load()
    toast.success('签到成功')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '签到失败')
  }
}

async function checkout(id: number) {
  if (!confirm(`确定将设备 #${id} 签退？`)) return
  try {
    await doCheckout(id)
    await load()
    toast.success('已签退')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '签退失败')
  }
}

async function restore(id: number) {
  if (!confirm(`确定恢复设备 #${id} 的签到（撤销签退）？`)) return
  try {
    await restoreCheckout(id)
    await load()
    toast.success('已恢复签到')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '恢复失败')
  }
}

async function reset(id: number) {
  if (!confirm(`确定解除设备 #${id} 的签到？`)) return
  try {
    await resetCheckin(id)
    await load()
    toast.success('已解除签到')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '操作失败')
  }
}

async function resetAll() {
  if (!confirm('确定解除所有设备的签到？此操作不可撤销。')) return
  try {
    const res = await resetAllCheckin()
    await load()
    toast.success(`已解除 ${res.affected_count} 台设备的签到`)
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '操作失败')
  }
}

async function openSwap(fromId: number) {
  swapFromId.value = fromId
  swapCandidates.value = devices.value.filter(d => d.assigned_id !== fromId && d.checkin_status === 0 && d.connected)
  swapToId.value = swapCandidates.value[0]?.assigned_id ?? 0
  swapModal.value = true
}

async function confirmSwap() {
  if (!swapToId.value) { toast.error('请选择目标设备'); return }
  if (!confirm(`确定将签到信息从设备 #${swapFromId.value} 迁移到 #${swapToId.value}？`)) return
  try {
    await swapCheckin(swapFromId.value, swapToId.value)
    swapModal.value = false
    await load()
    toast.success('签到信息迁移成功')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '迁移失败')
  }
}

function openCheckinModal(id: number) {
  checkinId.value = id
  checkinName.value = ''
  checkinNum.value = ''
  checkinModal.value = true
}

function checkinStatusInfo(s: number) {
  if (s === 1) return { text: '已签到', cls: 'text-emerald-700 bg-emerald-100' }
  if (s === 2) return { text: '已签退', cls: 'text-slate-600 bg-slate-100' }
  return { text: '未签到', cls: 'text-red-600 bg-red-100' }
}

const unsub = store.on('checkin_updated', load)
onUnmounted(() => unsub())
</script>

<template>
  <div>
    <h1 class="text-xl font-bold text-slate-900 mb-5">签到管理</h1>

    <!-- Stats -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-5">
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
        <div class="text-3xl font-bold text-slate-900">{{ stats.total }}</div>
        <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">设备总数</div>
      </div>
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
        <div class="text-3xl font-bold text-emerald-600">{{ stats.checked_in }}</div>
        <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">已签到</div>
      </div>
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
        <div class="text-3xl font-bold text-amber-500">{{ stats.checked_out }}</div>
        <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">已签退</div>
      </div>
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
        <div class="text-3xl font-bold text-slate-400">{{ stats.not_checked }}</div>
        <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">未签到</div>
      </div>
    </div>

    <!-- Table header -->
    <div class="flex items-center justify-between mb-3">
      <h2 class="font-semibold text-slate-800">设备签到列表</h2>
      <div class="flex gap-2">
        <a :href="exportCheckin()" download class="px-3 py-2 text-sm border border-slate-200 rounded-lg hover:bg-slate-50 font-medium text-slate-700">导出 Excel</a>
        <button class="px-3 py-2 text-sm bg-red-500 hover:bg-red-600 text-white rounded-lg font-medium" @click="resetAll">解除全部签到</button>
      </div>
    </div>

    <!-- Table -->
    <div class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="bg-slate-50 text-left border-b border-slate-200">
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">编号</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">主机名</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">学生姓名</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">学号</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">签到状态</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">在线</th>
              <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td colspan="7" class="text-center py-12 text-slate-400">加载中...</td>
            </tr>
            <tr v-else-if="!devices.length">
              <td colspan="7" class="text-center py-12 text-slate-400">暂无设备</td>
            </tr>
            <tr v-for="d in devices" :key="d.assigned_id" class="border-t border-slate-100 hover:bg-slate-50">
              <td class="px-4 py-3 font-bold text-blue-600">#{{ d.assigned_id }}</td>
              <td class="px-4 py-3 font-medium">{{ d.hostname }}</td>
              <td class="px-4 py-3">{{ d.student_name || '-' }}</td>
              <td class="px-4 py-3 text-slate-600">{{ d.student_num || '-' }}</td>
              <td class="px-4 py-3">
                <span :class="['inline-flex px-2 py-0.5 rounded-full text-xs font-semibold', checkinStatusInfo(d.checkin_status).cls]">
                  {{ checkinStatusInfo(d.checkin_status).text }}
                </span>
              </td>
              <td class="px-4 py-3">
                <span :class="['inline-flex w-2 h-2 rounded-full', d.connected ? 'bg-emerald-400' : 'bg-red-400']" />
                <span class="ml-1.5 text-xs text-slate-500">{{ d.connected ? '在线' : '离线' }}</span>
              </td>
              <td class="px-4 py-3">
                <div class="flex gap-1.5 flex-wrap">
                  <template v-if="d.checkin_status === 0">
                    <button class="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded font-medium" @click="openCheckinModal(d.assigned_id)">签到</button>
                  </template>
                  <template v-else-if="d.checkin_status === 1">
                    <button class="px-2 py-1 text-xs bg-blue-500 hover:bg-blue-600 text-white rounded font-medium" @click="openSwap(d.assigned_id)">换设备</button>
                    <button class="px-2 py-1 text-xs bg-amber-500 hover:bg-amber-600 text-white rounded font-medium" @click="checkout(d.assigned_id)">签退</button>
                    <button class="px-2 py-1 text-xs bg-red-500 hover:bg-red-600 text-white rounded font-medium" @click="reset(d.assigned_id)">解除</button>
                  </template>
                  <template v-else-if="d.checkin_status === 2">
                    <button class="px-2 py-1 text-xs bg-blue-500 hover:bg-blue-600 text-white rounded font-medium" @click="restore(d.assigned_id)">恢复</button>
                    <button class="px-2 py-1 text-xs bg-red-500 hover:bg-red-600 text-white rounded font-medium" @click="reset(d.assigned_id)">撤销</button>
                  </template>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Checkin Modal -->
    <Modal v-if="checkinModal" :title="`签到 — 设备 #${checkinId}`" max-width="420px" @close="checkinModal = false">
      <div class="p-6 flex flex-col gap-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1.5">学生姓名</label>
          <input v-model="checkinName" type="text" placeholder="请输入姓名" class="w-full px-3 py-2.5 border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30 focus:border-blue-400" autofocus />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1.5">学生学号</label>
          <input v-model="checkinNum" type="text" placeholder="请输入学号" class="w-full px-3 py-2.5 border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30 focus:border-blue-400" @keydown.enter="confirmCheckin" />
        </div>
        <button class="w-full py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-semibold text-sm" @click="confirmCheckin">确认签到</button>
      </div>
    </Modal>

    <!-- Swap Modal -->
    <Modal v-if="swapModal" :title="`换设备 — #${swapFromId}`" max-width="420px" @close="swapModal = false">
      <div class="p-6 flex flex-col gap-4">
        <p class="text-sm text-slate-600">将设备 #{{ swapFromId }} 的签到信息迁移到新设备，原设备重置为未签到。</p>
        <div v-if="!swapCandidates.length" class="text-sm text-slate-500 text-center py-4">无可用的在线未签到设备</div>
        <select v-else v-model.number="swapToId" class="w-full px-3 py-2.5 border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30">
          <option v-for="c in swapCandidates" :key="c.assigned_id" :value="c.assigned_id">
            #{{ c.assigned_id }} - {{ c.hostname }}
          </option>
        </select>
        <div class="flex gap-2">
          <button class="flex-1 py-2.5 border border-slate-200 text-slate-700 rounded-lg font-medium text-sm hover:bg-slate-50" @click="swapModal = false">取消</button>
          <button class="flex-1 py-2.5 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-semibold text-sm" :disabled="!swapCandidates.length" @click="confirmSwap">确认迁移</button>
        </div>
      </div>
    </Modal>
  </div>
</template>
