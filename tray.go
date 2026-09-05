package main

import (
	_ "embed"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// runTray starts the system tray icon in an isolated goroutine.
// Must be called with `go runTray(app)` before wails.Run because both
// Wails (winc) and systray want the main OS thread on Windows.
func runTray(app *App) {
	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("CipherSync")
		systray.SetTooltip("CipherSync - Gerenciador de senhas")

		mShow := systray.AddMenuItem("Mostrar", "Mostrar a janela do CipherSync")
		mLock := systray.AddMenuItem("Bloquear cofre", "Bloqueia o cofre atualmente aberto")
		mGen := systray.AddMenuItem("Gerar senha", "Gera uma senha forte e copia para a área de transferência")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Sair", "Encerra o CipherSync")

		mShow.Click(func() {
			if app.ctx != nil {
				runtime.WindowShow(app.ctx)
			}
		})
		mLock.Click(func() {
			app.Lock()
		})
		mGen.Click(func() {
			pw, err := generatePassword(PasswordOptions{
				Length:     20,
				UseUpper:   true,
				UseLower:   true,
				UseDigits:  true,
				UseSymbols: true,
			})
			if err == nil {
				_ = app.CopyToClipboard(pw)
			}
		})
		mQuit.Click(func() {
			systray.Quit()
			if app.ctx != nil {
				runtime.Quit(app.ctx)
			}
		})
	}, func() {})
}
