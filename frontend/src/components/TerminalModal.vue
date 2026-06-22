<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import Modal from './Modal.vue'

const props = defineProps<{ deviceId: number }>()
const emit = defineEmits<{ close: [] }>()

const containerRef = ref<HTMLDivElement>()
let term: import('@xterm/xterm').Terminal | null = null
let ws: WebSocket | null = null

onMounted(async () => {
  const { Terminal } = await import('@xterm/xterm')
  const { FitAddon } = await import('@xterm/addon-fit')

  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: '"Fira Code", "Consolas", monospace',
    theme: {
      background: '#1e1e2e',
      foreground: '#cdd6f4',
      cursor: '#f5e0dc',
      selectionBackground: 'rgba(205, 214, 244, 0.2)',
    },
  })

  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(containerRef.value!)
  fitAddon.fit()

  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  ws = new WebSocket(`${protocol}//${location.host}/ws/terminal/${props.deviceId}?cols=${term.cols}&rows=${term.rows}`)
  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    term!.write('\x1b[32m已连接到设备 #' + props.deviceId + '\x1b[0m\r\n')
  }
  ws.onmessage = (e) => {
    if (term && e.data) term.write(new Uint8Array(e.data as ArrayBuffer))
  }
  ws.onclose = () => {
    if (term) term.write('\r\n\x1b[31m连接已断开\x1b[0m\r\n')
  }
  ws.onerror = () => {
    if (term) term.write('\r\n\x1b[31m连接错误\x1b[0m\r\n')
  }

  term.onData((data) => {
    if (ws?.readyState === WebSocket.OPEN) ws.send(data)
  })

  term.onResize(({ cols, rows }) => {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols, rows }))
    }
  })

  const resizeObserver = new ResizeObserver(() => fitAddon.fit())
  resizeObserver.observe(containerRef.value!)
})

onUnmounted(() => {
  ws?.close()
  term?.dispose()
})
</script>

<template>
  <Modal :title="`终端 — 设备 #${deviceId}`" max-width="900px" @close="emit('close')">
    <div class="p-4">
      <div ref="containerRef" class="h-[520px] rounded-lg overflow-hidden bg-[#1e1e2e]" />
    </div>
  </Modal>
</template>
