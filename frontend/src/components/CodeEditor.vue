<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { StreamLanguage } from '@codemirror/language'
import { shell } from '@codemirror/legacy-modes/mode/shell'
import { oneDark } from '@codemirror/theme-one-dark'

const props = defineProps<{
  modelValue: string
  height?: string
  dark?: boolean
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const containerRef = ref<HTMLDivElement>()
let view: EditorView | null = null

onMounted(() => {
  const extensions = [
    basicSetup,
    StreamLanguage.define(shell),
    EditorView.updateListener.of(update => {
      if (update.docChanged) {
        emit('update:modelValue', update.state.doc.toString())
      }
    }),
    EditorView.theme({
      '&': { height: props.height || '200px' },
      '.cm-scroller': { overflow: 'auto', fontFamily: '"Fira Code", "Consolas", monospace' },
    }),
  ]

  if (props.dark) extensions.push(oneDark)

  view = new EditorView({
    state: EditorState.create({
      doc: props.modelValue,
      extensions,
    }),
    parent: containerRef.value!,
  })
})

onUnmounted(() => {
  view?.destroy()
  view = null
})

watch(() => props.modelValue, (val) => {
  if (!view) return
  const current = view.state.doc.toString()
  if (current !== val) {
    view.dispatch({
      changes: { from: 0, to: current.length, insert: val },
    })
  }
})

function getValue() {
  return view?.state.doc.toString() ?? ''
}

defineExpose({ getValue })
</script>

<template>
  <div ref="containerRef" class="code-editor rounded-lg overflow-hidden border border-slate-200" />
</template>
