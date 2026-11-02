package desktop

import "syscall"

// Frame owns native lifecycle callbacks for one optional GUI session.
type Frame struct {
	Close            func()
	AfterModal       func()
	window, original uintptr
	attached         bool
}

// A single callback plus HWND lookup avoids permanently rooting a Go closure
// for every disposable session. Detach removes the session from the lookup.
var frames = map[uintptr]*Frame{}
var frameCallback = syscall.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
	if f := frames[hwnd]; f != nil {
		return f.frameProc(hwnd, msg, wparam, lparam)
	}
	result, _, _ := procDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return result
})

func NewFrame(hwnd uintptr, close, afterModal func()) *Frame {
	f := &Frame{window: hwnd, Close: close, AfterModal: afterModal}
	f.Attach()
	return f
}

func (t *Frame) Attach() {
	if t.attached {
		return
	}
	frames[t.window] = t
	t.original, _, _ = procSetWindowLong.Call(t.window, gwlpWndProc, frameCallback)
	t.attached = true
}

// Detach restores the GUI's original procedure before the browser destroys it.
func (t *Frame) Detach() {
	if t.attached {
		procSetWindowLong.Call(t.window, gwlpWndProc, t.original)
		delete(frames, t.window)
		t.attached = false
	}
}

func (t *Frame) frameProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmClose:
		if t.Close != nil {
			t.Close()
		}
		return 0
	case wmExitSizeMove:
		if t.AfterModal != nil {
			t.AfterModal()
		}
	}
	result, _, _ := procCallWindowProc.Call(t.original, hwnd, msg, wparam, lparam)
	if msg == wmSize && wparam == sizeMinimized {
		procShowWindow.Call(hwnd, swHide)
	}
	return result
}

func (t *Frame) Show() {
	if t.attached {
		procShowWindow.Call(t.window, swRestore)
		procSetForeground.Call(t.window)
	}
}
