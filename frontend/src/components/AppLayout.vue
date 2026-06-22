<script setup lang="ts">
import { RouterView, useRouter, useRoute } from 'vue-router'
import { onMounted, onUnmounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { logout } from '@/api'
import { useToast } from '@/composables/useToast'

const store = useAppStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()

onMounted(() => store.connect())
onUnmounted(() => store.disconnect())

async function handleLogout() {
  if (!confirm('确定要退出登录吗？')) return
  try {
    await logout()
    router.push('/login')
  } catch {
    router.push('/login')
  }
}

const navItems = [
  { path: '/dashboard', label: '仪表盘', icon: '📊' },
  { path: '/devices', label: '设备管理', icon: '🖥' },
  { path: '/checkin', label: '签到管理', icon: '✅' },
  { path: '/commands', label: '命令执行', icon: '⌨' },
  { path: '/network', label: '网络屏蔽', icon: '🌐' },
  { path: '/broadcast', label: '广播管理', icon: '📺' },
  { path: '/screen', label: '选手屏幕', icon: '👁' },
  { path: '/distribute', label: '文件分发', icon: '📦' },
  { path: '/settings', label: '系统设置', icon: '⚙' },
]

function isActive(path: string) {
  return route.path === path
}
</script>

<template>
  <div class="min-h-screen bg-slate-50 flex flex-col">
    <!-- Top Navbar -->
    <nav class="bg-white border-b border-slate-200 shadow-sm sticky top-0 z-50">
      <div class="px-4 h-14 flex items-center gap-1 max-w-[1600px] mx-auto">
        <!-- Brand -->
        <div class="font-bold text-blue-600 text-base mr-4 whitespace-nowrap shrink-0">
          🏆 ICPC 集控
        </div>

        <!-- Nav Links (scrollable) -->
        <div class="flex items-center gap-0.5 overflow-x-auto flex-1 scrollbar-hide">
          <RouterLink
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            class="px-3 py-1.5 rounded-md text-sm font-medium whitespace-nowrap transition-all duration-150 shrink-0"
            :class="isActive(item.path)
              ? 'bg-blue-50 text-blue-700 font-semibold'
              : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100'"
          >
            <span class="mr-1 text-xs">{{ item.icon }}</span>{{ item.label }}
          </RouterLink>
        </div>

        <!-- Status + Logout -->
        <div class="flex items-center gap-3 ml-2 shrink-0">
          <div class="text-sm text-slate-500 whitespace-nowrap">
            <span class="font-semibold text-emerald-600">{{ store.onlineCount }}</span>
            <span class="text-slate-400 mx-0.5">/</span>
            <span>{{ store.totalCount }}</span>
          </div>
          <div
            class="w-2 h-2 rounded-full"
            :class="store.wsConnected ? 'bg-emerald-400' : 'bg-red-400'"
            :title="store.wsConnected ? 'WebSocket 已连接' : 'WebSocket 断开'"
          />
          <button
            class="text-sm text-red-500 hover:text-white hover:bg-red-500 px-3 py-1.5 rounded-md transition-all font-medium"
            @click="handleLogout"
          >
            退出
          </button>
        </div>
      </div>
    </nav>

    <!-- Page Content -->
    <main class="flex-1 px-6 py-5 max-w-[1600px] mx-auto w-full">
      <Transition name="page" mode="out-in">
        <RouterView :key="route.path" />
      </Transition>
    </main>
  </div>
</template>

<style scoped>
.scrollbar-hide::-webkit-scrollbar { display: none; }
.scrollbar-hide { -ms-overflow-style: none; scrollbar-width: none; }
</style>
