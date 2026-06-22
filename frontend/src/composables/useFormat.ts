export function useFormat() {
  function formatBytes(bytes: number): string {
    if (!bytes || bytes === 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(1024))
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
  }

  function formatUptime(seconds: number): string {
    if (!seconds || seconds <= 0) return '0分'
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    return hours > 0 ? `${hours}时 ${minutes}分` : `${minutes}分`
  }

  function formatDateTime(str: string): string {
    if (!str) return '-'
    try {
      const d = new Date(str)
      if (isNaN(d.getTime())) return str
      const pad = (n: number) => n < 10 ? '0' + n : String(n)
      return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
    } catch { return str }
  }

  function statusLabel(status: string): string {
    const map: Record<string, string> = {
      pending: '等待中', dispatched: '已派发', running: '运行中',
      completed: '已完成', failed: '失败', timeout: '超时',
    }
    return map[status] || status
  }

  function statusColor(status: string): string {
    const map: Record<string, string> = {
      pending: 'text-slate-500 bg-slate-100',
      dispatched: 'text-blue-700 bg-blue-100',
      running: 'text-amber-700 bg-amber-100',
      completed: 'text-emerald-700 bg-emerald-100',
      failed: 'text-red-700 bg-red-100',
      timeout: 'text-red-700 bg-red-100',
    }
    return map[status] || 'text-slate-500 bg-slate-100'
  }

  function getDeviceIP(localIpJson: string): string | null {
    if (!localIpJson) return null
    try {
      const ips = JSON.parse(localIpJson) as Array<{ ipv4: string; defaultRoute?: boolean }>
      if (!Array.isArray(ips)) return null
      ips.sort((a, b) => ((b.defaultRoute ? 1 : 0) - (a.defaultRoute ? 1 : 0)))
      for (const ip of ips) {
        const addr = ip.ipv4?.split('/')[0]
        if (addr && addr !== '127.0.0.1' && !addr.startsWith('169.254')) return addr
      }
    } catch {}
    return null
  }

  return { formatBytes, formatUptime, formatDateTime, statusLabel, statusColor, getDeviceIP }
}
