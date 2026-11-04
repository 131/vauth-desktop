# Tray and window

## Application contract

One resident process owns the Vault client, settings, Wails runtime and tray.
The optional GUI owns one WebView/browser session, not those services.

| Action | GUI | Resident services |
| --- | --- | --- |
| Launch | No window/browser | Tray starts |
| Single/double click, Open, second launch | Create a window or show the existing one | Remain active; second process exits |
| Close | Destroy window and browser | Vault session/tray retained |
| Minimize or native minimize | Hide window, retain browser | Remain active |
| Quit | Release window | Exit and close Vault/OIDC state |

The window uses the standard native Windows frame: title bar, system icon,
Minimize/Maximize/Close controls and resizable edges/corners. There are no HTML
window controls or custom resize zones. Minimum size remains 600×380; initial
size is 920×600. Windows/Wails handle drag, resize, maximize and restore; native
minimize still hides the session to the tray. Closing loses navigation,
modal dialogs and unsaved drafts; saved settings and the resident authenticated
session survive. The new document restores that session through the existing
backend API, subject to expiry.

## Native menu

Single/double click opens the workspace; right-click opens the native, localized
Open / separator / Quit menu. There are no pinned secrets, displayed codes,
owner-draw items or animated menu icons. Menu reconstruction occurs only from
native UI-thread callbacks. Its visual theme is supplied by Windows, not CSS.

The PNG icon and Windows application icon share the existing artwork. No GUI is
created by tray-only startup or startup cleanup. Known Wails initialization-Quit
behavior and native tests are documented in [DEVELOPMENT.md](DEVELOPMENT.md).
