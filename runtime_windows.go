package main

import (
	"os/exec"

	"github.com/131/vauth-desktop/v2/internal/desktop"
	"github.com/131/vauth-desktop/v2/internal/tray"
)

// Runtime owns the resident services and host. Its only optional
// resource is a disposable GUI session; closing that session does not quit.
type Runtime struct {
	app      *App
	host     *desktop.Host
	icon     *tray.Tray
	window   *WindowSession
	changing bool
}

func NewRuntime(app *App) (*Runtime, error) {
	r := &Runtime{app: app}
	host, err := desktop.New(tray.WindowTitle)
	if err != nil {
		return nil, err
	}
	r.host = host
	r.icon = tray.Install(host.Window())
	r.icon.Open, r.icon.Quit = r.OpenWindow, host.Quit
	r.icon.AfterModal = r.Drain
	host.Open, host.ProcessThread = r.OpenWindow, r.Drain
	host.Message, host.OnDestroy = r.icon.HandleMessage, r.icon.Remove
	app.Reload = func() {
		host.Dispatch(func() {
			if r.window != nil {
				r.window.Reload()
			}
		})
	}
	app.Relaunch = func(path string) error {
		command := exec.Command(path, updatedFlag)
		if err := command.Start(); err != nil {
			return err
		}
		command.Process.Release()
		host.Dispatch(host.Quit)
		return nil
	}
	return r, nil
}

func (r *Runtime) Run() {
	defer r.icon.Remove()
	defer r.app.Client.Close()
	r.host.Run()
	r.CloseWindow()
}

func (r *Runtime) OpenWindow() {
	if r.changing || r.host.Closed() {
		return
	}
	if r.window != nil {
		if err := r.window.Show(); err != nil {
			r.host.Error(err)
		}
		return
	}
	r.changing = true
	defer func() { r.changing = false }()
	window, err := NewWindowSession(r.app, r.host, r.CloseWindow, r.Drain)
	if err != nil {
		if !r.host.Closed() {
			r.host.Error(err)
		}
		return
	}
	if r.host.Closed() {
		window.Close()
		return
	}
	r.window = window
	if err := window.Show(); err != nil {
		r.host.Error(err)
	}
}

func (r *Runtime) CloseWindow() {
	if r.window == nil || r.changing {
		return
	}
	r.changing = true
	defer func() { r.changing = false }()
	window := r.window
	r.window = nil
	if err := window.Close(); err != nil {
		r.window = window
		r.host.Error(err)
	}
}

func (r *Runtime) Drain() {
	if r.window != nil {
		r.window.Drain()
	}
}
