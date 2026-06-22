<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { getStats } from '@/api'
import { useFormat } from '@/composables/useFormat'
import type { Stats } from '@/types'

const store = useAppStore()
const { statusLabel, statusColor } = useFormat()
const stats = ref<Stats | null>(null)
const loading = ref(true)

async function load() {
  try {
    stats.value = await getStats()
  } finally {
    loading.value = false
  }
}

load()

// Refresh on device events
const unsub = store.on('command_status', load)
const unsub2 = store.on('device_connected', load)
const unsub3 = store.on('device_disconnected', load)
onUnmounted(() => { unsub(); unsub2(); unsub3() })
</script>

<template>
  <div>
    <h1 class="text-xl font-bold text-slate-900 mb-5">仪表盘</h1>

    <div v-if="loading" class="flex justify-center py-16 text-slate-400">加载中...</div>

    <template v-else-if="stats">
      <!-- Stats grid -->
      <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4 mb-6">
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
          <div class="text-3xl font-bold text-slate-900">{{ stats.total_devices }}</div>
          <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">设备总数</div>
        </div>
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
          <div class="text-3xl font-bold text-emerald-600">{{ stats.online_devices }}</div>
          <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">在线设备</div>
        </div>
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
          <div class="text-3xl font-bold text-red-500">{{ stats.offline_devices }}</div>
          <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">离线设备</div>
        </div>
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
          <div class="text-3xl font-bold text-blue-600">{{ stats.checked_in || 0 }}</div>
          <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">已签到</div>
        </div>
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-5 text-center">
          <div class="text-3xl font-bold text-slate-700">{{ stats.total_commands }}</div>
          <div class="text-xs text-slate-500 uppercase tracking-wide mt-1">命令总数</div>
        </div>
      </div>

      <!-- Recent commands -->
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
        <div class="px-5 py-4 border-b border-slate-100">
          <h2 class="font-semibold text-slate-800">最近命令</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="bg-slate-50 text-left">
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">ID</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">时间</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">目标</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">命令</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">状态</th>
                <th class="px-4 py-3 text-xs font-semibold text-slate-500 uppercase tracking-wide">耗时</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!stats.recent_commands?.length">
                <td colspan="6" class="text-center py-10 text-slate-400">暂无命令记录</td>
              </tr>
              <tr
                v-for="cmd in stats.recent_commands"
                :key="cmd.id"
                class="border-t border-slate-100 hover:bg-slate-50"
              >
                <td class="px-4 py-3 text-slate-500">#{{ cmd.id }}</td>
                <td class="px-4 py-3 text-slate-500 text-xs whitespace-nowrap">{{ cmd.created_at }}</td>
                <td class="px-4 py-3 font-medium">{{ cmd.target_type === 'broadcast' ? '全部' : '#' + cmd.target_id }}</td>
                <td class="px-4 py-3 font-mono text-xs text-slate-700 max-w-xs truncate">{{ cmd.command.substring(0, 60) }}{{ cmd.command.length > 60 ? '...' : '' }}</td>
                <td class="px-4 py-3">
                  <span :class="['inline-flex px-2 py-0.5 rounded-full text-xs font-semibold', statusColor(cmd.status)]">
                    {{ statusLabel(cmd.status) }}
                  </span>
                </td>
                <td class="px-4 py-3 text-slate-500 text-xs">{{ cmd.duration_ms ? cmd.duration_ms + 'ms' : '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>
