export interface Device {
  assigned_id: number
  hostname: string
  username: string
  os_name: string
  os_pretty_name: string
  cpu_model: string
  cpu_physical_cores: number
  cpu_logical_cores: number
  memory_total: number
  memory_used: number
  gpu_info: string
  disk_info: string
  local_ip: string
  kernel_release: string
  kernel_arch: string
  shell: string
  terminal: string
  de_name: string
  wm_name: string
  uptime: number
  mac_address: string
  connected: boolean
  first_seen: string
  last_seen: string
  checkin_status: number
  student_name: string
  student_num: string
  checkin_time: string
  checkout_time: string
}

export interface CommandLog {
  id: number
  created_at: string
  target_type: string
  target_id?: number
  command: string
  status: string
  output?: string
  error_output?: string
  duration_ms?: number
  children?: CommandLog[]
}

export interface Stats {
  total_devices: number
  online_devices: number
  offline_devices: number
  checked_in: number
  total_commands: number
  recent_commands: CommandLog[]
}

export interface BroadcastPage {
  id: number
  mode: string
  title: string
  sort_order: number
  duration_ms: number
  bg_color: string
  transition: string
  items: BroadcastItem[]
}

export interface BroadcastItem {
  id: number
  page_id: number
  item_type: 'text' | 'image' | 'clock'
  content: string
  pos_x: number
  pos_y: number
  width: number
  height: number
  font_size: string
  font_color: string
  font_weight: string
  text_align: string
  bg_color: string
  border_radius: string
  animation: string
  z_index: number
  extra_json: string
}

export interface BroadcastFont {
  id: number
  name: string
  filename: string
  format: string
}

export interface BroadcastConfig {
  active_font: string
  base_url: string
  pushed_state: string
  countdown_target: string
}

export interface NetworkRule {
  type: string
  value: string
}

export interface Preset {
  name: string
  desc: string
  command: string
  color: string
}

export interface CheckinConfig {
  welcome_text: string
  warning_text: string
  post_checkin_msg: string
  post_checkout_cmd: string
  post_checkout_msg: string
}

export interface Settings {
  hostname_prefix: string
  presets: Preset[]
  checkin_config: CheckinConfig
  screen_monitor_enabled: boolean
}

export interface DistributeFile {
  name: string
  size: number
  mod_time: string
}

export interface ClientProgress {
  device_id: number
  hostname: string
  downloaded: number
  total_chunks: number
  percentage: number
  speed_mbps: number
  status: string
  error: string
  updated_at: string
}

export interface DistributeTask {
  id: string
  status: string
  files: string[]
  active_file: string
  active_idx: number
  save_dir: string
  server_ip: string
  post_cmd: string
  progresses: Record<string, ClientProgress>
  suggested_ip: string
}

export interface PrecheckResult {
  device_id: number
  success: boolean
  error: string
}

export type AdminEvent =
  | { event: 'device_connected'; data: { assigned_id: number } }
  | { event: 'device_disconnected'; data: { assigned_id: number } }
  | { event: 'device_updated'; data: unknown }
  | { event: 'checkin_updated'; data: unknown }
  | { event: 'command_status'; data: CommandLog }
  | { event: 'command_output'; data: { command_id: number; device_id: number; stream: string; line: string } }
  | { event: 'command_result'; data: { command_id: number; device_id: number; status: string; error_output: string; duration_ms: number } }
  | { event: 'distribute_progress_update'; data: DistributeTask }
  | { event: 'distribute_task_finished'; data: DistributeTask }
