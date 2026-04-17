package main

import (
	"context"
	goruntime "runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	pSetWinEventHook     = user32.NewProc("SetWinEventHook")
	pUnhookWinEvent      = user32.NewProc("UnhookWinEvent")
	pGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	pGetClassNameW       = user32.NewProc("GetClassNameW")
	pMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	pGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
)

const (
	EVENT_SYSTEM_FOREGROUND  = 0x0003
	WINEVENT_OUTOFCONTEXT    = 0x0000
	MONITOR_DEFAULTTONEAREST = 0x00000002
)

type monitorInfo struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
}

type winMsg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// suppressed is true while we're hidden because a fullscreen app is foreground.
var suppressed atomic.Bool

// IsSuppressed reports whether the widget is currently hidden by a fullscreen app.
// The taskbar tracker checks this to skip repositioning work while invisible.
func IsSuppressed() bool { return suppressed.Load() }

func startFullscreenWatcher(ctx context.Context, app *App) {
	// OOC WinEvent hooks deliver via the calling thread's message queue, so we
	// must lock to an OS thread and pump messages here.
	goruntime.LockOSThread()

	cb := syscall.NewCallback(func(_ /*hHook*/, _ /*event*/, hwnd, _, _, _, _ uintptr) uintptr {
		evaluateForeground(ctx, hwnd)
		return 0
	})

	hook, _, _ := pSetWinEventHook.Call(
		EVENT_SYSTEM_FOREGROUND, EVENT_SYSTEM_FOREGROUND,
		0, cb, 0, 0, WINEVENT_OUTOFCONTEXT,
	)
	if hook == 0 {
		return
	}
	defer pUnhookWinEvent.Call(hook)

	// Initial evaluation in case a fullscreen app is already foreground.
	fg, _, _ := pGetForegroundWindow.Call()
	evaluateForeground(ctx, fg)

	var m winMsg
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func evaluateForeground(ctx context.Context, hwnd uintptr) {
	fs := isFullscreenApp(hwnd)
	if fs && !suppressed.Load() {
		suppressed.Store(true)
		wailsruntime.WindowHide(ctx)
	} else if !fs && suppressed.Load() {
		suppressed.Store(false)
		wailsruntime.WindowShow(ctx)
	}
}

func windowClass(hwnd uintptr) string {
	var buf [128]uint16
	n, _, _ := pGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// isFullscreenApp reports whether the given window is a fullscreen app on its
// monitor. Excludes shell/desktop windows and our own window so we don't hide
// ourselves when the user clicks on the desktop or our widget.
func isFullscreenApp(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}

	cls := windowClass(hwnd)
	switch cls {
	case "Progman", "WorkerW", "Shell_TrayWnd", "NotifyIconOverflowWindow",
		"Windows.UI.Core.CoreWindow", "Speedo":
		return false
	}

	mon, _, _ := pMonitorFromWindow.Call(hwnd, MONITOR_DEFAULTTONEAREST)
	if mon == 0 {
		return false
	}

	mi := monitorInfo{}
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ := pGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return false
	}

	wr := getWindowRect(hwnd)
	// Treat as fullscreen when the window covers the entire monitor.
	return wr.Left <= mi.rcMonitor.Left &&
		wr.Top <= mi.rcMonitor.Top &&
		wr.Right >= mi.rcMonitor.Right &&
		wr.Bottom >= mi.rcMonitor.Bottom
}
