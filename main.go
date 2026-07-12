package main

import (
	"embed"
	"log"
	"os"
	"runtime"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"

	"palworld-save-relay/internal/logger"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	hideConsole()

	if p := logger.DefaultPath(); p != "" {
		if err := logger.Init(p); err != nil {
			log.Printf("logger init failed (%s): %v", p, err)
		}
	}
	defer logger.Close()
	wd, _ := os.Getwd()
	logger.Infof("=== Palworld 存档转换 starting (pid=%d go=%s os=%s/%s wd=%s) ===",
		os.Getpid(), runtime.Version(), runtime.GOOS, runtime.GOARCH, wd)

	app := application.New(application.Options{
		Name:        "Palworld 存档转换",
		Description: "Palworld co-op save relay",
		Services: []application.Service{
			application.NewService(NewApp()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Palworld 存档转换",
		URL:              "/",
		Width:            960,
		Height:           680,
		BackgroundColour: application.NewRGB(245, 245, 247),
	})
	logger.Info("webview window created; running app loop")
	if err := app.Run(); err != nil {
		logger.Errorf("app.Run failed: %v", err)
		log.Fatal(err)
	}
	logger.Info("=== app exited ===")
}

// hideConsole hides the hosting console window (if any) so the GUI app does not
// show a black cmd window. No-op on non-Windows (this app is Windows-only).
func hideConsole() {
	if runtime.GOOS != "windows" {
		return
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	user32 := syscall.NewLazyDLL("user32.dll")
	showWindow := user32.NewProc("ShowWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	showWindow.Call(hwnd, 0) // SW_HIDE
}
