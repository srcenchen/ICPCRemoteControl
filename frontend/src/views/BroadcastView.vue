<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import {
  getBroadcastPages, createBroadcastPage, updateBroadcastPage, deleteBroadcastPage,
  createBroadcastItem, updateBroadcastItem, updateBroadcastItemPosition, deleteBroadcastItem,
  getBroadcastFonts, uploadBroadcastFont, deleteBroadcastFont,
  uploadBroadcastImage, getBroadcastConfig, updateBroadcastConfig,
  executeCommand,
} from '@/api'
import { useToast } from '@/composables/useToast'
import type { BroadcastPage, BroadcastItem, BroadcastFont, BroadcastConfig } from '@/types'

const toast = useToast()

// ─── State ────────────────────────────────────────────────────────────────────
const mode = ref<'before' | 'contesting' | 'after'>('before')
const pages = ref<BroadcastPage[]>([])
const fonts = ref<BroadcastFont[]>([])
const config = ref<BroadcastConfig>({ active_font: '', base_url: '', pushed_state: '', countdown_target: '' })
const selPageId = ref<number | null>(null)
const selItemId = ref<number | null>(null)

const selPage = computed(() => pages.value.find(p => p.id === selPageId.value) ?? null)
const selItem = computed(() => selPage.value?.items?.find(i => i.id === selItemId.value) ?? null)
const sortedItems = computed(() => selPage.value?.items ? [...selPage.value.items].sort((a, b) => (b.z_index || 10) - (a.z_index || 10)) : [])

// ─── Clock ────────────────────────────────────────────────────────────────────
const clockDisplay = ref('00:00:00')
let clockTimer: ReturnType<typeof setInterval>
onMounted(() => {
  clockTimer = setInterval(() => {
    const d = new Date()
    clockDisplay.value = [d.getHours(), d.getMinutes(), d.getSeconds()].map(n => String(n).padStart(2, '0')).join(':')
  }, 1000)
})
onUnmounted(() => clearInterval(clockTimer))

// ─── Canvas drag/resize ───────────────────────────────────────────────────────
const canvasRef = ref<HTMLDivElement>()
type DragState = { itemId: number; startX: number; startY: number; startLeft: number; startTop: number }
type ResizeState = DragState & { startW: number; startH: number; handle: string }

let dragState: DragState | null = null
let resizeState: ResizeState | null = null
const isDragging = ref(false)

function canvasRect(): DOMRect | null {
  return canvasRef.value?.getBoundingClientRect() ?? null
}

function startDrag(e: PointerEvent, item: BroadcastItem) {
  if ((e.target as HTMLElement).classList.contains('bc-handle')) return
  e.stopPropagation()
  selItemId.value = item.id
  const rect = canvasRect()
  if (!rect) return
  dragState = { itemId: item.id, startX: e.clientX, startY: e.clientY, startLeft: item.pos_x, startTop: item.pos_y }
  isDragging.value = true
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  ;(e.target as HTMLElement).setPointerCapture(e.pointerId)
}

function startResize(e: PointerEvent, item: BroadcastItem, handle: string) {
  e.stopPropagation()
  const rect = canvasRect()
  if (!rect) return
  resizeState = { itemId: item.id, startX: e.clientX, startY: e.clientY, startLeft: item.pos_x, startTop: item.pos_y, startW: item.width, startH: item.height, handle }
  isDragging.value = true
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  ;(e.target as HTMLElement).setPointerCapture(e.pointerId)
}

