<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { getDevices, getDevice, getSettings, updateSettings } from '@/api'
import { useToast } from '@/composables/useToast'
import { useFormat } from '@/composables/useFormat'
import Modal from '@/components/Modal.vue'
import type { Device } from '@/types'

const store = useAppStore()
const toast = useToast()
const { getDeviceIP } = useFormat()

const devices = ref<Device[]>([])
const enabled = ref(false)
const largeScreen = ref<{ id: number; hostname: string; ip: string; checkin: string } | null>(null)
const loading = ref(true)

async function load() {
  const [s, d] = await Promise.all([getSettings(), getDevices()])
  enabled.value = s.screen_monitor_enabled || false
  devices.value = d.sort((a, b) => a.assigned_id - b.assigned_id)
  loading.value = false
}
load()

async function toggleEnabled(val: boolean) {
  try {
    const s = await updateSettings({ screen_monitor_enabled: val })
    enabled.value = s.screen_monitor_enabled || false
    toast.success(val ? '屏幕捕捉已开启' : '屏幕捕捉已关闭')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '操作失败')
    enabled.value = !val
  }
}

async function openLarge(d: Device) {
  const ip = getDeviceIP(d.local_ip)
  if (!ip || !d.connected) { toast.error('设备离线，无法开启监控'); return }
  const checkin = d.student_name ? `${d.student_name} (${d.student_num})` : '未签到'
  largeScreen.value = { id: d.assigned_id, hostname: d.hostname, ip, checkin }
}

function streamUrl(ip: string, hd = false) {
  return `http://${ip}:8090/screen${hd ? '?hd=1' : ''}`
}

const unsub = store.on('device_connected', load)
const unsub2 = store.on('device_disconnected', load)
onUnmounted(() => { unsub(); unsub2() })
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-5">
      <h1 class="text-xl font-bold text-slate-900">选手屏幕</h1>
      <label class="flex items-center gap-3 cursor-pointer">
        <span class="text-sm font-medium text-slate-700">屏幕捕捉</span>
        <div class="relative">
          <input
            type="checkbox"
            class="sr-only"
            :checked="enabled"
            @change="toggleEnabled(($event.target as HTMLInputElement).checked)"
          />
          <div
            class="w-11 h-6 rounded-full transition-colors"
            :class="enabled ? 'bg-emerald-500' : 'bg-slate-300'"
          >
            <div
              class="absolute top-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform"
              :class="enabled ? 'translate-x-5' : 'translate-x-0.5'"
            />
          </div>
        </div>
      </label>
    </div>

    <div v-if="loading" class="text-center py-16 text-slate-400">加载中...</div>
    <div v-else-if="!enabled" class="bg-white rounded-xl border border-slate-200 shadow-sm p-12 text-center text-slate-400">
      屏幕捕捉功能未开启，请在右上方开启。
    </div>
    <div v-else-if="!devices.length" class="bg-white rounded-xl border border-slate-200 shadow-sm p-12 text-center text-slate-400">
      暂无已注册设备
    </div>

    <!-- Screen grid -->
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      <div
        v-for="d in devices"
        :key="d.assigned_id"
        class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden cursor-pointer hover:shadow-md transition-shadow"
        @click="openLarge(d)"
      >
        <!-- Card header -->
        <div class="flex items-center justify-between px-3 py-2 bg-slate-50 border-b border-slate-200 text-xs">
          <span class="font-bold text-blue-600">#{{ d.assigned_id }}</span>
          <span class="text-slate-500 truncate mx-2">{{ d.student_name || d.hostname }}</span>
          <span :class="['px-1.5 py-0.5 rounded-full font-semibold', d.connected ? 'text-emerald-700 bg-emerald-100' : 'text-red-700 bg-red-100']">
            {{ d.connected ? '在线' : '离线' }}
          </span>
        </div>

        <!-- Screen area -->
        <div class="relative bg-black" style="padding-top: 56.25%">
          <template v-if="d.connected && getDeviceIP(d.local_ip)">
            <img
              :src="streamUrl(getDeviceIP(d.local_ip)!)"
              class="absolute inset-0 w-full h-full object-contain"
              loading="lazy"
            />
          </template>
          <div v-else class="absolute inset-0 flex items-center justify-center text-slate-500 text-sm">
            {{ d.connected ? '未知IP' : '设备离线' }}
          </div>
        </div>
      </div>
    </div>

    <!-- Large screen modal -->
    <Modal
      v-if="largeScreen"
      :title="`选手 #${largeScreen.id} (${largeScreen.hostname}) — ${largeScreen.checkin}`"
      max-width="1100px"
      @close="largeScreen = null"
    >
      <div class="p-4">
        <div class="relative bg-black rounded-lg overflow-hidden" style="aspect-ratio: 16/9">
          <img
            :src="streamUrl(largeScreen.ip, true)"
            class="w-full h-full object-contain"
          />
        </div>
      </div>
    </Modal>
  </div>
</template>
