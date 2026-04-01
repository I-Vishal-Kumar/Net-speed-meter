package main

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	pCreatePopupMenu    = user32.NewProc("CreatePopupMenu")
	pAppendMenuW        = user32.NewProc("AppendMenuW")
	pTrackPopupMenuEx   = user32.NewProc("TrackPopupMenuEx")
	pDestroyMenu        = user32.NewProc("DestroyMenu")
	pGetCursorPos       = user32.NewProc("GetCursorPos")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

const (
	MF_STRING    = 0x0000
	MF_SEPARATOR = 0x0800
	MF_GRAYED    = 0x0001
	MF_CHECKED   = 0x0008
	TPM_BOTTOMALIGN = 0x0020
	TPM_LEFTALIGN   = 0x0000
	TPM_RETURNCMD   = 0x0100
)

type point struct {
	X, Y int32
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1_000_000_000:
		return fmt.Sprintf("%.1f GB", float64(b)/1_000_000_000)
	case b >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(b)/1_000_000)
	case b >= 1_000:
		return fmt.Sprintf("%.0f KB", float64(b)/1_000)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatSpeedGo(bps float64) string {
	switch {
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1f MB/s", bps/1_000_000)
	case bps >= 1_000:
		return fmt.Sprintf("%.0f KB/s", bps/1_000)
	default:
		return fmt.Sprintf("%.0f B/s", bps)
	}
}

func showNativeMenu(ctx context.Context, app *App, evt SpeedEvent, placement string, autoStart bool) {
	hMenu, _, _ := pCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer pDestroyMenu.Call(hMenu)

	id := uintptr(1)
	appendItem := func(flags uintptr, label string) uintptr {
		thisID := id
		lp, _ := syscall.UTF16PtrFromString(label)
		pAppendMenuW.Call(hMenu, flags, thisID, uintptr(unsafe.Pointer(lp)))
		id++
		return thisID
	}
	appendSep := func() {
		pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
		id++
	}

	// ── Stats (grayed info items) ──
	sessionLabel := fmt.Sprintf("Session:  ▼ %s  ▲ %s", formatBytes(evt.SessionDown), formatBytes(evt.SessionUp))
	todayLabel := fmt.Sprintf("Today:     ▼ %s  ▲ %s", formatBytes(evt.TodayDown), formatBytes(evt.TodayUp))
	peakLabel := fmt.Sprintf("Peak:       ▼ %s  ▲ %s", formatSpeedGo(evt.PeakDown), formatSpeedGo(evt.PeakUp))

	appendItem(MF_STRING|MF_GRAYED, sessionLabel)
	appendItem(MF_STRING|MF_GRAYED, todayLabel)
	appendItem(MF_STRING|MF_GRAYED, peakLabel)
	appendSep()

	// ── Position toggle ──
	var posLeftID, posTrayID uintptr
	leftFlags := uintptr(MF_STRING)
	trayFlags := uintptr(MF_STRING)
	if placement == "left" {
		leftFlags |= MF_CHECKED
	} else {
		trayFlags |= MF_CHECKED
	}
	posLeftID = appendItem(leftFlags, "Position: Far left")
	posTrayID = appendItem(trayFlags, "Position: Near tray")
	appendSep()

	// ── Auto-start ──
	autoFlags := uintptr(MF_STRING)
	if autoStart {
		autoFlags |= MF_CHECKED
	}
	autoID := appendItem(autoFlags, "Start with Windows")
	appendSep()

	// ── Actions ──
	hideID := appendItem(MF_STRING, "Hide")
	quitID := appendItem(MF_STRING, "Quit")

	// Show menu at cursor position
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// Use the hook thread's hidden window as owner — it's on the same
	// thread as the message pump, which TrackPopupMenuEx requires.
	ownerHwnd := hookMsgHwnd
	if ownerHwnd == 0 {
		ownerHwnd = getOurHwnd()
	}
	pSetForegroundWindow.Call(ownerHwnd)

	ret, _, _ := pTrackPopupMenuEx.Call(
		hMenu,
		TPM_BOTTOMALIGN|TPM_LEFTALIGN|TPM_RETURNCMD,
		uintptr(pt.X), uintptr(pt.Y),
		ownerHwnd,
		0,
	)

	switch ret {
	case posLeftID:
		app.SetPlacement("left")
	case posTrayID:
		app.SetPlacement("tray")
	case autoID:
		app.ToggleAutoStart()
	case hideID:
		wailsruntime.WindowHide(ctx)
	case quitID:
		wailsruntime.Quit(ctx)
	}
}
