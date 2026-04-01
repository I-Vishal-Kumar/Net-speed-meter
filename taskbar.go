package main

import (
	"context"
	"syscall"
	"time"
	"unsafe"
)

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
	shell32           = syscall.NewLazyDLL("shell32.dll")
	user32            = syscall.NewLazyDLL("user32.dll")
	pSHAppBarMessage  = shell32.NewProc("SHAppBarMessage")
	pFindWindowW      = user32.NewProc("FindWindowW")
	pFindWindowExW    = user32.NewProc("FindWindowExW")
	pGetWindowRect    = user32.NewProc("GetWindowRect")
	pSetWindowPos     = user32.NewProc("SetWindowPos")
	pShowWindow       = user32.NewProc("ShowWindow")
	pGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	pGetWindowLongW   = user32.NewProc("GetWindowLongW")
	pSetWindowLongW   = user32.NewProc("SetWindowLongW")
	pReleaseCapture   = user32.NewProc("ReleaseCapture")
	pSendMessageW     = user32.NewProc("SendMessageW")
)

const (
	ABM_GETTASKBARPOS = 5
	ABM_GETSTATE      = 4
	ABS_AUTOHIDE      = 1
	HWND_TOPMOST      = ^uintptr(0) // -1
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

// findTrayNotifyArea finds the notification area (tray icons + clock) inside
// the taskbar. It walks the window hierarchy:
//   Shell_TrayWnd → TrayNotifyWnd  (Win10/11 traditional)
// Falls back to a reasonable offset if not found.
func findTrayNotifyArea(tbHwnd uintptr) (rect, bool) {
	if tbHwnd == 0 {
		return rect{}, false
	}

	// Look for TrayNotifyWnd child
	trayNotify, _, _ := pFindWindowExW.Call(
		tbHwnd, 0,
		uintptr(unsafe.Pointer(utf16Ptr("TrayNotifyWnd"))),
		0,
	)
	if trayNotify != 0 {
		return getWindowRect(trayNotify), true
	}

	return rect{}, false
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
		if placement == "left" {
			x = int(tbRc.Left) + 2
		} else {
			// "tray" — left of notification area
			trayRc, found := findTrayNotifyArea(tbHwnd)
			if found {
				x = int(trayRc.Left) - winW - 16
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

func startTaskbarTracker(_ context.Context, app *App, winW, winH int) {
	// Wait a moment for the Wails window to fully initialize
	time.Sleep(500 * time.Millisecond)

	ourHwnd := getOurHwnd()

	// Hide the "W" icon from the taskbar and force exact window size
	if ourHwnd != 0 {
		hideFromTaskbar(ourHwnd)
		pSetWindowPos.Call(
			ourHwnd,
			HWND_TOPMOST,
			0, 0,
			uintptr(winW), uintptr(winH),
			SWP_NOMOVE|SWP_NOACTIVATE,
		)
		showWindowDirect(ourHwnd)
	}

	// Start the low-level mouse hook for right-click detection
	startMouseHook(app)

	lastX, lastY := -1, -1
	sizeForced := false

	for {
		time.Sleep(16 * time.Millisecond)

		placement := app.GetPlacement()
		x, y := dockPosition(winW, winH, placement)

		if ourHwnd == 0 {
			ourHwnd = getOurHwnd()
		}

		if ourHwnd != 0 {
			flags := uintptr(SWP_NOACTIVATE)
			if sizeForced && x == lastX && y == lastY {
				flags |= SWP_NOMOVE | SWP_NOSIZE
			}
			pSetWindowPos.Call(
				ourHwnd,
				HWND_TOPMOST,
				uintptr(x), uintptr(y),
				uintptr(winW), uintptr(winH),
				flags,
			)
			sizeForced = true
			lastX, lastY = x, y
		}
	}
}
