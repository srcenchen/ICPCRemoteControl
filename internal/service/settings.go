package service

import (
	"encoding/json"
	"sync"
)

// PresetCommand is a named preset command.
type PresetCommand struct {
	Name    string `json:"name"`
	Desc    string `json:"desc"`
	Command string `json:"command"`
	Color   string `json:"color"` // button color hint: danger, warning, success, primary
}

// NetworkRule is a mihomo routing rule.
type NetworkRule struct {
	Type  string `json:"type"`  // DOMAIN, DOMAIN-SUFFIX, DOMAIN-KEYWORD
	Value string `json:"value"` // the domain, suffix, or keyword
}

// CheckinConfig holds contestant-facing check-in page configuration.
type CheckinConfig struct {
	WelcomeText    string `json:"welcome_text"`     // welcome message shown at top of checkin page
	WarningText    string `json:"warning_text"`     // warning shown on checkin form
	PostCheckinMsg string `json:"post_checkin_msg"` // message shown after successful checkin
	PostCheckoutCmd string `json:"post_checkout_cmd"` // command to run after checkout
	PostCheckoutMsg string `json:"post_checkout_msg"` // message shown after successful checkout
}

// ServerSettings holds mutable server configuration that can be changed via the admin UI.
type ServerSettings struct {
	mu             sync.RWMutex
	HostnamePrefix string          `json:"hostname_prefix"`
	Presets        []PresetCommand `json:"presets"`
	NetworkRules   []NetworkRule   `json:"network_rules"`
	CheckinCfg     CheckinConfig   `json:"checkin_config"`
}

// defaultPresets returns the built-in preset commands.
func defaultPresets() []PresetCommand {
	return []PresetCommand{
		{
			Name:    "锁定键鼠",
			Desc:    "禁止所有输入设备（键盘、鼠标）",
			Command: "mkdir -p /etc/udev/rules.d && echo 'ACTION==\"add|change\", SUBSYSTEM==\"input\", ENV{LIBINPUT_IGNORE_DEVICE}=\"1\"' > /etc/udev/rules.d/99-icpc-lock.rules && udevadm control --reload-rules && udevadm trigger --subsystem-match=input && echo '键鼠已锁定'",
			Color:   "danger",
		},
		{
			Name:    "解锁键鼠",
			Desc:    "恢复所有输入设备",
			Command: "rm -f /etc/udev/rules.d/99-icpc-lock.rules && udevadm control --reload-rules && udevadm trigger --subsystem-match=input && echo '键鼠已解锁'",
			Color:   "success",
		},
		{
			Name:    "锁屏",
			Desc:    "锁定屏幕（需要桌面环境支持）",
			Command: "loginctl lock-sessions 2>/dev/null || xdg-screensaver lock 2>/dev/null || echo '无法锁屏'",
			Color:   "warning",
		},
		{
			Name:    "解锁",
			Desc:    "解锁屏幕（需要桌面环境支持）",
			Command: "loginctl unlock-sessions 2>/dev/null || \\\nDISPLAY=:0 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(pgrep -u $(whoami) -x kded5 || echo 1000)/bus qdbus org.freedesktop.ScreenSaver /ScreenSaver SetActive false 2>/dev/null || \\\npkill -9 -f kscreenlocker_greet 2>/dev/null || \\\necho '无法解锁'",
			Color:   "success",
		},
		{
			Name:    "关机",
			Desc:    "立即关闭选手机",
			Command: "shutdown now",
			Color:   "danger",
		},
		{
			Name:    "重启",
			Desc:    "立即重启选手机",
			Command: "reboot",
			Color:   "warning",
		},
		{
			Name:    "同步时间",
			Desc:    "从服务器同步系统时间",
			Command: "timedatectl set-ntp true 2>/dev/null && echo 'NTP 已启用' || echo '时间同步设置失败'",
			Color:   "primary",
		},
	}
}

func defaultNetworkRules() []NetworkRule {
	return []NetworkRule{
		{Type: "DOMAIN-SUFFIX", Value: "baidu.com"},
		{Type: "DOMAIN-SUFFIX", Value: "nowcoder.com"},
		{Type: "DOMAIN-KEYWORD", Value: "nowcoder"},
	}
}

// NewServerSettings creates a new ServerSettings with defaults.
func NewServerSettings(prefix string) *ServerSettings {
	return &ServerSettings{
		HostnamePrefix: prefix,
		Presets:        defaultPresets(),
		NetworkRules:   defaultNetworkRules(),
		CheckinCfg: CheckinConfig{
			WelcomeText:    "欢迎参加XCPC竞赛",
			WarningText:    "严禁场外答题，否则成绩无效！",
			PostCheckinMsg: "签到成功",
			PostCheckoutCmd: "shutdown -h +1",
			PostCheckoutMsg: "签退成功，您的电脑将在1分钟后自动关机。",
		},
	}
}

// GetCheckinConfig returns a copy of the checkin config.
func (s *ServerSettings) GetCheckinConfig() CheckinConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CheckinCfg
}

// SetCheckinConfig updates the checkin config.
func (s *ServerSettings) SetCheckinConfig(cfg CheckinConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CheckinCfg = cfg
}

// GetHostnamePrefix returns the current hostname prefix.
func (s *ServerSettings) GetHostnamePrefix() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HostnamePrefix
}

// SetHostnamePrefix updates the hostname prefix.
func (s *ServerSettings) SetHostnamePrefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HostnamePrefix = prefix
}

// GetPresets returns a copy of the current presets.
func (s *ServerSettings) GetPresets() []PresetCommand {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PresetCommand, len(s.Presets))
	copy(out, s.Presets)
	return out
}

// SetPresets replaces the presets list.
func (s *ServerSettings) SetPresets(presets []PresetCommand) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Presets = presets
}

// GetNetworkRules returns a copy of the current network rules.
func (s *ServerSettings) GetNetworkRules() []NetworkRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]NetworkRule, len(s.NetworkRules))
	copy(out, s.NetworkRules)
	return out
}

// SetNetworkRules replaces the network rules list.
func (s *ServerSettings) SetNetworkRules(rules []NetworkRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NetworkRules = rules
}

// Snapshot returns a copy of current settings for JSON serialization.
func (s *ServerSettings) Snapshot() ServerSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	presets := make([]PresetCommand, len(s.Presets))
	copy(presets, s.Presets)
	rules := make([]NetworkRule, len(s.NetworkRules))
	copy(rules, s.NetworkRules)
	return ServerSettings{HostnamePrefix: s.HostnamePrefix, Presets: presets, NetworkRules: rules, CheckinCfg: s.CheckinCfg}
}

// MarshalPresets serializes presets to JSON bytes.
func (s *ServerSettings) MarshalPresets() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.Presets)
}
