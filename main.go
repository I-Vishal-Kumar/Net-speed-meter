package main

import (
	"embed"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// enablePerMonitorDPI declares the process per-monitor v2 DPI aware so that
// Win32 sizing calls operate in real physical pixels of the target monitor
// instead of being silently virtualized by the system. Without this, a 40-px
// SetWindowPos on a 150% display becomes a 60-px window.
func enablePerMonitorDPI() {
	u32 := syscall.NewLazyDLL("user32.dll")
	proc := u32.NewProc("SetProcessDpiAwarenessContext")
	if proc.Find() != nil {
		return
	}
	const perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	proc.Call(perMonitorAwareV2)
}

func main() {
	enablePerMonitorDPI()
	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "Speedo",
		Width:             86,
		Height:            36,
		Frameless:         true,
		AlwaysOnTop:       true,
		DisableResize:     true,
		StartHidden:       false,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableWindowIcon:                 true,
			DisableFramelessWindowDecorations: true,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
