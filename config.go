package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config holds persistent user settings.
type Config struct {
	Placement        string `json:"placement"` // "tray" or "left"
	StartWithWindows bool   `json:"startWithWindows"`
	CalibrateW       int    `json:"calibrateW,omitempty"` // 0 = auto-size from taskbar
	CalibrateH       int    `json:"calibrateH,omitempty"`
	CalibrateX       int    `json:"calibrateX,omitempty"` // 0,0 = auto-dock; any other = explicit position
	CalibrateY       int    `json:"calibrateY,omitempty"`
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
	if err := os.MkdirAll(d, 0o755); err != nil {
		log.Printf("speedo: create config dir %s: %v", d, err)
	}
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
		if !os.IsNotExist(err) {
			log.Printf("speedo: read config %s: %v", cfgPath, err)
		}
		return defaultConfig()
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		log.Printf("speedo: parse config %s: %v", cfgPath, err)
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
		if !os.IsNotExist(err) {
			log.Printf("speedo: read daily stats %s: %v", dayPath, err)
		}
		return DailyStats{Date: today}
	}
	var d DailyStats
	if err := json.Unmarshal(data, &d); err != nil {
		log.Printf("speedo: parse daily stats %s: %v", dayPath, err)
		return DailyStats{Date: today}
	}
	if d.Date != today {
		return DailyStats{Date: today}
	}
	return d
}

func SaveDailyStats(d DailyStats) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dayPath, data, 0o644)
}
