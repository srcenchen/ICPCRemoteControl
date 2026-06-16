package data

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// NewDB opens (or creates) the SQLite database at the given path and runs migrations.
func NewDB(path string) (*sql.DB, error) {
	// Use WAL mode for better concurrency with multiple readers.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite WAL mode supports concurrent readers. Allow enough connections for
	// HTTP API, WebSocket updates, and TCP client communication to operate in parallel.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// migrate creates tables if they don't exist.
func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS devices (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			assigned_id         INTEGER NOT NULL UNIQUE,
			mac_address         TEXT    NOT NULL DEFAULT '',
			hostname            TEXT    NOT NULL DEFAULT '',
			username            TEXT    NOT NULL DEFAULT '',
			os_name             TEXT    NOT NULL DEFAULT '',
			os_version          TEXT    NOT NULL DEFAULT '',
			os_pretty_name      TEXT    NOT NULL DEFAULT '',
			kernel_release      TEXT    NOT NULL DEFAULT '',
			kernel_arch         TEXT    NOT NULL DEFAULT '',
			cpu_model           TEXT    NOT NULL DEFAULT '',
			cpu_physical_cores  INTEGER NOT NULL DEFAULT 0,
			cpu_logical_cores   INTEGER NOT NULL DEFAULT 0,
			cpu_packages        INTEGER NOT NULL DEFAULT 0,
			gpu_info            TEXT    NOT NULL DEFAULT '[]',
			memory_total        INTEGER NOT NULL DEFAULT 0,
			memory_used         INTEGER NOT NULL DEFAULT 0,
			disk_info           TEXT    NOT NULL DEFAULT '[]',
			local_ip            TEXT    NOT NULL DEFAULT '[]',
			de_name             TEXT    NOT NULL DEFAULT '',
			wm_name             TEXT    NOT NULL DEFAULT '',
			shell               TEXT    NOT NULL DEFAULT '',
			terminal            TEXT    NOT NULL DEFAULT '',
			display_info        TEXT    NOT NULL DEFAULT '[]',
			uptime              INTEGER NOT NULL DEFAULT 0,
			packages            TEXT    NOT NULL DEFAULT '{}',
			fastfetch_raw       TEXT    NOT NULL DEFAULT '[]',
			connected           INTEGER NOT NULL DEFAULT 0,
			last_seen           TEXT    NOT NULL DEFAULT '',
			first_seen          TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at          TEXT    NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_assigned_id ON devices(assigned_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_mac ON devices(mac_address)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_connected ON devices(connected)`,
		// Migration for existing DBs.
		`ALTER TABLE devices ADD COLUMN mac_address TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS command_log (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			parent_id       INTEGER,
			target_type     TEXT    NOT NULL,
			target_id       INTEGER,
			command         TEXT    NOT NULL,
			status          TEXT    NOT NULL DEFAULT 'pending',
			output          TEXT    NOT NULL DEFAULT '',
			error_output    TEXT    NOT NULL DEFAULT '',
			executed_by     TEXT    NOT NULL DEFAULT '',
			created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
			dispatched_at   TEXT,
			completed_at    TEXT,
			duration_ms     INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_command_log_target ON command_log(target_id)`,
		`CREATE INDEX IF NOT EXISTS idx_command_log_status ON command_log(status)`,
		`CREATE INDEX IF NOT EXISTS idx_command_log_created ON command_log(created_at)`,
		// Migration for existing DBs: add parent_id if missing (ignore error if exists).
		`ALTER TABLE command_log ADD COLUMN parent_id INTEGER`,
		// Migration: check-in management fields.
		`ALTER TABLE devices ADD COLUMN checkin_status INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE devices ADD COLUMN student_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE devices ADD COLUMN student_num TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE devices ADD COLUMN checkin_time TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE devices ADD COLUMN checkout_time TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_devices_checkin_status ON devices(checkin_status)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			// Ignore "duplicate column" errors from ALTER TABLE on new databases.
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("exec migration: %w\n%s", err, stmt)
		}
	}
	return nil
}
