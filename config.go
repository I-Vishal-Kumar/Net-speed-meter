package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config holds persistent user settings.
type Config struct {
	Placement        string `json:"placement"`        // "tray" or "left"
	StartWithWindows bool   `json:"startWithWindows"`
}

// DailyStats tracks cumulative data usage for the current day.
type DailyStats struct {
	Date string `json:"date"` // "2006-01-02"
	Down uint64 `json:"down"`
	Up   uint64 `json:"up"`
}

var (
	cfgMu   sync.RWMutex
	cfgPath string
	dayPath string
)

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	d := filepath.Join(dir, "speedo")
	os.MkdirAll(d, 0o755)
	return d
}

func init() {
	d := configDir()
	cfgPath = filepath.Join(d, "config.json")
	dayPath = filepath.Join(d, "daily.json")
}

func defaultConfig() Config {
	return Config{
		Placement:        "tray",
		StartWithWindows: false,
	}
}

func LoadConfig() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return defaultConfig()
	}
	var c Config
	if json.Unmarshal(data, &c) != nil {
		return defaultConfig()
	}
	if c.Placement == "" {
		c.Placement = "tray"
	}
	return c
}

func SaveConfig(c Config) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0o644)
}

func LoadDailyStats() DailyStats {
	today := time.Now().Format("2006-01-02")
	data, err := os.ReadFile(dayPath)
	if err != nil {
		return DailyStats{Date: today}
	}
	var d DailyStats
	if json.Unmarshal(data, &d) != nil || d.Date != today {
		return DailyStats{Date: today}
	}
	return d
}

func SaveDailyStats(d DailyStats) {
	data, _ := json.MarshalIndent(d, "", "  ")
	os.WriteFile(dayPath, data, 0o644)
}
