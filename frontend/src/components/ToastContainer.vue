<script setup lang="ts">
import type { Toast } from '@/composables/useToast'

defineProps<{ toasts: Toast[] }>()
const emit = defineEmits<{ remove: [id: number] }>()

const iconMap = {
  success: '✓',
  error: '✕',
  warning: '⚠',
  info: 'ℹ',
}
const colorMap = {
  success: 'bg-emerald-500',
  error: 'bg-red-500',
  warning: 'bg-amber-500',
  info: 'bg-blue-500',
}
</script>

<template>
  <div class="fixed top-4 right-4 z-[9999] flex flex-col gap-2 max-w-sm w-full pointer-events-none">
    <TransitionGroup name="toast">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="flex items-center gap-3 px-4 py-3 rounded-lg shadow-lg bg-slate-900 text-white text-sm pointer-events-auto cursor-pointer"
        @click="emit('remove', t.id)"
      >
        <span
          :class="[colorMap[t.type], 'w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0']"
        >{{ iconMap[t.type] }}</span>
        <span class="flex-1">{{ t.message }}</span>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active, .toast-leave-active { transition: all 0.25s ease; }
.toast-enter-from { opacity: 0; transform: translateX(100%); }
.toast-leave-to { opacity: 0; transform: translateX(100%); }
</style>