function onPointerMove(e: PointerEvent) {
  const rect = canvasRect()
  if (!rect) return

  if (dragState) {
    const dx = ((e.clientX - dragState.startX) / rect.width) * 100
    const dy = ((e.clientY - dragState.startY) / rect.height) * 100
    updateItemLocal(dragState.itemId, {
      pos_x: clamp(dragState.startLeft + dx, 0, 90),
      pos_y: clamp(dragState.startTop + dy, 0, 90),
    })
  }

  if (resizeState) {
    const dx = ((e.clientX - resizeState.startX) / rect.width) * 100
    const dy = ((e.clientY - resizeState.startY) / rect.height) * 100
    const h = resizeState.handle
    let x = resizeState.startLeft
    let y = resizeState.startTop
    let w = resizeState.startW
    let ht = resizeState.startH

    if (h.includes('e')) w = Math.max(2, resizeState.startW + dx)
    if (h.includes('s')) ht = Math.max(2, resizeState.startH + dy)
    if (h.includes('w')) { x = resizeState.startLeft + dx; w = Math.max(2, resizeState.startW - dx) }
    if (h.includes('n')) { y = resizeState.startTop + dy; ht = Math.max(2, resizeState.startH - dy) }

    updateItemLocal(resizeState.itemId, { pos_x: x, pos_y: y, width: w, height: ht })
  }
}

function onPointerUp() {
  isDragging.value = false
  if (dragState) {
    const item = findItem(dragState.itemId)
    if (item) saveItemPosition(item)
    dragState = null
  }
  if (resizeState) {
    const item = findItem(resizeState.itemId)
    if (item) saveItemPosition(item)
    resizeState = null
  }
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
}

function clamp(v: number, min: number, max: number) { return Math.min(max, Math.max(min, v)) }

function findItem(id: number): BroadcastItem | undefined {
  for (const p of pages.value) {
    const item = p.items?.find(i => i.id === id)
    if (item) return item
  }
}

function updateItemLocal(id: number, patch: Partial<BroadcastItem>) {
  for (const p of pages.value) {
    const item = p.items?.find(i => i.id === id)
    if (item) { Object.assign(item, patch); return }
  }
}

async function saveItemPosition(item: BroadcastItem) {
  try {
    await updateBroadcastItemPosition(item.id, { pos_x: rd(item.pos_x), pos_y: rd(item.pos_y), width: rd(item.width), height: rd(item.height) })
  } catch {}
}

function rd(v: number) { return Math.round(v * 100) / 100 }

// ─── Load ────────────────────────────────────────────────────────────────────
async function loadAll() {
  try {
    const [cfg, fs] = await Promise.all([getBroadcastConfig(), getBroadcastFonts()])
    config.value = cfg
    fonts.value = fs
    await loadPages()
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : '加载失败')
  }
}

async function loadPages() {
  try {
    const res = await getBroadcastPages(mode.value)
    pages.value = res.pages || (res as unknown as BroadcastPage[])
  } catch {}
}

onMounted(loadAll)

function switchMode(m: typeof mode.value) {
  mode.value = m
  selPageId.value = null
  selItemId.value = null
  loadPages()
}

// ─── Page CRUD ───────────────────────────────────────────────────────────────
const editPage = ref<Partial<BroadcastPage>>({})

watch(selPage, (p) => {
  if (p) editPage.value = { title: p.title, duration_ms: p.duration_ms, bg_color: p.bg_color }
})

async function addPage() {
  try {
    const p = await createBroadcastPage({ mode: mode.value, title: '新页面', sort_order: pages.value.length, duration_ms: 10000, bg_color: '#000000', transition: 'fade' })
    await loadPages()
    selPageId.value = p.id
    selItemId.value = null
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '失败') }
}

async function savePage() {
  if (!selPageId.value) return
  try {
    await updateBroadcastPage(selPageId.value, {
      title: editPage.value.title,
      duration_ms: (editPage.value.duration_ms! > 0 ? editPage.value.duration_ms : 10000),
      bg_color: editPage.value.bg_color,
      transition: 'fade',
    })
    await loadPages()
    toast.success('已保存')
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '失败') }
}

async function deletePage() {
  if (!selPageId.value || !confirm('确定删除此页面？')) return
  try {
    await deleteBroadcastPage(selPageId.value)
    selPageId.value = null
    selItemId.value = null
    await loadPages()
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '失败') }
}

