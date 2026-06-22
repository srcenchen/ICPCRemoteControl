<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Device } from '@/types'

const props = defineProps<{
  devices: Device[]
  modelValue: number[] // selected IDs, empty = broadcast
}>()

const emit = defineEmits<{ 'update:modelValue': [ids: number[]] }>()

const search = ref('')

function safeIP(localIp: string) {
  try { return (JSON.parse(localIp || '[]') as Array<{ ipv4?: string }>)[0]?.ipv4?.split('/')[0] ?? '' }
  catch { return '' }
}

const filtered = computed(() => {
  const q = search.value.toLowerCase()
  if (!q) return props.devices
  return props.devices.filter(d =>
    String(d.assigned_id).includes(q) ||
    d.hostname.toLowerCase().includes(q) ||
    (d.local_ip && d.local_ip.toLowerCase().includes(q))
  )
})

function isSelected(id: number) {
  return props.modelValue.includes(id)
}

function toggle(id: number) {
  const sel = props.modelValue.slice()
  const idx = sel.indexOf(id)
  if (idx >= 0) sel.splice(idx, 1)
  else sel.push(id)
  emit('update:modelValue', sel)
}

function selectAll() {
  emit('update:modelValue', props.devices.map(d => d.assigned_id))
}

function selectOnline() {
  emit('update:modelValue', props.devices.filter(d => d.connected).map(d => d.assigned_id))
}

function selectNone() {
  emit('update:modelValue', [])
}

const isBroadcast = computed(() => props.modelValue.length === 0)
const selectedLabel = computed(() => {
  if (isBroadcast.value) return `全部在线广播 (${props.devices.filter(d => d.connected).length} 台)`
  return `已选 ${props.modelValue.length} 台`
})
</script>

<template>
  <div class="flex flex-col gap-2">
    <!-- Toolbar -->
    <div class="flex gap-1.5 flex-wrap items-center">
      <input
        v-model="search"
        type="text"
        placeholder="搜索设备..."
        class="flex-1 min-w-24 text-sm px-3 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30 focus:border-blue-400"
      />
      <button class="px-2 py-1.5 text-xs border border-slate-200 rounded-lg hover:bg-slate-50" @click="selectNone">广播</button>
      <button class="px-2 py-1.5 text-xs border border-slate-200 rounded-lg hover:bg-slate-50" @click="selectOnline">在线</button>
      <button class="px-2 py-1.5 text-xs border border-slate-200 rounded-lg hover:bg-slate-50" @click="selectAll">全选</button>
    </div>

    <!-- Status bar -->
    <div class="text-xs text-slate-500 font-medium">
      <span
        :class="isBroadcast ? 'text-blue-600' : 'text-slate-700'"
      >{{ selectedLabel }}</span>
    </div>

    <!-- Device list -->
    <div class="border border-slate-200 rounded-lg overflow-y-auto max-h-64 bg-white">
      <!-- Broadcast row -->
      <div
        class="flex items-center gap-2 px-3 py-2 border-b border-slate-100 cursor-pointer text-sm font-semibold"
        :class="isBroadcast ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-50'"
        @click="selectNone"
      >
        <span class="w-4 text-center text-blue-600">{{ isBroadcast ? '●' : '○' }}</span>
        <span>📡 全部广播</span>
      </div>

      <!-- Device rows -->
      <div
        v-for="d in filtered"
        :key="d.assigned_id"
        class="flex items-center gap-2 px-3 py-1.5 cursor-pointer text-sm border-b border-slate-50 last:border-0"
        :class="[
          isSelected(d.assigned_id) ? 'bg-blue-50 text-blue-700' : 'hover:bg-slate-50 text-slate-700',
          !d.connected ? 'opacity-50' : '',
        ]"
        @click="toggle(d.assigned_id)"
      >
        <span class="w-4 text-center" :class="isSelected(d.assigned_id) ? 'text-blue-600' : 'text-slate-300'">
          {{ isSelected(d.assigned_id) ? '✓' : '·' }}
        </span>
        <div
          class="w-2 h-2 rounded-full flex-shrink-0"
          :class="d.connected ? 'bg-emerald-400' : 'bg-red-400'"
        />
        <span class="w-8 font-mono font-bold text-xs" :class="isSelected(d.assigned_id) ? 'text-blue-700' : 'text-slate-400'">#{{ d.assigned_id }}</span>
        <span class="flex-1 truncate">{{ d.hostname || '未知' }}</span>
        <span class="text-xs text-slate-400 max-w-24 truncate">{{ safeIP(d.local_ip) }}</span>
      </div>

      <div v-if="filtered.length === 0" class="py-8 text-center text-slate-400 text-sm">
        无匹配设备
      </div>
    </div>
  </div>
</template>
