package main

import (
	"context"
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var logOnce sync.Once

type appBarData struct {
	cbSize           uint32
	hWnd             uintptr
	uCallbackMessage uint32
	uEdge            uint32
	rc               rect
	lParam           uintptr
}

type rect struct {
	Left, Top, Right, Bottom int32
}

var (
	shell32                = syscall.NewLazyDLL("shell32.dll")
	user32                 = syscall.NewLazyDLL("user32.dll")
	pSHAppBarMessage       = shell32.NewProc("SHAppBarMessage")
	pFindWindowW           = user32.NewProc("FindWindowW")
	pFindWindowExW         = user32.NewProc("FindWindowExW")
	pGetWindowRect         = user32.NewProc("GetWindowRect")
	pSetWindowPos          = user32.NewProc("SetWindowPos")
	pShowWindow            = user32.NewProc("ShowWindow")
	pGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	pGetWindowLongW        = user32.NewProc("GetWindowLongW")
	pSetWindowLongW        = user32.NewProc("SetWindowLongW")
	pReleaseCapture        = user32.NewProc("ReleaseCapture")
	pSendMessageW          = user32.NewProc("SendMessageW")
	pEnumChildWindows      = user32.NewProc("EnumChildWindows")
	pIsWindowVisible       = user32.NewProc("IsWindowVisible")
	pSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
)

// Enumeration state for findVisibleTrayLeftEdge. Only one enumeration runs
// at a time (serialized via enumLock), so a package-level state + a single
// permanent syscall callback is safe and avoids leaking a new callback per call.
var (
	enumLock sync.Mutex
	enumOut  struct {
		have     bool
		bestLeft int32
		bestRect rect
		minLeft  int32
		tbRight  int32
	}
	enumCallback = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		visible, _, _ := pIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		r := getWindowRect(hwnd)
		if r.Right <= r.Left || r.Bottom <= r.Top {
			return 1
		}
		// Must sit in the right half of the taskbar to count as tray content.
		if r.Left < enumOut.minLeft {
			return 1
		}
		// Must end near the taskbar's right edge — drops huge intermediate
		// containers that span well past the visible icons.
		if enumOut.tbRight-r.Right > 300 {
			return 1
		}
		if !enumOut.have || r.Left < enumOut.bestLeft {
			enumOut.have = true
			enumOut.bestLeft = r.Left
			enumOut.bestRect = r
		}
		return 1
	})
)

func findVisibleTrayLeftEdge(tbHwnd uintptr) (rect, bool) {
	if tbHwnd == 0 {
		return rect{}, false
	}
	enumLock.Lock()
	defer enumLock.Unlock()

	tbRc := getWindowRect(tbHwnd)
	enumOut.have = false
	enumOut.bestLeft = 0
	enumOut.bestRect = rect{}
	enumOut.minLeft = (tbRc.Left + tbRc.Right) / 2
	enumOut.tbRight = tbRc.Right

	pEnumChildWindows.Call(tbHwnd, enumCallback, 0)

	logOnce.Do(func() {
		log.Printf("speedo: visible-tray-enum taskbar=%d..%d  have=%v  bestLeft=%d",
			tbRc.Left, tbRc.Right, enumOut.have, enumOut.bestLeft)
	})

	if enumOut.have {
		return enumOut.bestRect, true
	}
	return rect{}, false
}

const (
	ABM_GETTASKBARPOS = 5
	ABM_GETSTATE      = 4
	ABS_AUTOHIDE      = 1
	HWND_TOPMOST      = ^uintptr(0) // -1
	HWND_TOP          = uintptr(0)
	SWP_NOSIZE        = 0x0001
	SWP_NOACTIVATE    = 0x0010
	SWP_NOMOVE        = 0x0002
	SW_SHOWNOACTIVATE = 8
	SW_HIDE           = 0
	SM_CXSCREEN       = 0
	SM_CYSCREEN       = 1
	GWL_EXSTYLE       = ^uintptr(19) // -20 as uintptr
	WM_NCLBUTTONDOWN  = 0x00A1
	HTCAPTION         = 2
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_APPWINDOW   = 0x00040000
)

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func findTaskbar() uintptr {
	hwnd, _, _ := pFindWindowW.Call(
		uintptr(unsafe.Pointer(utf16Ptr("Shell_TrayWnd"))),
		0,
	)
	return hwnd
}

func getOurHwnd() uintptr {
	hwnd, _, _ := pFindWindowW.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr("Speedo"))),
	)
	return hwnd
}

func getWindowRect(hwnd uintptr) rect {
	var rc rect
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	return rc
}

func getScreenSize() (int, int) {
	cx, _, _ := pGetSystemMetrics.Call(SM_CXSCREEN)
	cy, _, _ := pGetSystemMetrics.Call(SM_CYSCREEN)
	return int(cx), int(cy)
}

