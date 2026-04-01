package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	pGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	pSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	pCallNextHookEx      = user32.NewProc("CallNextHookEx")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	pPostMessageW        = user32.NewProc("PostMessageW")
	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
)

const (
	WH_MOUSE_LL    = 14
	WM_RBUTTONDOWN = 0x0204
	WM_APP         = 0x8000
)

type mSLLHOOKSTRUCT struct {
	Pt        point
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

var mouseHookApp *App
var mouseHookHandle uintptr
var hookMsgHwnd uintptr

type wndClassExW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

func mouseHookProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && wParam == WM_RBUTTONDOWN {
		if mouseHookApp != nil {
			ms := (*mSLLHOOKSTRUCT)(unsafe.Pointer(lParam))
			hwnd := getOurHwnd()
			if hwnd != 0 {
				rc := getWindowRect(hwnd)
				if ms.Pt.X >= rc.Left && ms.Pt.X < rc.Right &&
					ms.Pt.Y >= rc.Top && ms.Pt.Y < rc.Bottom {
					pPostMessageW.Call(hookMsgHwnd, WM_APP, 0, 0)
				}
			}
		}
	}
	ret, _, _ := pCallNextHookEx.Call(mouseHookHandle, uintptr(nCode), wParam, lParam)
	return ret
}

func startMouseHook(app *App) {
	mouseHookApp = app

	go func() {
		runtime.LockOSThread()

		hMod, _, _ := pGetModuleHandleW.Call(0)

		// Register a minimal window class for our hidden message window
		className := utf16Ptr("SpeedoHookClass")
		wndProc := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
			if msg == WM_APP {
				mouseHookApp.ShowContextMenu()
				return 0
			}
			ret, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
			return ret
		})
		wc := wndClassExW{
			Size:      uint32(unsafe.Sizeof(wndClassExW{})),
			WndProc:   wndProc,
			Instance:  hMod,
			ClassName: className,
		}
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

		// Create a message-only window (HWND_MESSAGE parent)
		hwnd, _, _ := pCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(utf16Ptr(""))),
			0, 0, 0, 0, 0,
			^uintptr(2), // HWND_MESSAGE = -3 (^2 in unsigned)
			0, hMod, 0,
		)
		hookMsgHwnd = hwnd

		callback := syscall.NewCallback(func(nCode int, wParam uintptr, lParam uintptr) uintptr {
			return mouseHookProc(nCode, wParam, lParam)
		})

		hook, _, err := pSetWindowsHookExW.Call(
			WH_MOUSE_LL,
			callback,
			hMod,
			0,
		)

		if hook == 0 {
			fmt.Println("HOOK FAILED:", err)
			return
		}
		mouseHookHandle = hook
		fmt.Println("HOOK INSTALLED:", hook)

		defer pUnhookWindowsHookEx.Call(hook)

		// Message pump — dispatches hook messages and our WM_APP to the WndProc
		var m msg
		for {
			ret, _, _ := pGetMessageW.Call(
				uintptr(unsafe.Pointer(&m)),
				0, 0, 0,
			)
			if ret == 0 || ret == ^uintptr(0) {
				break
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
}
