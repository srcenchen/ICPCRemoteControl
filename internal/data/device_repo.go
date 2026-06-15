package data

import (
	"database/sql"
	"fmt"
	"time"

	"ICPCRemoteControl/internal/model"
)

// DeviceRepo handles CRUD operations for the devices table.
type DeviceRepo struct {
	db *sql.DB
}

// NewDeviceRepo creates a new DeviceRepo.
func NewDeviceRepo(db *sql.DB) *DeviceRepo {
	return &DeviceRepo{db: db}
}

// QueryRow exposes the underlying DB for atomic ID allocation.
func (r *DeviceRepo) QueryRow(query string, args ...interface{}) *sql.Row {
	return r.db.QueryRow(query, args...)
}

// Exec exposes the underlying DB for atomic ID allocation.
func (r *DeviceRepo) Exec(query string, args ...interface{}) error {
	_, err := r.db.Exec(query, args...)
	return err
}

// Create inserts a new device record. Returns the device with its DB-assigned ID.
func (r *DeviceRepo) Create(d *model.Device) error {
	now := time.Now().Format(time.RFC3339)
	d.FirstSeen = now
	d.LastSeen = now
	d.UpdatedAt = now

	query := `INSERT INTO devices (
		assigned_id, mac_address, hostname, username, os_name, os_version, os_pretty_name,
		kernel_release, kernel_arch, cpu_model, cpu_physical_cores, cpu_logical_cores,
		cpu_packages, gpu_info, memory_total, memory_used, disk_info, local_ip,
		de_name, wm_name, shell, terminal, display_info, uptime, packages,
		fastfetch_raw, connected, last_seen, first_seen, updated_at
	) VALUES (
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	)`

	result, err := r.db.Exec(query,
		d.AssignedID, d.MacAddress, d.Hostname, d.Username, d.OSName, d.OSVersion, d.OSPrettyName,
		d.KernelRelease, d.KernelArch, d.CPUModel, d.CPUPhysicalCores, d.CPULogicalCores,
		d.CPUPackages, d.GPUInfo, d.MemoryTotal, d.MemoryUsed, d.DiskInfo, d.LocalIP,
		d.DEName, d.WMName, d.Shell, d.Terminal, d.DisplayInfo, d.Uptime, d.Packages,
		d.FastfetchRaw, boolToInt(d.Connected), d.LastSeen, d.FirstSeen, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert device: %w", err)
	}
	id, _ := result.LastInsertId()
	d.ID = id
	return nil
}

// Update updates an existing device record.
func (r *DeviceRepo) Update(d *model.Device) error {
	now := time.Now().Format(time.RFC3339)
	d.UpdatedAt = now

	query := `UPDATE devices SET
		mac_address=?, hostname=?, username=?, os_name=?, os_version=?, os_pretty_name=?,
		kernel_release=?, kernel_arch=?, cpu_model=?, cpu_physical_cores=?, cpu_logical_cores=?,
		cpu_packages=?, gpu_info=?, memory_total=?, memory_used=?, disk_info=?, local_ip=?,
		de_name=?, wm_name=?, shell=?, terminal=?, display_info=?, uptime=?, packages=?,
		fastfetch_raw=?, connected=?, last_seen=?, updated_at=?
	WHERE assigned_id=?`

	_, err := r.db.Exec(query,
		d.MacAddress, d.Hostname, d.Username, d.OSName, d.OSVersion, d.OSPrettyName,
		d.KernelRelease, d.KernelArch, d.CPUModel, d.CPUPhysicalCores, d.CPULogicalCores,
		d.CPUPackages, d.GPUInfo, d.MemoryTotal, d.MemoryUsed, d.DiskInfo, d.LocalIP,
		d.DEName, d.WMName, d.Shell, d.Terminal, d.DisplayInfo, d.Uptime, d.Packages,
		d.FastfetchRaw, boolToInt(d.Connected), d.LastSeen, d.UpdatedAt,
		d.AssignedID,
	)
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}
	return nil
}

// GetByAssignedID retrieves a device by its assigned ID.
func (r *DeviceRepo) GetByAssignedID(assignedID int) (*model.Device, error) {
	query := `SELECT
		id, assigned_id, mac_address, hostname, username, os_name, os_version, os_pretty_name,
		kernel_release, kernel_arch, cpu_model, cpu_physical_cores, cpu_logical_cores,
		cpu_packages, gpu_info, memory_total, memory_used, disk_info, local_ip,
		de_name, wm_name, shell, terminal, display_info, uptime, packages,
		fastfetch_raw, connected, last_seen, first_seen, updated_at
	FROM devices WHERE assigned_id=?`

	d := &model.Device{}
	var connected int
	err := r.db.QueryRow(query, assignedID).Scan(
		&d.ID, &d.AssignedID, &d.MacAddress, &d.Hostname, &d.Username, &d.OSName, &d.OSVersion, &d.OSPrettyName,
		&d.KernelRelease, &d.KernelArch, &d.CPUModel, &d.CPUPhysicalCores, &d.CPULogicalCores,
		&d.CPUPackages, &d.GPUInfo, &d.MemoryTotal, &d.MemoryUsed, &d.DiskInfo, &d.LocalIP,
		&d.DEName, &d.WMName, &d.Shell, &d.Terminal, &d.DisplayInfo, &d.Uptime, &d.Packages,
		&d.FastfetchRaw, &connected, &d.LastSeen, &d.FirstSeen, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get device by assigned_id %d: %w", assignedID, err)
	}
	d.Connected = connected != 0
	return d, nil
}

