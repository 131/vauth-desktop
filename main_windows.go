package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/131/vauth-desktop/v2/internal/i18n"
	"github.com/131/vauth-desktop/v2/internal/tray"
	"github.com/131/vauth-desktop/v2/internal/update"
	"github.com/131/vauth-desktop/v2/internal/vault"
	"github.com/131/vauth-desktop/v2/ui"
	webview "github.com/jchv/go-webview2"
)

func openBrowser(link string) error {
	command := exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.Start()
}

// updatedFlag marks the first start after an update, while the previous
// instance is still closing.
const updatedFlag = "--updated"

func main() {
	if tray.ActivateExisting(slices.Contains(os.Args[1:], updatedFlag)) {
		return
	}
	update.Cleanup()
	app := NewApp(openBrowser)
	defer app.Client.Close()
	data := ""
	if dir, err := os.UserCacheDir(); err == nil {
		data = filepath.Join(dir, "vauth-desktop", "webview")
	}
	w := webview.NewWithOptions(webview.WebViewOptions{
		Debug:     os.Getenv("VAUTH_DESKTOP_DEBUG") != "",
		AutoFocus: true,
		DataPath:  data,
		WindowOptions: webview.WindowOptions{
			Title: tray.WindowTitle, Width: tray.Scale(920), Height: tray.Scale(600), Center: true, IconId: tray.IconID,
		},
	})
	if w == nil {
		os.Exit(1)
	}
	defer w.Destroy()
	w.SetSize(int(tray.Scale(600)), int(tray.Scale(380)), webview.HintMin)
	icon := tray.Install(uintptr(w.Window()))
	icon.AfterModal = func() { w.Dispatch(func() {}) }
	app.Relaunch = func(path string) error {
		command := exec.Command(path, updatedFlag)
		if err := command.Start(); err != nil {
			return err
		}
		command.Process.Release()
		icon.Dispatch(icon.Quit)
		return nil
	}
	// Vault calls run outside the UI thread; results come back through Dispatch.
	w.Bind("vauth_call", func(id int, page string, method string, args []json.RawMessage) {
		go func() {
			result, err := app.Handle(method, args)
			message := ""
			if err != nil {
				message = err.Error()
			}
			payload, _ := json.Marshal(map[string]any{"id": id, "page": page, "error": message, "expired": errors.Is(err, vault.ErrSessionExpired), "result": result})
			icon.Dispatch(func() { w.Eval("window.vauthDone(" + string(payload) + ")") })
		}()
	})
	app.Reload = func() { icon.Dispatch(func() { w.SetHtml(ui.Page(i18n.Current())) }) }
	w.SetHtml(ui.Page(i18n.Current()))
	w.Run()
}
