package app

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	procSHQueryUserNotificationState = syscall.NewLazyDLL("shell32.dll").NewProc("SHQueryUserNotificationState")
	procGetWindowThreadProcessIdDesk = user32.NewProc("GetWindowThreadProcessId")
)

// fullscreenInFront reports whether another app is full screen in front: a
// game, in a full-screen mode of its own or in a window as large as the
// screen, a video played full screen, or a presentation. Windows says so as
// it holds back its notifications. The app's own full screen is left out.
func fullscreenInFront() bool {
	if procSHQueryUserNotificationState.Find() != nil {
		return false
	}
	var state int32
	if hr, _, _ := procSHQueryUserNotificationState.Call(uintptr(unsafe.Pointer(&state))); hr != 0 {
		return false
	}
	const (
		qunsBusy             = 2 // a full-screen app
		qunsRunningD3DFull   = 3 // a full-screen Direct3D app, a game
		qunsPresentationMode = 4
	)
	if state != qunsBusy && state != qunsRunningD3DFull && state != qunsPresentationMode {
		return false
	}
	if hwnd, _, _ := procGetForegroundWnd.Call(); hwnd != 0 {
		var pid uint32
		procGetWindowThreadProcessIdDesk.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if int(pid) == os.Getpid() {
			return false
		}
	}
	return true
}
