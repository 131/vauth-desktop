package main

import (
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/131/vauth-desktop/v2/internal/i18n"
	"github.com/131/vauth-desktop/v2/internal/tray"
	"github.com/131/vauth-desktop/v2/internal/vault"
	"github.com/131/vauth-desktop/v2/ui"
)

// WindowSession owns one GUI and its bridge. Callbacks retain this object,
// not the runtime's next window. Backend operations may finish after their
// session is closed; their replies are then dropped.
type WindowSession struct {
	app    App
	view   *application.WebviewWindow
	closed atomic.Bool
}

// edges maps the UI resize zones to the toolkit's resize messages.
var edges = map[string]string{
	"top": "n-resize", "top-right": "ne-resize", "right": "e-resize", "bottom-right": "se-resize",
	"bottom": "s-resize", "bottom-left": "sw-resize", "left": "w-resize", "top-left": "nw-resize",
}

func NewWindowSession(app *App, wails *application.App, closed func(*WindowSession)) *WindowSession {
	// UI-only callbacks belong to this session; the copied facade shares the
	// resident stores/services and never mutates their lifetime.
	s := &WindowSession{app: *app}
	s.app.Window = s.WindowAction
	s.view = wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: tray.WindowTitle, Width: 920, Height: 600, MinWidth: 600, MinHeight: 380,
		Frameless: true, Hidden: true, InitialPosition: application.WindowCentered,
		Windows:         application.WindowsWindow{DisableFramelessWindowDecorations: true},
		DevToolsEnabled: os.Getenv("VAUTH_DESKTOP_DEBUG") != "",
		HTML:            ui.Page(i18n.Current()),
	})
	// Native minimize (e.g. a system shortcut) follows the same tray contract.
	s.view.OnWindowEvent(events.Common.WindowMinimise, func(*application.WindowEvent) { s.view.Hide() })
	s.view.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		s.closed.Store(true)
		closed(s)
	})
	return s
}

// Message runs one bridge call: {"id", "method", "args"} from app.js.
func (s *WindowSession) Message(message string) {
	if s.closed.Load() {
		return
	}
	var call bridgeCall
	if err := json.Unmarshal([]byte(message), &call); err != nil {
		return
	}
	go func() {
		result, err := s.app.Handle(call.Method, call.Args)
		s.eval("window.vauthDone(" + string(bridgeResponse(call, result, err)) + ")")
	}()
}

func (s *WindowSession) eval(js string) {
	if !s.closed.Load() {
		s.view.ExecJS(js)
	}
}

func (s *WindowSession) WindowAction(action string) error {
	if s.closed.Load() {
		return i18n.Errorf("err.window")
	}
	switch action {
	case "drag":
		application.InvokeSync(func() { s.view.HandleMessage("wails:drag") })
	case "minimize":
		s.view.Hide()
	case "close":
		s.view.Close()
	default:
		edge, ok := edges[action]
		if !ok {
			return i18n.Errorf("err.window")
		}
		application.InvokeSync(func() { s.view.HandleMessage("wails:resize:" + edge) })
	}
	return nil
}

func (s *WindowSession) Close() { s.view.Close() }

func (s *WindowSession) Show() {
	if !s.closed.Load() {
		s.view.Restore()
		s.view.Show()
		s.view.Focus()
	}
}

func (s *WindowSession) Reload() {
	if !s.closed.Load() {
		s.view.SetHTML(ui.Page(i18n.Current()))
	}
}

type bridgeCall struct {
	ID     int               `json:"id"`
	Page   string            `json:"page"`
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

func bridgeResponse(call bridgeCall, result any, err error) []byte {
	message := ""
	if err != nil {
		message = err.Error()
	}
	payload, _ := json.Marshal(map[string]any{"id": call.ID, "page": call.Page, "error": message, "expired": errors.Is(err, vault.ErrSessionExpired), "result": result})
	return payload
}