// GetByMacAddress retrieves a device by its MAC address.
func (r *DeviceRepo) GetByMacAddress(mac string) (*model.Device, error) {
	if mac == "" {
		return nil, sql.ErrNoRows
	}
	query := `SELECT
		id, assigned_id, mac_address, hostname, username, os_name, os_version, os_pretty_name,
		kernel_release, kernel_arch, cpu_model, cpu_physical_cores, cpu_logical_cores,
		cpu_packages, gpu_info, memory_total, memory_used, disk_info, local_ip,
		de_name, wm_name, shell, terminal, display_info, uptime, packages,
		fastfetch_raw, connected, last_seen, first_seen, updated_at
	FROM devices WHERE mac_address=?`

	d := &model.Device{}
	var connected int
	err := r.db.QueryRow(query, mac).Scan(
		&d.ID, &d.AssignedID, &d.MacAddress, &d.Hostname, &d.Username, &d.OSName, &d.OSVersion, &d.OSPrettyName,
		&d.KernelRelease, &d.KernelArch, &d.CPUModel, &d.CPUPhysicalCores, &d.CPULogicalCores,
		&d.CPUPackages, &d.GPUInfo, &d.MemoryTotal, &d.MemoryUsed, &d.DiskInfo, &d.LocalIP,
		&d.DEName, &d.WMName, &d.Shell, &d.Terminal, &d.DisplayInfo, &d.Uptime, &d.Packages,
		&d.FastfetchRaw, &connected, &d.LastSeen, &d.FirstSeen, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get device by mac %s: %w", mac, err)
	}
	d.Connected = connected != 0
	return d, nil
}

// GetAll returns summaries of all devices.
func (r *DeviceRepo) GetAll() ([]model.DeviceSummary, error) {
	query := `SELECT assigned_id, hostname, username, os_name, cpu_model,
		memory_total, connected, last_seen
	FROM devices ORDER BY assigned_id`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get all devices: %w", err)
	}
	defer rows.Close()

	var devices []model.DeviceSummary
	for rows.Next() {
		var d model.DeviceSummary
		var connected int
		if err := rows.Scan(&d.AssignedID, &d.Hostname, &d.Username, &d.OSName,
			&d.CPUModel, &d.MemoryTotal, &connected, &d.LastSeen); err != nil {
			return nil, fmt.Errorf("scan device summary: %w", err)
		}
		d.Connected = connected != 0
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []model.DeviceSummary{}
	}
	return devices, rows.Err()
}

// UpdateConnected sets the connected status and last_seen time for a device.
func (r *DeviceRepo) UpdateConnected(assignedID int, connected bool) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.Exec(
		`UPDATE devices SET connected=?, last_seen=?, updated_at=? WHERE assigned_id=?`,
		boolToInt(connected), now, now, assignedID,
	)
	return err
}

// Delete removes a device record by assigned ID.
func (r *DeviceRepo) Delete(assignedID int) error {
	_, err := r.db.Exec(`DELETE FROM devices WHERE assigned_id=?`, assignedID)
	return err
}

// MarkAllOffline sets all devices to offline. Called on server startup.
func (r *DeviceRepo) MarkAllOffline() error {
	_, err := r.db.Exec(`UPDATE devices SET connected=0`)
	return err
}

// ResetAll deletes all device records and resets the autoincrement counter.
func (r *DeviceRepo) ResetAll() error {
	if _, err := r.db.Exec(`DELETE FROM devices`); err != nil {
		return err
	}
	// Reset the autoincrement so IDs start from 1 again.
	if _, err := r.db.Exec(`DELETE FROM sqlite_sequence WHERE name='devices'`); err != nil {
		// Ignore if it doesn't exist.
		return nil
	}
	return nil
}

// GetNextAssignedID returns the next available assigned ID (MAX + 1).
func (r *DeviceRepo) GetNextAssignedID() (int, error) {
	var maxID sql.NullInt64
	err := r.db.QueryRow(`SELECT MAX(assigned_id) FROM devices`).Scan(&maxID)
	if err != nil {
		return 0, fmt.Errorf("get next assigned id: %w", err)
	}
	if !maxID.Valid {
		return 1, nil
	}
	return int(maxID.Int64) + 1, nil
}

// IsAssignedIDOnline checks if a device with the given assigned_id is currently connected.
func (r *DeviceRepo) IsAssignedIDOnline(assignedID int) (bool, error) {
	var connected int
	err := r.db.QueryRow(`SELECT connected FROM devices WHERE assigned_id=?`, assignedID).Scan(&connected)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return connected != 0, nil
}

// GetStats returns total and online device counts.
func (r *DeviceRepo) GetStats() (total int, online int, err error) {
	err = r.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&total)
	if err != nil {
		return 0, 0, err
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE connected=1`).Scan(&online)
	if err != nil {
		return 0, 0, err
	}
	return total, online, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