// ─── Item CRUD ────────────────────────────────────────────────────────────────
async function addItem(type: 'text' | 'image' | 'clock') {
  if (!selPageId.value) return
  if (type === 'image') {
    const inp = document.createElement('input')
    inp.type = 'file'
    inp.accept = 'image/*'
    inp.onchange = async () => {
      const file = inp.files?.[0]
      if (!file) return
      try {
        const { url } = await uploadBroadcastImage(file)
        await createAndRefresh({ item_type: 'image', content: url, width: 30, height: 18, font_size: '3', font_color: '#ffffff', font_weight: 'normal', text_align: 'center', bg_color: 'transparent', border_radius: '0', animation: '', z_index: 10 })
      } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '上传失败') }
    }
    inp.click()
    return
  }
  const defaults: Partial<BroadcastItem> = type === 'clock'
    ? { content: 'clock', width: 24, height: 10, font_size: '5', font_color: '#ffffff', font_weight: 'bold', text_align: 'center', bg_color: 'transparent', border_radius: '0', animation: '', z_index: 10 }
    : { content: '新建文字\n支持换行', width: 30, height: 12, font_size: '3', font_color: '#ffffff', font_weight: 'normal', text_align: 'center', bg_color: 'transparent', border_radius: '0', animation: '', z_index: 10 }
  await createAndRefresh({ item_type: type, ...defaults })
}

async function createAndRefresh(data: Partial<BroadcastItem>) {
  try {
    const cx = 50 - ((data.width ?? 25) / 2)
    const cy = 50 - ((data.height ?? 10) / 2)
    await createBroadcastItem({ page_id: selPageId.value!, pos_x: cx, pos_y: cy, ...data, extra_json: '{}' })
    await loadPages()
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '失败') }
}

// ─── Item Properties ─────────────────────────────────────────────────────────
const propForm = ref<Partial<BroadcastItem>>({})
const bgTransparent = ref(true)

watch(selItem, (it) => {
  if (!it) return
  propForm.value = { ...it }
  bgTransparent.value = it.bg_color === 'transparent'
})

async function saveProps() {
  const it = selItem.value
  if (!it) return
  const bg = bgTransparent.value ? 'transparent' : (propForm.value.bg_color || '#000000')
  try {
    await updateBroadcastItem(it.id, {
      page_id: it.page_id, item_type: it.item_type,
      content: propForm.value.content ?? it.content,
      pos_x: Number(propForm.value.pos_x), pos_y: Number(propForm.value.pos_y),
      width: Number(propForm.value.width), height: Number(propForm.value.height),
      font_size: propForm.value.font_size, font_color: propForm.value.font_color,
      font_weight: propForm.value.font_weight, text_align: propForm.value.text_align,
      bg_color: bg, border_radius: propForm.value.border_radius || '0',
      animation: propForm.value.animation || '', z_index: it.z_index,
      extra_json: it.extra_json || '{}',
    })
    await loadPages()
    toast.success('已保存')
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '失败') }
}

async function deleteItem() {
  if (!selItemId.value || !confirm('确定删除此元素？')) return
  try {
    await deleteBroadcastItem(selItemId.value)
    selItemId.value = null
    await loadPages()
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '失败') }
}

async function changeZ(id: number, delta: number) {
  const item = findItem(id)
  if (!item) return
  item.z_index = (item.z_index || 10) + delta
  try {
    await updateBroadcastItem(id, { ...item })
    await loadPages()
  } catch {}
}

// ─── Fonts ───────────────────────────────────────────────────────────────────
const fontFileRef = ref<HTMLInputElement>()
const fontName = ref('')

async function uploadFont() {
  const file = fontFileRef.value?.files?.[0]
  if (!file) { toast.error('请选择文件'); return }
  try {
    await uploadBroadcastFont(file, fontName.value)
    fontName.value = ''
    if (fontFileRef.value) fontFileRef.value.value = ''
    await loadAll()
    toast.success('字体上传成功')
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '上传失败') }
}