// getWorkArea returns the primary monitor's work area (screen minus the
// taskbar and any docked bars). Used during calibrate so our overlay window
// does not cover the taskbar — which would otherwise trigger Windows' auto-
// hide behavior and make tray icons disappear.
const SPI_GETWORKAREA = 48

func getWorkArea() rect {
	var r rect
	pSystemParametersInfoW.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&r)), 0)
	// Fallback: if the call failed the rect is zero — use the full screen.
	if r.Right == 0 && r.Bottom == 0 {
		w, h := getScreenSize()
		return rect{Left: 0, Top: 0, Right: int32(w), Bottom: int32(h)}
	}
	return r
}

func getTaskbarInfo() (r rect, autoHide bool) {
	abd := appBarData{}
	abd.cbSize = uint32(unsafe.Sizeof(abd))

	pSHAppBarMessage.Call(ABM_GETTASKBARPOS, uintptr(unsafe.Pointer(&abd)))
	r = abd.rc

	state, _, _ := pSHAppBarMessage.Call(ABM_GETSTATE, uintptr(unsafe.Pointer(&abd)))
	autoHide = (state & ABS_AUTOHIDE) != 0

	return r, autoHide
}

// startWindowDrag initiates a native window drag via Win32 messages.
func startWindowDrag() {
	hwnd := getOurHwnd()
	if hwnd == 0 {
		return
	}
	pReleaseCapture.Call()
	pSendMessageW.Call(hwnd, WM_NCLBUTTONDOWN, HTCAPTION, 0)
}

func showWindowDirect(hwnd uintptr) {
	pShowWindow.Call(hwnd, SW_SHOWNOACTIVATE)
}

// hideFromTaskbar removes the window from the taskbar by setting WS_EX_TOOLWINDOW
// and removing WS_EX_APPWINDOW extended styles.
func hideFromTaskbar(hwnd uintptr) {
	style, _, _ := pGetWindowLongW.Call(hwnd, GWL_EXSTYLE)
	style = style &^ WS_EX_APPWINDOW // remove app window style
	style = style | WS_EX_TOOLWINDOW // add tool window style
	pSetWindowLongW.Call(hwnd, GWL_EXSTYLE, style)
}

func forceTopmost(hwnd uintptr, x, y int) {
	pSetWindowPos.Call(
		hwnd,
		HWND_TOPMOST,
		uintptr(x), uintptr(y),
		0, 0,
		SWP_NOSIZE|SWP_NOACTIVATE,
	)
}

// findTrayNotifyArea finds the visible tray notification area inside the
// taskbar. On Win11 the TrayNotifyWnd container is much wider than the
// actually-visible icons, so we drill into specific known children
// (SysPager, then TrayClockWClass) and use the leftmost one. That gives us
// a "left edge of visible tray content" suitable for docking against.
func findTrayNotifyArea(tbHwnd uintptr) (rect, bool) {
	if tbHwnd == 0 {
		return rect{}, false
	}

	trayNotify, _, _ := pFindWindowExW.Call(
		tbHwnd, 0,
		uintptr(unsafe.Pointer(utf16Ptr("TrayNotifyWnd"))),
		0,
	)
	if trayNotify == 0 {
		return rect{}, false
	}

	leftmost := rect{Left: 1<<31 - 1}
	have := false

	consider := func(className string) {
		hwnd, _, _ := pFindWindowExW.Call(
			trayNotify, 0,
			uintptr(unsafe.Pointer(utf16Ptr(className))),
			0,
		)
		if hwnd == 0 {
			return
		}
		r := getWindowRect(hwnd)
		// Skip empty/invisible rects.
		if r.Right <= r.Left || r.Bottom <= r.Top {
			return
		}
		if !have || r.Left < leftmost.Left {
			leftmost = r
			have = true
		}
	}

	consider("SysPager")          // visible system-tray icons toolbar
	consider("TrayClockWClass")   // clock
	consider("ToolbarWindow32")   // direct child variant on some builds

	logOnce.Do(func() {
		notifyRc := getWindowRect(trayNotify)
		tbRc := getWindowRect(tbHwnd)
		log.Printf("speedo: taskbar=%d..%d  TrayNotifyWnd=%d..%d  best=%v  leftmost.Left=%d",
			tbRc.Left, tbRc.Right, notifyRc.Left, notifyRc.Right, have, leftmost.Left)
	})

	if have {
		return leftmost, true
	}
	// Fallback: the wide TrayNotifyWnd rect — better than nothing.
	return getWindowRect(trayNotify), true
}

