import { createRouter, createWebHashHistory } from 'vue-router'
import LoginView from '@/views/LoginView.vue'
import AppLayout from '@/components/AppLayout.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/login', component: LoginView },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', redirect: '/dashboard' },
        { path: 'dashboard', component: () => import('@/views/DashboardView.vue') },
        { path: 'devices', component: () => import('@/views/DevicesView.vue') },
        { path: 'checkin', component: () => import('@/views/CheckinView.vue') },
        { path: 'commands', component: () => import('@/views/CommandsView.vue') },
        { path: 'network', component: () => import('@/views/NetworkView.vue') },
        { path: 'broadcast', component: () => import('@/views/BroadcastView.vue') },
        { path: 'screen', component: () => import('@/views/ScreenView.vue') },
        { path: 'distribute', component: () => import('@/views/DistributeView.vue') },
        { path: 'settings', component: () => import('@/views/SettingsView.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
  ],
})

// Only check auth once per session (not on every navigation)
let authChecked = false
router.beforeEach(async (to) => {
  if (to.path === '/login') return
  if (authChecked) return
  try {
    const res = await fetch('/api/stats')
    if (res.status === 401) return '/login'
    authChecked = true
  } catch {
    return '/login'
  }
})

export default router