async function activateFont(filename: string) {
  try {
    await updateBroadcastConfig({ active_font: filename })
    config.value.active_font = filename
    toast.success('字体已激活')
  } catch {}
}

async function deleteFont(id: number) {
  if (!confirm('删除此字体？')) return
  try {
    await deleteBroadcastFont(id)
    await loadAll()
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '失败') }
}

// ─── Config ──────────────────────────────────────────────────────────────────
async function saveCfg(key: 'countdown_target' | 'base_url', val: string) {
  try {
    await updateBroadcastConfig({ [key]: val })
    ;(config.value as Record<string, string>)[key] = val
    toast.success('已保存')
  } catch {}
}

// ─── Push / Preview ──────────────────────────────────────────────────────────
function openPreview() {
  window.open(`/broadcast/${mode.value}`, '_blank')
}

async function pushToDevices() {
  const base = config.value.base_url || 'http://icpc-server.local:8082'
  const url = `${base}/broadcast/${mode.value}`
  const cmd = `full-firefox ${url}`
  if (!confirm(`向目标推送广播？\n命令: ${cmd}`)) return

  const performPush = async () => {
    await updateBroadcastConfig({ sync_reset: mode.value, pushed_state: mode.value })
    await executeCommand({ target_type: 'broadcast', command: cmd })
    await loadAll()
    toast.success('已推送')
  }

  if (config.value.pushed_state) {
    await executeCommand({ target_type: 'broadcast', command: 'full-firefox kill' })
    await new Promise(r => setTimeout(r, 500))
  }
  await performPush()
}

async function killBroadcast() {
  if (!confirm('关闭所有客户端的广播窗口？')) return
  try {
    await updateBroadcastConfig({ pushed_state: '' })
    await executeCommand({ target_type: 'broadcast', command: 'full-firefox kill' })
    await loadAll()
    toast.success('已关闭广播')
  } catch (e: unknown) { toast.error(e instanceof Error ? e.message : '失败') }
}

async function syncReset() {
  if (!confirm('复位同步时钟？所有展示端将从第一页重新开始轮播。')) return
  await updateBroadcastConfig({ sync_reset: mode.value })
  toast.success('已复位')
}

// ─── Canvas item style ────────────────────────────────────────────────────────
function itemStyle(it: BroadcastItem): Record<string, string> {
  const canvasH = canvasRef.value?.offsetHeight || 450
  const fsPx = (parseFloat(it.font_size) || 3) * canvasH / 100
  const alignMap: Record<string, string> = { left: 'flex-start', center: 'center', right: 'flex-end' }
  return {
    position: 'absolute',
    left: it.pos_x + '%',
    top: it.pos_y + '%',
    width: it.width + '%',
    height: it.height + '%',
    fontSize: fsPx + 'px',
    color: it.font_color,
    fontWeight: it.font_weight,
    background: it.bg_color !== 'transparent' ? it.bg_color : 'transparent',
    borderRadius: it.border_radius || '0',
    zIndex: String(it.z_index || 10),
    display: 'flex',
    alignItems: 'center',
    justifyContent: alignMap[it.text_align || 'center'],
    textAlign: it.text_align || 'center',
    cursor: isDragging.value ? 'grabbing' : 'grab',
    boxSizing: 'border-box',
  }
}

const HANDLES = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w']
const handleStyle = (h: string): Record<string, string> => {
  const map: Record<string, Record<string, string>> = {
    nw: { top: '-5px', left: '-5px', cursor: 'nw-resize' },
    n:  { top: '-5px', left: 'calc(50% - 5px)', cursor: 'n-resize' },
    ne: { top: '-5px', right: '-5px', cursor: 'ne-resize' },
    e:  { top: 'calc(50% - 5px)', right: '-5px', cursor: 'e-resize' },
    se: { bottom: '-5px', right: '-5px', cursor: 'se-resize' },
    s:  { bottom: '-5px', left: 'calc(50% - 5px)', cursor: 's-resize' },
    sw: { bottom: '-5px', left: '-5px', cursor: 'sw-resize' },
    w:  { top: 'calc(50% - 5px)', left: '-5px', cursor: 'w-resize' },
  }
  return { ...map[h], position: 'absolute', width: '10px', height: '10px', background: '#2563eb', border: '1.5px solid #fff', borderRadius: '50%', zIndex: '100' }
}

