package main

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"speedo/monitor"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SpeedEvent is the full data payload pushed to the frontend each tick.
type SpeedEvent struct {
	Download    float64 `json:"Download"`
	Upload      float64 `json:"Upload"`
	Iface       string  `json:"Iface"`
	PeakDown    float64 `json:"PeakDown"`
	PeakUp      float64 `json:"PeakUp"`
	SessionDown uint64  `json:"SessionDown"`
	SessionUp   uint64  `json:"SessionUp"`
	TodayDown   uint64  `json:"TodayDown"`
	TodayUp     uint64  `json:"TodayUp"`
}

type App struct {
	ctx context.Context

	mu        sync.RWMutex
	placement string
	cfg       Config

	// Accumulated stats
	peakDown    float64
	peakUp      float64
	sessionDown uint64
	sessionUp   uint64
	daily       DailyStats
	saveTicker  int // counts ticks for periodic daily save
}

func NewApp() *App {
	cfg := LoadConfig()
	daily := LoadDailyStats()
	return &App{
		placement: cfg.Placement,
		cfg:       cfg,
		daily:     daily,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initial position — tracker will refine size/position from the live taskbar.
	winW, winH := widgetSize()
	x, y := dockPosition(winW, winH, a.GetPlacement())
	runtime.WindowSetPosition(ctx, x, y)
	runtime.WindowSetSize(ctx, winW, winH)

	go a.speedLoop(ctx)
	go startTray(ctx, a)
	go startTaskbarTracker(ctx, a, winW, winH)
	go startFullscreenWatcher(ctx, a)
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	runtime.WindowHide(ctx)
	return true
}

// GetPlacement returns the current dock position.
func (a *App) GetPlacement() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.placement
}

// SetPlacement changes the dock position and persists it.
func (a *App) SetPlacement(p string) {
	if p != "tray" && p != "left" {
		return
	}
	a.mu.Lock()
	a.placement = p
	a.cfg.Placement = p
	cfg := a.cfg
	a.mu.Unlock()
	if err := SaveConfig(cfg); err != nil {
		log.Printf("speedo: save config: %v", err)
	}
}

// ToggleAutoStart enables or disables startup with Windows.
func (a *App) ToggleAutoStart() bool {
	current := isAutoStartEnabled()
	if err := setAutoStart(!current); err != nil {
		log.Printf("speedo: set autostart: %v", err)
	}
	a.mu.Lock()
	a.cfg.StartWithWindows = !current
	cfg := a.cfg
	a.mu.Unlock()
	if err := SaveConfig(cfg); err != nil {
		log.Printf("speedo: save config: %v", err)
	}
	return !current
}

// IsAutoStart returns whether auto-start is enabled.
func (a *App) IsAutoStart() bool {
	return isAutoStartEnabled()
}

// StartDrag initiates window dragging via Win32 message.
func (a *App) StartDrag() {
	startWindowDrag()
}

// ShowContextMenu is called from the frontend on right-click.
func (a *App) ShowContextMenu() {
	a.mu.RLock()
	evt := SpeedEvent{
		PeakDown:    a.peakDown,
		PeakUp:      a.peakUp,
		SessionDown: a.sessionDown,
		SessionUp:   a.sessionUp,
		TodayDown:   a.daily.Down,
		TodayUp:     a.daily.Up,
	}
	placement := a.placement
	a.mu.RUnlock()

	autoStart := isAutoStartEnabled()
	showNativeMenu(a.ctx, a, evt, placement, autoStart)
}

func (a *App) speedLoop(ctx context.Context) {
	prev := monitor.Snapshot()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Persist final daily stats on shutdown.
			a.mu.RLock()
			d := a.daily
			a.mu.RUnlock()
			if err := SaveDailyStats(d); err != nil {
				log.Printf("speedo: save daily stats on shutdown: %v", err)
			}
			return
		case <-ticker.C:
		}

		speed, next := monitor.Poll(prev)
		prev = next

		a.mu.Lock()
		if speed.Valid {
			a.peakDown = math.Max(a.peakDown, speed.Download)
			a.peakUp = math.Max(a.peakUp, speed.Upload)
			a.sessionDown += speed.DeltaRecv
			a.sessionUp += speed.DeltaSent

			today := time.Now().Format("2006-01-02")
			if a.daily.Date != today {
				a.daily = DailyStats{Date: today}
			}
			a.daily.Down += speed.DeltaRecv
			a.daily.Up += speed.DeltaSent
		}

		evt := SpeedEvent{
			Download:    speed.Download,
			Upload:      speed.Upload,
			Iface:       speed.Iface,
			PeakDown:    a.peakDown,
			PeakUp:      a.peakUp,
			SessionDown: a.sessionDown,
			SessionUp:   a.sessionUp,
			TodayDown:   a.daily.Down,
			TodayUp:     a.daily.Up,
		}

		a.saveTicker++
		var snapshot DailyStats
		shouldSave := false
		if a.saveTicker >= 30 {
			a.saveTicker = 0
			snapshot = a.daily
			shouldSave = true
		}
		a.mu.Unlock()

		if shouldSave {
			go func(d DailyStats) {
				if err := SaveDailyStats(d); err != nil {
					log.Printf("speedo: save daily stats: %v", err)
				}
			}(snapshot)
		}

		runtime.EventsEmit(a.ctx, "speed", evt)
	}
}
