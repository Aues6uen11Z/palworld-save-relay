//go:build windows

package main

import "syscall"

// hideConsole hides the hosting console window (if any) so the GUI app does not
// show a black cmd window. Works regardless of the -H windowsgui build flag.
func hideConsole() {
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