const modeLabels = { before: '赛前', contesting: '赛中', after: '赛后' }
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- Top header -->
    <div class="flex flex-col sm:flex-row sm:items-center gap-3">
      <div class="flex items-center gap-3">
        <h1 class="text-xl font-bold text-slate-900">广播管理</h1>
        <span v-if="config.pushed_state" class="inline-flex px-2 py-0.5 rounded-full text-xs font-bold text-emerald-700 bg-emerald-100">
          已推送: {{ modeLabels[config.pushed_state as keyof typeof modeLabels] }}
        </span>
        <span v-else class="inline-flex px-2 py-0.5 rounded-full text-xs font-bold text-slate-500 bg-slate-100">未推送</span>
      </div>
      <div class="flex gap-2 ml-auto flex-wrap">
        <button class="px-3 py-1.5 text-sm border border-slate-200 rounded-lg hover:bg-slate-50 font-medium" @click="openPreview">预览</button>
        <button class="px-3 py-1.5 text-sm border border-slate-200 rounded-lg hover:bg-slate-50 font-medium" @click="syncReset">复位</button>
        <button class="px-3 py-1.5 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-semibold" @click="pushToDevices">推送</button>
        <button class="px-3 py-1.5 text-sm bg-red-500 hover:bg-red-600 text-white rounded-lg font-semibold" @click="killBroadcast">关闭广播</button>
      </div>
    </div>

    <!-- Mode tabs -->
    <div class="flex gap-1.5">
      <button
        v-for="(label, m) in modeLabels"
        :key="m"
        class="px-4 py-1.5 text-sm rounded-lg font-semibold transition-all"
        :class="mode === m ? 'bg-blue-600 text-white shadow-sm' : 'border border-slate-200 text-slate-600 hover:bg-slate-50'"
        @click="switchMode(m as typeof mode.value)"
      >{{ label }}</button>
    </div>

    <!-- Main editor layout: sidebar | canvas | props -->
    <div class="flex gap-4 min-h-0">

      <!-- ─── Left Sidebar ─────────────────────────────────────────────────── -->
      <div class="w-56 shrink-0 flex flex-col gap-3 overflow-y-auto max-h-[80vh]">

        <!-- Fonts -->
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-3">
          <div class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">字体管理</div>
          <input ref="fontFileRef" type="file" accept=".ttf,.woff,.woff2" class="w-full text-xs mb-1.5" />
          <input v-model="fontName" type="text" placeholder="字体名称(可选)" class="w-full px-2 py-1.5 text-xs border border-slate-200 rounded mb-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500" />
          <button class="w-full py-1.5 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded font-medium" @click="uploadFont">上传字体</button>
          <div v-if="fonts.length" class="mt-2 space-y-1">
            <div v-for="f in fonts" :key="f.id" class="flex items-center justify-between text-xs py-1 border-b border-slate-100 last:border-0">
              <span class="truncate flex-1 text-slate-700">{{ f.name }}</span>
              <div class="flex gap-1 ml-1 shrink-0">
                <span v-if="config.active_font === f.filename" class="px-1 py-0.5 text-emerald-700 bg-emerald-100 rounded text-xs font-bold">激活</span>
                <button v-else class="px-1 py-0.5 border border-slate-200 rounded text-xs hover:bg-slate-50" @click="activateFont(f.filename)">激活</button>
                <button class="px-1 py-0.5 border border-red-200 text-red-500 rounded text-xs hover:bg-red-50" @click="deleteFont(f.id)">删</button>
              </div>
            </div>
          </div>
          <div v-else class="text-xs text-slate-400 mt-2 text-center">暂无字体</div>
        </div>

        <!-- Pages -->
        <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-3">
          <div class="flex items-center justify-between mb-2">
            <div class="text-xs font-bold uppercase tracking-widest text-slate-500">页面列表</div>
            <button class="px-1.5 py-0.5 text-xs bg-blue-600 text-white rounded font-semibold" @click="addPage">+ 添加</button>
          </div>
          <div v-if="!pages.length" class="text-xs text-slate-400 text-center py-3">暂无页面</div>
          <div class="space-y-1">
            <div
              v-for="(p, idx) in pages"
              :key="p.id"
              class="px-2 py-2 rounded-lg cursor-pointer text-xs transition-all"
              :class="selPageId === p.id ? 'bg-blue-50 border border-blue-200 text-blue-800' : 'hover:bg-slate-50 border border-transparent text-slate-700'"
              @click="selPageId = p.id; selItemId = null"
            >
              <div class="font-semibold truncate">{{ idx + 1 }}. {{ p.title || '未命名' }}</div>
              <div class="text-slate-400 text-xs mt-0.5">{{ p.duration_ms / 1000 }}s · {{ (p.items || []).length }} 元素</div>
            </div>
          </div>
        </div>

        <!-- Layer list -->
        <div v-if="selPage" class="bg-white rounded-xl border border-slate-200 shadow-sm p-3">
          <div class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-2">图层列表</div>
          <div v-if="!sortedItems.length" class="text-xs text-slate-400 text-center py-2">暂无元素</div>
          <div class="space-y-1">
            <div
              v-for="it in sortedItems"
              :key="it.id"
              class="flex items-center gap-1 px-2 py-1.5 rounded-lg cursor-pointer text-xs"
              :class="selItemId === it.id ? 'bg-blue-50 border border-blue-200' : 'hover:bg-slate-50 border border-transparent'"
              @click="selItemId = it.id"
            >
              <span class="text-slate-400 text-xs w-8 shrink-0">[Z:{{ it.z_index || 10 }}]</span>
              <span class="flex-1 truncate text-slate-700">{{ it.item_type }}: {{ it.content.substring(0, 10) }}</span>
              <div class="flex gap-0.5 shrink-0">
                <button class="w-5 h-5 text-center border border-slate-200 rounded text-xs hover:bg-slate-100" @click.stop="changeZ(it.id, 1)">+</button>
                <button class="w-5 h-5 text-center border border-slate-200 rounded text-xs hover:bg-slate-100" @click.stop="changeZ(it.id, -1)">-</button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ─── Center Canvas Area ──────────────────────────────────────────── -->
      <div class="flex-1 flex flex-col gap-3 min-w-0">
        <div v-if="!selPage" class="flex-1 bg-white rounded-xl border border-slate-200 shadow-sm flex items-center justify-center text-slate-400 min-h-64">
          请选择左侧页面开始编辑
        </div>

        <template v-else>
          <!-- Page toolbar -->
          <div class="flex flex-wrap items-center gap-2 bg-white rounded-xl border border-slate-200 shadow-sm px-4 py-3">
            <input v-model="editPage.title" type="text" placeholder="页面标题" class="px-2 py-1.5 text-sm border border-slate-200 rounded focus:outline-none focus:ring-1 focus:ring-blue-500 w-28" />
            <label class="text-xs text-slate-500 flex items-center gap-1">
              时长(s)
              <input
                type="number" min="1"
                :value="(editPage.duration_ms ?? selPage.duration_ms) / 1000"
                class="px-2 py-1.5 text-sm border border-slate-200 rounded w-16 focus:outline-none focus:ring-1 focus:ring-blue-500"
                @input="e => editPage.duration_ms = Number((e.target as HTMLInputElement).value) * 1000"
              />
            </label>
            <label class="text-xs text-slate-500 flex items-center gap-1">
              背景
              <input v-model="editPage.bg_color" type="color" class="w-8 h-7 rounded cursor-pointer" />
            </label>
            <button class="px-3 py-1.5 text-xs bg-blue-600 text-white rounded-lg font-semibold" @click="savePage">保存</button>
            <button class="px-3 py-1.5 text-xs bg-red-500 text-white rounded-lg font-semibold" @click="deletePage">删页</button>

            <div class="ml-auto flex gap-1.5">
              <button class="px-3 py-1.5 text-xs border border-slate-200 rounded-lg hover:bg-slate-50 font-medium" @click="addItem('text')">+ 文字</button>
              <button class="px-3 py-1.5 text-xs border border-slate-200 rounded-lg hover:bg-slate-50 font-medium" @click="addItem('image')">+ 图片</button>
              <button class="px-3 py-1.5 text-xs border border-slate-200 rounded-lg hover:bg-slate-50 font-medium" @click="addItem('clock')">+ 时钟</button>
            </div>
          </div>

          <!-- Canvas -->
          <div
            ref="canvasRef"
            class="relative w-full bg-black border border-slate-600 rounded-xl overflow-hidden shadow-2xl select-none"
            style="aspect-ratio: 16/9"
            :style="{ background: selPage.bg_color || '#000' }"
            @pointerdown.self="selItemId = null"
          >
            <!-- Center guides -->
            <div class="absolute inset-x-0 top-1/2 h-px border-t border-dashed border-white/10 pointer-events-none z-0" />
            <div class="absolute inset-y-0 left-1/2 w-px border-l border-dashed border-white/10 pointer-events-none z-0" />

            <!-- Items -->
            <div
              v-for="it in (selPage.items || [])"
              :key="it.id"
              :style="itemStyle(it)"
              :class="['bc-item', selItemId === it.id ? 'outline outline-2 outline-blue-500 outline-offset-0' : '']"
              @pointerdown.stop="startDrag($event, it)"
              @click.stop="selItemId = it.id"
            >
              <!-- Content -->
              <template v-if="it.item_type === 'text'">
                <span style="white-space: pre-wrap; line-height: 1.2; pointer-events: none; width: 100%;">{{ it.content }}</span>
              </template>
              <template v-else-if="it.item_type === 'image'">
                <img :src="it.content" style="width:100%;height:100%;object-fit:contain;pointer-events:none;" draggable="false" />
              </template>
              <template v-else-if="it.item_type === 'clock'">
                <span style="font-family:monospace;white-space:nowrap;pointer-events:none;">{{ clockDisplay }}</span>
              </template>

              <!-- Resize handles (visible only when selected) -->
              <template v-if="selItemId === it.id">
                <div
                  v-for="h in HANDLES"
                  :key="h"
                  class="bc-handle"
                  :style="handleStyle(h)"
                  @pointerdown.stop="startResize($event, it, h)"
                />
              </template>
            </div>
          </div>

          <!-- Canvas info -->
          <div class="text-xs text-slate-400 text-right">
            画布: {{ canvasRef?.offsetWidth ?? '--' }}px × {{ canvasRef?.offsetHeight ?? '--' }}px (16:9) | 字号为视口高度百分比
          </div>

          <!-- Properties panel (only when item selected) -->
          <div v-if="selItem" class="bg-white rounded-xl border border-slate-200 shadow-sm border-t-4 border-t-blue-500 p-4">
            <div class="flex items-center justify-between mb-4">
              <h3 class="font-semibold text-slate-800 text-sm">编辑属性 — {{ selItem.item_type }}</h3>
              <div class="flex gap-2">
                <button class="px-3 py-1.5 text-xs bg-blue-600 text-white rounded-lg font-semibold" @click="saveProps">保存修改</button>
                <button class="px-3 py-1.5 text-xs bg-red-500 text-white rounded-lg font-semibold" @click="deleteItem">删除元素</button>
              </div>
            </div>

            <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3 text-xs">
              <!-- Content -->
              <div class="col-span-full">
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">内容</label>
                <div v-if="selItem.item_type === 'clock'" class="px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-slate-400 text-xs">实时系统时钟 (不可编辑)</div>
                <textarea
                  v-else
                  v-model="propForm.content"
                  rows="2"
                  class="w-full px-3 py-2 border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/30 resize-y"
                />
              </div>

              <!-- Position & Size -->
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">X (%)</label>
                <input v-model.number="propForm.pos_x" type="number" step="0.5" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm" />
              </div>
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">Y (%)</label>
                <input v-model.number="propForm.pos_y" type="number" step="0.5" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm" />
              </div>
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">宽 (%)</label>
                <input v-model.number="propForm.width" type="number" step="0.5" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm" />
              </div>
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">高 (%)</label>
                <input v-model.number="propForm.height" type="number" step="0.5" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm" />
              </div>

              <!-- Typography -->
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">字号 (vh%)</label>
                <input v-model="propForm.font_size" type="text" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm" />
              </div>
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">字体颜色</label>
                <input v-model="propForm.font_color" type="color" class="w-full h-8 rounded-lg cursor-pointer border border-slate-200" />
              </div>
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">字重</label>
                <select v-model="propForm.font_weight" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm bg-white">
                  <option value="normal">普通</option>
                  <option value="bold">加粗</option>
                </select>
              </div>
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">对齐</label>
                <select v-model="propForm.text_align" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm bg-white">
                  <option value="left">左</option>
                  <option value="center">居中</option>
                  <option value="right">右</option>
                </select>
              </div>

              <!-- Background -->
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">背景色</label>
                <div class="flex items-center gap-2">
                  <input v-model="propForm.bg_color" type="color" :disabled="bgTransparent" class="w-10 h-8 rounded cursor-pointer border border-slate-200 disabled:opacity-30" />
                  <label class="flex items-center gap-1 cursor-pointer text-xs text-slate-600">
                    <input v-model="bgTransparent" type="checkbox" class="rounded" />
                    透明
                  </label>
                </div>
              </div>

              <!-- Animation & Border radius -->
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">动画</label>
                <select v-model="propForm.animation" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm bg-white">
                  <option value="">无</option>
                  <option value="fadeIn">淡入</option>
                  <option value="slideUp">上滑</option>
                  <option value="pulse">脉冲</option>
                </select>
              </div>
              <div>
                <label class="block text-slate-500 font-bold uppercase tracking-wide mb-1">圆角</label>
                <input v-model="propForm.border_radius" type="text" placeholder="如 8px" class="w-full px-2 py-1.5 border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-blue-500 text-sm" />
              </div>
            </div>
          </div>

          <div v-else class="text-center text-xs text-slate-400 py-2">在画布或左侧图层列表中点击元素进行编辑</div>
        </template>
      </div>
    </div>

    <!-- Config bar -->
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm px-4 py-3 flex items-center gap-3">
        <span class="text-sm font-medium text-slate-700 whitespace-nowrap">倒计时目标</span>
        <input
          v-model="config.countdown_target"
          type="text"
          placeholder="2026-06-16T14:00:00"
          class="flex-1 px-3 py-1.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30"
        />
        <button class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-lg font-semibold" @click="saveCfg('countdown_target', config.countdown_target)">保存</button>
      </div>
      <div class="bg-white rounded-xl border border-slate-200 shadow-sm px-4 py-3 flex items-center gap-3">
        <span class="text-sm font-medium text-slate-700 whitespace-nowrap">推送地址</span>
        <input
          v-model="config.base_url"
          type="text"
          placeholder="http://icpc-server.local:8082"
          class="flex-1 px-3 py-1.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/30"
        />
        <button class="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-lg font-semibold" @click="saveCfg('base_url', config.base_url)">保存</button>
      </div>
    </div>
  </div>
</template>
