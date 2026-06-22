import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { AdminEvent, Device } from '@/types'
import { getStats } from '@/api'

type EventHandler = (data: unknown) => void

export const useAppStore = defineStore('app', () => {
  const onlineCount = ref(0)
  const totalCount = ref(0)
  const wsConnected = ref(false)
  const devices = ref<Device[]>([])

  let ws: WebSocket | null = null
  const handlers = new Map<string, Set<EventHandler>>()

  function on(event: string, handler: EventHandler) {
    if (!handlers.has(event)) handlers.set(event, new Set())
    handlers.get(event)!.add(handler)
    return () => handlers.get(event)?.delete(handler)
  }

  function emit(event: string, data: unknown) {
    handlers.get(event)?.forEach(h => h(data))
    handlers.get('*')?.forEach(h => h({ event, data }))
  }

  async function refreshStats() {
    try {
      const stats = await getStats()
      onlineCount.value = stats.online_devices
      totalCount.value = stats.total_devices
    } catch {}
  }

  function connect() {
    if (ws) return
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    ws = new WebSocket(`${protocol}//${location.host}/ws/admin`)

    ws.onopen = () => {
      wsConnected.value = true
      refreshStats()
    }

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data) as AdminEvent
        emit(msg.event, msg.data)

        const refreshEvents = ['device_connected', 'device_disconnected', 'device_updated', 'checkin_updated']
        if (refreshEvents.includes(msg.event)) {
          refreshStats()
        }
      } catch {}
    }

    ws.onclose = () => {
      wsConnected.value = false
      ws = null
      setTimeout(connect, 3000)
    }

    ws.onerror = () => {
      ws?.close()
    }
  }

  function disconnect() {
    ws?.close()
    ws = null
  }

  return {
    onlineCount,
    totalCount,
    wsConnected,
    devices,
    on,
    refreshStats,
    connect,
    disconnect,
  }
})
