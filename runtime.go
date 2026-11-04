package main

import (
	"errors"
	"github.com/131/vauth-desktop/v2/internal/clipboard"
	"github.com/131/vauth-desktop/v2/internal/i18n"
	"github.com/131/vauth-desktop/v2/internal/tray"
	"github.com/131/vauth-desktop/v2/internal/update"
	"github.com/131/vauth-desktop/v2/winres"
	"github.com/wailsapp/wails/v3/pkg/application"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Runtime owns resident Vault services and one optional disposable GUI session.
type Runtime struct {
	app    *App
	wails  *application.App
	icon   *tray.Tray
	mu     sync.Mutex
	window *WindowSession
}

func NewRuntime(app *App) *Runtime {
	r := &Runtime{app: app}
	data := ""
	if dir, err := os.UserCacheDir(); err == nil {
		data = filepath.Join(dir, "vauth-desktop", "webview")
	}
	r.wails = application.New(application.Options{
		Name: tray.WindowTitle, Icon: winres.Icon,
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "vauth-desktop", OnSecondInstanceLaunch: func(application.SecondInstanceData) { r.OpenWindow() }},
		RawMessageHandler: func(window application.Window, message string, _ *application.OriginInfo) {
			if s := r.session(); s != nil && s.view.ID() == window.ID() {
				s.Message(message)
			}
		},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true, WebviewUserDataPath: data},
		Linux:   application.LinuxOptions{DisableQuitOnLastWindowClosed: true},
		Mac:     application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory, ApplicationShouldTerminateAfterLastWindowClosed: false},
	})
	r.icon = tray.Install(r.wails)
	r.icon.Open, r.icon.Quit = r.OpenWindow, r.wails.Quit
	app.Open = r.wails.Browser.OpenURL
	app.Copy = func(text string) error {
		err := application.InvokeSyncWithError(func() error { return clipboard.Copy(text) })
		if errors.Is(err, errors.ErrUnsupported) {
			err = nil
			if !r.wails.Clipboard.SetText(text) {
				err = i18n.Errorf("err.copy")
			}
		}
		return err
	}
	app.Reload = func() {
		if s := r.session(); s != nil {
			s.Reload()
		}
	}
	app.Updater.Wails = r.wails.Updater
	if err := app.Updater.Init(); err != nil {
		log.Print(err)
	}
	return r
}
func (r *Runtime) Run() error {
	defer r.app.Client.Close()
	go func() {
		if err := update.CleanupLegacy(); err != nil {
			log.Printf("legacy update cleanup: %v", err)
		}
	}()
	return r.wails.Run()
}
func (r *Runtime) session() *WindowSession { r.mu.Lock(); defer r.mu.Unlock(); return r.window }
func (r *Runtime) OpenWindow()             { application.InvokeAsync(r.openWindow) }
func (r *Runtime) openWindow() {
	s := r.session()
	if s == nil {
		s = NewWindowSession(r.app, r.wails, r.closed)
		r.mu.Lock()
		r.window = s
		r.mu.Unlock()
	}
	s.Show()
}
func (r *Runtime) closed(s *WindowSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.window == s {
		r.window = nil
	}
}
