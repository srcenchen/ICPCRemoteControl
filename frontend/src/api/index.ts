import type {
  Device, CommandLog, Stats, BroadcastPage, BroadcastItem, BroadcastFont,
  BroadcastConfig, NetworkRule, Settings, Preset, CheckinConfig,
  DistributeFile, DistributeTask, PrecheckResult,
} from '@/types'

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = { method, headers: {} }
  if (body !== undefined) {
    ;(opts.headers as Record<string, string>)['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(path, opts)
  if (res.status === 401) {
    window.location.hash = '#/login'
    throw new Error('Unauthorized')
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

const get = <T>(path: string) => req<T>('GET', path)
const post = <T>(path: string, body?: unknown) => req<T>('POST', path, body)
const put = <T>(path: string, body: unknown) => req<T>('PUT', path, body)
const patch = <T>(path: string, body: unknown) => req<T>('PATCH', path, body)
const del = <T>(path: string) => req<T>('DELETE', path)

// Auth
export const login = (password: string) => post<{ token: string }>('/api/auth/login', { password })
export const logout = () => post<void>('/api/auth/logout')
export const changePassword = (old_password: string, new_password: string) =>
  post<void>('/api/auth/password', { old_password, new_password })

// Stats
export const getStats = () => get<Stats>('/api/stats')

// Devices
export const getDevices = () => get<Device[]>('/api/devices')
export const getDevice = (id: number) => get<Device>(`/api/devices/${id}`)
export const deleteDevice = (id: number) => del<void>(`/api/devices/${id}`)
export const resetDevices = () => post<void>('/api/devices/reset')
export const exportDevices = () => '/api/devices/export'

// Commands
export const executeCommand = (body: { target_type: string; target_id?: number; command: string }) =>
  post<CommandLog>('/api/commands', body)
export const getCommands = (limit = 30) => get<CommandLog[]>(`/api/commands?limit=${limit}`)
export const getCommand = (id: number) => get<CommandLog>(`/api/commands/${id}`)
export const cancelCommand = (id: number) => post<void>(`/api/commands/${id}/cancel`)
export const clearCommands = () => post<void>('/api/commands/clear')
export const getPresets = () => get<Preset[]>('/api/presets')

// Checkin
export const getCheckinList = () => get<Device[]>('/api/checkin')
export const getCheckinStats = () => get<{ total: number; checked_in: number; checked_out: number; not_checked: number }>('/api/checkin/stats')
export const doCheckin = (id: number, student_name: string, student_num: string) =>
  post<void>(`/api/checkin/${id}/checkin`, { student_name, student_num })
export const doCheckout = (id: number) => post<void>(`/api/checkin/${id}/checkout`)
export const restoreCheckout = (id: number) => post<void>(`/api/checkin/${id}/restore`)
export const resetCheckin = (id: number) => post<void>(`/api/checkin/${id}/reset`)
export const swapCheckin = (from_assigned_id: number, to_assigned_id: number) =>
  post<void>('/api/checkin/swap', { from_assigned_id, to_assigned_id })
export const resetAllCheckin = () => post<{ affected_count: number }>('/api/checkin/reset-all')
export const exportCheckin = () => '/api/checkin/export'

// Network
export const getNetworkRules = () => get<NetworkRule[]>('/api/network/rules')
export const updateNetworkRules = (rules: NetworkRule[]) => put<void>('/api/network/rules', rules)
export const applyNetwork = (body: { target_type: string; target_id?: number }) =>
  post<CommandLog>('/api/network/apply', body)
export const removeNetwork = (body: { target_type: string; target_id?: number }) =>
  post<CommandLog>('/api/network/remove', body)

// Broadcast
export const getBroadcastPages = (mode: string) =>
  get<{ pages: BroadcastPage[]; server_time?: string; started_at?: string }>(`/api/broadcast/pages?mode=${mode}`)
export const createBroadcastPage = (data: Partial<BroadcastPage>) =>
  post<BroadcastPage>('/api/broadcast/pages', data)
export const updateBroadcastPage = (id: number, data: Partial<BroadcastPage>) =>
  put<BroadcastPage>(`/api/broadcast/pages/${id}`, data)
export const deleteBroadcastPage = (id: number) => del<void>(`/api/broadcast/pages/${id}`)
export const createBroadcastItem = (data: Partial<BroadcastItem>) =>
  post<BroadcastItem>('/api/broadcast/items', data)
export const updateBroadcastItem = (id: number, data: Partial<BroadcastItem>) =>
  put<BroadcastItem>(`/api/broadcast/items/${id}`, data)
export const updateBroadcastItemPosition = (id: number, data: { pos_x: number; pos_y: number; width: number; height: number }) =>
  patch<void>(`/api/broadcast/items/${id}/position`, data)
export const deleteBroadcastItem = (id: number) => del<void>(`/api/broadcast/items/${id}`)
export const getBroadcastFonts = () => get<BroadcastFont[]>('/api/broadcast/fonts')
export const uploadBroadcastFont = (file: File, name: string) => {
  const fd = new FormData()
  fd.append('file', file)
  if (name) fd.append('name', name)
  return fetch('/api/broadcast/fonts', { method: 'POST', body: fd }).then(r => r.json() as Promise<BroadcastFont>)
}
export const deleteBroadcastFont = (id: number) => del<void>(`/api/broadcast/fonts/${id}`)
export const uploadBroadcastImage = (file: File) => {
  const fd = new FormData()
  fd.append('file', file)
  return fetch('/api/broadcast/images/upload', { method: 'POST', body: fd }).then(r => r.json() as Promise<{ url: string }>)
}
export const getBroadcastConfig = () => get<BroadcastConfig>('/api/broadcast/config')
export const updateBroadcastConfig = (data: Partial<BroadcastConfig> & { sync_reset?: string; pushed_state?: string }) =>
  put<void>('/api/broadcast/config', data)

// Settings
export const getSettings = () => get<Settings>('/api/settings')
export const updateSettings = (data: Partial<Settings>) => post<Settings>('/api/settings', data)
export const getCheckinConfig = () => get<CheckinConfig>('/api/settings/checkin')
export const updateCheckinConfig = (data: CheckinConfig) => put<void>('/api/settings/checkin', data)
export const getSettingsPresets = () => get<Preset[]>('/api/settings/presets')
export const updateSettingsPresets = (presets: Preset[]) => put<Preset[]>('/api/settings/presets', presets)

// Distribution
export const getDistributionStatus = () => get<DistributeTask | null>('/api/distribution/status')
export const getDistributionFiles = () => get<DistributeFile[]>('/api/distribution/files')
export const uploadDistributionFile = (file: File, onProgress?: (pct: number) => void) => {
  return new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/distribution/upload')
    if (onProgress) {
      xhr.upload.addEventListener('progress', e => {
        if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
      })
    }
    xhr.onload = () => (xhr.status < 400 ? resolve() : reject(new Error('上传失败')))
    xhr.onerror = () => reject(new Error('上传失败'))
    const fd = new FormData()
    fd.append('file', file)
    xhr.send(fd)
  })
}
export const deleteDistributionFiles = (filenames: string[]) =>
  post<void>('/api/distribution/delete', { filenames })
export const clearDistributionFiles = () => post<void>('/api/distribution/clear')
export const startDistribution = (data: {
  files: string[]; save_dir: string; target_ids: number[]; server_ip: string; post_cmd: string
}) => post<DistributeTask>('/api/distribution/start', data)
export const stopDistribution = () => post<void>('/api/distribution/stop')
export const retryDeviceDistribution = (device_id: number) =>
  post<void>('/api/distribution/retry', { device_id })
export const precheckDistribution = (data: { server_ip: string; target_ids: number[] }) =>
  post<PrecheckResult[]>('/api/distribution/precheck', data)
export const resetDistributionTask = () => post<void>('/api/distribution/reset')
