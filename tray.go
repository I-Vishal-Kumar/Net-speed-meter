package main

import (
	"context"
	_ "embed"
	goruntime "runtime"

	"github.com/energye/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

func startTray(ctx context.Context, app *App) {
	goruntime.LockOSThread()

	start, _ := systray.RunWithExternalLoop(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("Speedo")
		systray.SetTooltip("Speedo — Network Speed Monitor")

		mShow := systray.AddMenuItem("Show", "Show the speed widget")
		mHide := systray.AddMenuItem("Hide", "Hide the speed widget")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit Speedo")

		mShow.Click(func() {
			wailsruntime.WindowShow(ctx)
		})
		mHide.Click(func() {
			wailsruntime.WindowHide(ctx)
		})
		mQuit.Click(func() {
			systray.Quit()
			wailsruntime.Quit(ctx)
		})
	}, nil)

	start()
}
