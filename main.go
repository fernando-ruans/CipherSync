package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Browser-extension native-messaging host mode. Browsers launch the
	// manifest path with their own args (origin/extension id) and piped
	// stdin, so detect that in addition to the explicit flag.
	if isNativeHostInvocation() {
		runNativeHost()
		return
	}

	// Create an instance of the app structure
	app := NewApp()

	// System tray runs isolated: both Wails and systray want the main OS thread.
	go runTray(app)

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "CipherSync",
		Width:     1200,
		Height:    800,
		MinWidth:  940,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 17, B: 26, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		// X button: hide to tray (if enabled in settings) instead of quitting.
		// The tray menu always offers Sair to really quit.
		OnBeforeClose: func(ctx context.Context) bool {
			if app.closeToTray() {
				runtime.WindowHide(ctx)
				return true
			}
			return false
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "ciphersync-single-instance",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				runtime.WindowShow(app.ctx)
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
