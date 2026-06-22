<script setup lang="ts">
defineProps<{ title?: string; maxWidth?: string }>()
const emit = defineEmits<{ close: [] }>()

function onOverlayClick(e: MouseEvent) {
  if (e.target === e.currentTarget) emit('close')
}
</script>

<template>
  <Teleport to="body">
    <div
      class="fixed inset-0 z-[100] flex items-center justify-center bg-black/40 backdrop-blur-sm"
      @click="onOverlayClick"
    >
      <div
        class="bg-white rounded-xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh] w-full"
        :style="{ maxWidth: maxWidth || '600px' }"
        @click.stop
      >
        <!-- Header -->
        <div v-if="title || $slots.header" class="flex items-center justify-between px-6 py-4 border-b border-slate-200">
          <slot name="header">
            <h2 class="text-lg font-semibold text-slate-900">{{ title }}</h2>
          </slot>
          <button
            class="text-slate-400 hover:text-slate-700 text-2xl leading-none w-8 h-8 flex items-center justify-center rounded"
            @click="emit('close')"
          >&times;</button>
        </div>

        <!-- Body -->
        <div class="overflow-y-auto flex-1">
          <slot />
        </div>

        <!-- Footer -->
        <div v-if="$slots.footer" class="px-6 py-4 border-t border-slate-200 bg-slate-50">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>
