package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/131/vauth-desktop/v2/internal/desktop"
	"github.com/131/vauth-desktop/v2/internal/i18n"
	"github.com/131/vauth-desktop/v2/internal/tray"
	"github.com/131/vauth-desktop/v2/internal/vault"
	"github.com/131/vauth-desktop/v2/ui"
	webview "github.com/jchv/go-webview2"
)

// WindowSession owns one GUI, frame and bridge. Callbacks retain this object,
// not the runtime's next window, and dispatch checks its closed state on the
// UI thread. Backend operations may finish after their session is closed.
type WindowSession struct {
	host   *desktop.Host
	view   webview.WebView
	frame  *desktop.Frame
	closed bool
}

func NewWindowSession(app *App, host *desktop.Host, close, afterModal func()) (*WindowSession, error) {
	data := ""
	if dir, err := os.UserCacheDir(); err == nil {
		data = filepath.Join(dir, "vauth-desktop", "webview")
	}
	view := webview.NewWithOptions(webview.WebViewOptions{
		Debug: os.Getenv("VAUTH_DESKTOP_DEBUG") != "", AutoFocus: true, DataPath: data,
		WindowOptions: webview.WindowOptions{Hidden: true, Title: tray.WindowTitle, Width: desktop.Scale(920), Height: desktop.Scale(600), Center: true, IconId: tray.IconID},
	})
	if view == nil {
		return nil, i18n.Errorf("err.window")
	}
	s := &WindowSession{host: host, view: view}
	view.SetSize(int(desktop.Scale(600)), int(desktop.Scale(380)), webview.HintMin)
	s.frame = desktop.NewFrame(uintptr(view.Window()), close, afterModal)
	if err := view.Bind("vauth_call", func(id int, page string, method string, args []json.RawMessage) {
		go func() {
			result, err := app.Handle(method, args)
			message := ""
			if err != nil {
				message = err.Error()
			}
			payload, _ := json.Marshal(map[string]any{"id": id, "page": page, "error": message, "expired": errors.Is(err, vault.ErrSessionExpired), "result": result})
			s.dispatch(func() { s.view.Eval("window.vauthDone(" + string(payload) + ")") })
		}()
	}); err != nil {
		s.Close()
		return nil, err
	}
	s.Reload()
	return s, nil
}

func (s *WindowSession) dispatch(f func()) {
	s.host.Dispatch(func() {
		if !s.closed {
			f()
		}
	})
}

func (s *WindowSession) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	s.frame.Detach()
	if err := s.view.Close(); err != nil {
		s.closed = false
		s.frame.Attach()
		return err
	}
	return nil
}

func (s *WindowSession) Show() error {
	if s.closed {
		return nil
	}
	// Showing the parent HWND does not change the visibility of a controller
	// created under a hidden parent. Publish the browser as well as its frame.
	s.frame.Show()
	return s.view.Show()
}
func (s *WindowSession) Drain() {
	if !s.closed {
		s.view.ProcessDispatch()
	}
}
func (s *WindowSession) Reload() {
	if !s.closed {
		s.view.SetHtml(ui.Page(i18n.Current()))
	}
}