// effectiveWidgetSize returns the calibrated override if set, otherwise the
// auto-computed size derived from the taskbar.
func effectiveWidgetSize(a *App) (int, int) {
	if cw, ch := a.CalibratedSize(); cw > 0 && ch > 0 {
		return cw, ch
	}
	return widgetSize()
}

// widgetSize derives the widget pixel dimensions from the current taskbar.
// Height matches the taskbar's shorter dimension; width is proportional to
// preserve the original 86:36 aspect ratio.
func widgetSize() (w, h int) {
	tbHwnd := findTaskbar()
	if tbHwnd == 0 {
		return 86, 36
	}
	tbRc := getWindowRect(tbHwnd)
	tbH := int(tbRc.Bottom - tbRc.Top)
	tbW := int(tbRc.Right - tbRc.Left)

	// The taskbar's narrow dimension is its "thickness". For a horizontal
	// taskbar that's the height; for a vertical taskbar it's the width — but
	// we still want a horizontal-pill widget, so keep the same orientation.
	thickness := tbH
	if tbW < tbH {
		thickness = tbW
	}
	if thickness < 20 {
		thickness = 36
	}

	h = thickness
	// Width tuned to match native two-line tray items (e.g. the clock):
	// roughly 1.9× the taskbar thickness gives enough room for "999 KB/s".
	w = (h * 19) / 10
	if w < 60 {
		w = 60
	}
	return w, h
}

// dockPosition places the widget on the taskbar. Placement is "tray" (left of
// notification area) or "left" (far left of taskbar, right of Start button).
func dockPosition(winW, winH int, placement string) (int, int) {
	tbHwnd := findTaskbar()
	if tbHwnd == 0 {
		return 0, 0
	}

	tbRc := getWindowRect(tbHwnd)
	tbH := int(tbRc.Bottom - tbRc.Top)
	tbW := int(tbRc.Right - tbRc.Left)

	if tbH < tbW {
		// Horizontal taskbar
		var x int
		// Gap scales with widget thickness so it visually breathes the same on
		// any taskbar size.
		gap := winH
		if gap < 20 {
			gap = 20
		}
		if placement == "left" {
			x = int(tbRc.Left) + gap/2
		} else {
			// "tray" — left of the visible tray icons. Try the enumeration
			// approach first (works on modern Win11 XAML taskbars), fall back
			// to the legacy TrayNotifyWnd lookup, then to a fixed offset.
			if trayRc, ok := findVisibleTrayLeftEdge(tbHwnd); ok {
				x = int(trayRc.Left) - winW - gap
			} else if trayRc, ok := findTrayNotifyArea(tbHwnd); ok {
				x = int(trayRc.Left) - winW - gap
			} else {
				x = int(tbRc.Right) - winW - 200
			}
		}
		y := int(tbRc.Top) + (tbH-winH)/2
		return x, y
	}

	// Vertical taskbar
	x := int(tbRc.Left) + (tbW-winW)/2
	var y int
	if placement == "left" {
		y = int(tbRc.Top) + 48
	} else {
		trayRc, found := findTrayNotifyArea(tbHwnd)
		if found {
			y = int(trayRc.Top) - winH - 4
		} else {
			y = int(tbRc.Bottom) - winH - 100
		}
	}
	return x, y
}

func startTaskbarTracker(ctx context.Context, app *App, _, _ int) {
	// Wait a moment for the Wails window to fully initialize
	select {
	case <-ctx.Done():
		return
	case <-time.After(500 * time.Millisecond):
	}

	ourHwnd := getOurHwnd()

	if ourHwnd != 0 {
		hideFromTaskbar(ourHwnd)
		showWindowDirect(ourHwnd)
	}

	// Start the low-level mouse hook for right-click detection
	startMouseHook(app)

	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// Don't fight the fullscreen watcher — it has hidden the window
		// intentionally and our SetWindowPos calls would not unhide it but
		// would still burn CPU.
		if IsSuppressed() {
			continue
		}

		// While the user is calibrating, let the frontend own size+pos.
		if app.IsCalibrating() {
			continue
		}

		w, h := effectiveWidgetSize(app)
		var x, y int
		if cx, cy := app.CalibratedPosition(); cx != 0 || cy != 0 {
			x, y = cx, cy
		} else {
			placement := app.GetPlacement()
			x, y = dockPosition(w, h, placement)
		}

		if ourHwnd == 0 {
			ourHwnd = getOurHwnd()
		}
		if ourHwnd == 0 {
			continue
		}

		// Always re-apply position+size: the user (or another window) may have
		// moved us. The call is cheap when nothing actually changes.
		pSetWindowPos.Call(
			ourHwnd,
			HWND_TOPMOST,
			uintptr(x), uintptr(y),
			uintptr(w), uintptr(h),
			uintptr(SWP_NOACTIVATE),
		)
	}
}
