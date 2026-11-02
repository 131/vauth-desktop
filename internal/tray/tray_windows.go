// Package tray owns the resident notification icon and its native menu.
package tray

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/131/vauth-desktop/v2/internal/i18n"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
	procLoadImage       = user32.NewProc("LoadImageW")
	procGetSystemMetric = user32.NewProc("GetSystemMetrics")
	procCreatePopupMenu = user32.NewProc("CreatePopupMenu")
	procAppendMenu      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu  = user32.NewProc("TrackPopupMenu")
	procDestroyMenu     = user32.NewProc("DestroyMenu")
	procGetCursorPos    = user32.NewProc("GetCursorPos")
	procRegisterMessage = user32.NewProc("RegisterWindowMessageW")
	procNotifyIcon      = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmTray          = 0x8000 + 1 // WM_APP + 1
	nimAdd          = 0
	nimDelete       = 2
	nifMessage      = 0x1
	nifIcon         = 0x2
	nifTip          = 0x4
	lrShared        = 0x8000
	imageIcon       = 1
	smCxSmIcon      = 49
	smCySmIcon      = 50
	mfString        = 0
	mfSeparator     = 0x800
	tpmReturnCmd    = 0x100
	tpmRightButton  = 0x2
	menuOpen        = 1
	menuQuit        = 2
	IconID          = 1
)

type notifyIconData struct {
	CbSize          uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUIDItem        windows.GUID
	BalloonIcon     uintptr
}

type Tray struct {
	hwnd           uintptr
	data           notifyIconData
	taskbarCreated uintptr
	AfterModal     func()
	Open           func()
	Quit           func()
	removed        bool
}

const WindowTitle = "vauth desktop"

func registeredMessage(name string) uintptr {
	pointer, _ := windows.UTF16PtrFromString(name)
	message, _, _ := procRegisterMessage.Call(uintptr(unsafe.Pointer(pointer)))
	return message
}

func Install(hwnd uintptr) *Tray {
	t := &Tray{hwnd: hwnd}
	var instance windows.Handle
	windows.GetModuleHandleEx(0, nil, &instance)
	cx, _, _ := procGetSystemMetric.Call(smCxSmIcon)
	cy, _, _ := procGetSystemMetric.Call(smCySmIcon)
	icon, _, _ := procLoadImage.Call(uintptr(instance), IconID, imageIcon, cx, cy, lrShared)
	t.data = notifyIconData{Wnd: hwnd, ID: 1, Flags: nifMessage | nifIcon | nifTip, CallbackMessage: wmTray, Icon: icon}
	t.data.CbSize = uint32(unsafe.Sizeof(t.data))
	copy(t.data.Tip[:len(t.data.Tip)-1], windows.StringToUTF16(WindowTitle))
	t.taskbarCreated = registeredMessage("TaskbarCreated")
	t.add()
	return t
}

func (t *Tray) add() { procNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&t.data))) }

// Remove is idempotent: both host destruction and runtime cleanup may call it.
func (t *Tray) Remove() {
	if !t.removed {
		procNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&t.data)))
		t.removed = true
	}
}

func (t *Tray) restore() {
	if t.Open != nil {
		t.Open()
	}
}

func (t *Tray) menu() {
	menu, _, _ := procCreatePopupMenu.Call()
	defer func() {
		procDestroyMenu.Call(menu)
		if t.AfterModal != nil {
			t.AfterModal()
		}
	}()
	open, _ := windows.UTF16PtrFromString(i18n.T("tray.open"))
	quit, _ := windows.UTF16PtrFromString(i18n.T("tray.quit"))
	procAppendMenu.Call(menu, mfString, menuOpen, uintptr(unsafe.Pointer(open)))
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	procAppendMenu.Call(menu, mfString, menuQuit, uintptr(unsafe.Pointer(quit)))
	var point struct{ X, Y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	// Required so the menu closes when clicking elsewhere.
	procSetForeground.Call(t.hwnd)
	command, _, _ := procTrackPopupMenu.Call(menu, tpmReturnCmd|tpmRightButton, uintptr(point.X), uintptr(point.Y), 0, t.hwnd, 0)
	switch command {
	case menuOpen:
		t.restore()
	case menuQuit:
		if t.Quit != nil {
			t.Quit()
		}
	}
}

// HandleMessage receives icon events and Explorer restart broadcasts from the host.
func (t *Tray) HandleMessage(hwnd, msg, wparam, lparam uintptr) (uintptr, bool) {
	switch {
	case msg == wmTray:
		switch lparam & 0xffff {
		case wmLButtonUp, wmLButtonDblClk:
			t.restore()
		case wmRButtonUp:
			t.menu()
		}
		return 0, true
	case msg == t.taskbarCreated && msg != 0:
		t.add()
		return 0, true
	}
	return 0, false
}
