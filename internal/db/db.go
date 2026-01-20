package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Service struct {
	ID             int
	Name           string
	RestartCommand string
	CheckCommand   string
	StatusCommand  string // Used to check if service is running before scheduled restart
	CronSchedule   string
	Enabled        bool
	LastChecked    *time.Time // Pointer to handle NULL
	LastRestarted  *time.Time // Pointer to handle NULL
}

var DB *sql.DB

func InitDB(filepathStr string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filepathStr), 0755); err != nil {
		return err
	}

	var err error
	DB, err = sql.Open("sqlite", filepathStr)
	if err != nil {
		return err
	}

	createTableServices := `
	CREATE TABLE IF NOT EXISTS services (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		restart_command TEXT NOT NULL,
		check_command TEXT NOT NULL,
		status_command TEXT,
		cron_schedule TEXT,
		enabled BOOLEAN DEFAULT 1,
		last_checked DATETIME,
		last_restarted DATETIME
	);
	`
	if _, err := DB.Exec(createTableServices); err != nil {
		return err
	}

	createTableConfig := `
	CREATE TABLE IF NOT EXISTS app_config (
		key TEXT PRIMARY KEY,
		value TEXT
	);
	`
	_, err = DB.Exec(createTableConfig)
	return err
}

func AddService(s Service) error {
	stmt, err := DB.Prepare("INSERT INTO services(name, restart_command, check_command, status_command, cron_schedule, enabled) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(s.Name, s.RestartCommand, s.CheckCommand, s.StatusCommand, s.CronSchedule, s.Enabled)
	return err
}

func ListServices() ([]Service, error) {
	rows, err := DB.Query("SELECT id, name, restart_command, check_command, status_command, cron_schedule, enabled, last_checked, last_restarted FROM services")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var s Service
		err = rows.Scan(&s.ID, &s.Name, &s.RestartCommand, &s.CheckCommand, &s.StatusCommand, &s.CronSchedule, &s.Enabled, &s.LastChecked, &s.LastRestarted)
		if err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, nil
}

func GetService(name string) (*Service, error) {
	var s Service
	err := DB.QueryRow("SELECT id, name, restart_command, check_command, status_command, cron_schedule, enabled, last_checked, last_restarted FROM services WHERE name = ?", name).
		Scan(&s.ID, &s.Name, &s.RestartCommand, &s.CheckCommand, &s.StatusCommand, &s.CronSchedule, &s.Enabled, &s.LastChecked, &s.LastRestarted)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func ToggleService(name string, enable bool) error {
	stmt, err := DB.Prepare("UPDATE services SET enabled = ? WHERE name = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(enable, name)
	return err
}

func RemoveService(name string) error {
	_, err := DB.Exec("DELETE FROM services WHERE name = ?", name)
	return err
}

func UpdateService(s Service) error {
	// We only update mutable fields. ID and Name are identification.
	// Actually Name could be mutable but let's keep it simple for now as ID.
	query := `
		UPDATE services 
		SET restart_command = ?, check_command = ?, status_command = ?, cron_schedule = ?, enabled = ?
		WHERE name = ?
	`
	_, err := DB.Exec(query, s.RestartCommand, s.CheckCommand, s.StatusCommand, s.CronSchedule, s.Enabled, s.Name)
	return err
}

type LogConfig struct {
	MaxSize    int // MB
	MaxBackups int
	MaxAge     int // Days
	Compress   bool
}

func SetLogConfig(cfg LogConfig) error {
	// Upsert keys
	keys := map[string]string{
		"log_max_size":    fmt.Sprintf("%d", cfg.MaxSize),
		"log_max_backups": fmt.Sprintf("%d", cfg.MaxBackups),
		"log_max_age":     fmt.Sprintf("%d", cfg.MaxAge),
		"log_compress":    fmt.Sprintf("%t", cfg.Compress),
	}

	for k, v := range keys {
		_, err := DB.Exec("INSERT OR REPLACE INTO app_config(key, value) VALUES(?, ?)", k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetLogConfig() (*LogConfig, error) {
	rows, err := DB.Query("SELECT key, value FROM app_config WHERE key LIKE 'log_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cfg := &LogConfig{
		MaxSize:    10, // Default 10MB
		MaxBackups: 3,  // Default 3 files
		MaxAge:     28, // Default 28 days
		Compress:   true,
	}

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		switch k {
		case "log_max_size":
			fmt.Sscanf(v, "%d", &cfg.MaxSize)
		case "log_max_backups":
			fmt.Sscanf(v, "%d", &cfg.MaxBackups)
		case "log_max_age":
			fmt.Sscanf(v, "%d", &cfg.MaxAge)
		case "log_compress":
			cfg.Compress = (v == "true")
		}
	}
	return cfg, nil
}

func UpdateLastChecked(id int) error {
	_, err := DB.Exec("UPDATE services SET last_checked = ? WHERE id = ?", time.Now(), id)
	return err
}

func UpdateLastRestarted(id int) error {
	_, err := DB.Exec("UPDATE services SET last_restarted = ? WHERE id = ?", time.Now(), id)
	return err
}

// GetPauseConfig returns the pause_on_active_user setting
func GetPauseConfig() (bool, error) {
	row := DB.QueryRow("SELECT value FROM app_config WHERE key = 'pause_on_active_user'")
	var val string
	if err := row.Scan(&val); err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Default false
		}
		return false, err
	}
	return val == "true", nil
}

// SetPauseConfig updates the pause_on_active_user setting
func SetPauseConfig(enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	_, err := DB.Exec("INSERT OR REPLACE INTO app_config (key, value) VALUES ('pause_on_active_user', ?)", val)
	return err
}
