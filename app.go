package main

import (
	"context"
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

	// Initial position
	winW, winH := 86, 36
	x, y := dockPosition(winW, winH, a.GetPlacement())
	runtime.WindowSetPosition(ctx, x, y)
	runtime.WindowSetSize(ctx, winW, winH)

	go a.speedLoop()
	go startTray(ctx, a)
	go startTaskbarTracker(ctx, a, winW, winH)
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
	SaveConfig(cfg)
}

// ToggleAutoStart enables or disables startup with Windows.
func (a *App) ToggleAutoStart() bool {
	current := isAutoStartEnabled()
	setAutoStart(!current)
	a.mu.Lock()
	a.cfg.StartWithWindows = !current
	cfg := a.cfg
	a.mu.Unlock()
	SaveConfig(cfg)
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

func (a *App) speedLoop() {
	prev := monitor.Snapshot()
	time.Sleep(time.Second)

	for {
		speed, next := monitor.Poll(prev)
		prev = next

		// Calculate byte deltas for accumulation
		dlBytes := uint64(speed.Download)
		ulBytes := uint64(speed.Upload)

		a.mu.Lock()
		// Peak
		a.peakDown = math.Max(a.peakDown, speed.Download)
		a.peakUp = math.Max(a.peakUp, speed.Upload)
		// Session
		a.sessionDown += dlBytes
		a.sessionUp += ulBytes
		// Daily
		today := time.Now().Format("2006-01-02")
		if a.daily.Date != today {
			// Day rolled over — reset
			a.daily = DailyStats{Date: today}
		}
		a.daily.Down += dlBytes
		a.daily.Up += ulBytes

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

		// Save daily stats every 30 ticks (~30s)
		a.saveTicker++
		if a.saveTicker >= 30 {
			a.saveTicker = 0
			go SaveDailyStats(a.daily)
		}
		a.mu.Unlock()

		runtime.EventsEmit(a.ctx, "speed", evt)
		time.Sleep(time.Second)
	}
}
