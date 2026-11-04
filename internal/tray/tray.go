// Package tray builds the native resident icon and Open/Quit menu.
package tray

import (
	"github.com/131/vauth-desktop/v2/internal/i18n"
	"github.com/131/vauth-desktop/v2/winres"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const WindowTitle = "vauth desktop"

type Tray struct {
	Open, Quit func()
	icon       *application.SystemTray
	menu       *application.Menu
}

func Install(app *application.App) *Tray {
	t := &Tray{menu: app.Menu.New()}
	t.icon = app.SystemTray.New().SetIcon(winres.Icon).SetMenu(t.menu)
	t.icon.SetTooltip(WindowTitle)
	t.icon.OnClick(func() { t.Open() })
	t.icon.OnDoubleClick(func() { t.Open() })
	t.icon.OnRightClick(t.showMenu)
	return t
}

// Native callbacks run on the UI thread; no menu mutation crosses goroutines.
func (t *Tray) showMenu() {
	t.menu.Clear()
	t.menu.Add(i18n.T("tray.open")).OnClick(func(*application.Context) { t.Open() })
	t.menu.AddSeparator()
	t.menu.Add(i18n.T("tray.quit")).OnClick(func(*application.Context) { t.Quit() })
	t.menu.Update()
	t.icon.OpenMenu()
}
