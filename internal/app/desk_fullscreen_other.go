//go:build !windows

package app

// fullscreenInFront reports whether another app is full screen in front:
// asked of Windows only.
func fullscreenInFront() bool { return false }
