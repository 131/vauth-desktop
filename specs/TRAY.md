# Tray and window

## Application contract

One resident process owns the Vault client, settings, Wails runtime and tray.
The optional GUI owns one WebView/browser session, not those services.
At most one GUI session is created or open at a time. Rapid tray clicks,
Open commands and second launches reuse it. Requests received during native
creation or showing are coalesced; they cannot create another window.

| Action | GUI | Resident services |
| --- | --- | --- |
| Launch | No window/browser | Tray starts |
| Single/double click, Open, second launch | Create a window or show the existing one | Remain active; second process exits |
| Close | Destroy window and browser | Vault session/tray retained |
| Minimize or native minimize | Hide window, retain browser | Remain active |
| Quit | Release window | Exit and close Vault/OIDC state |

The window uses a compact hybrid frame: an HTML header with Minimize,
Maximize/Restore and Close buttons, and Windows-managed border, shadow and
rounded corners when supported by the OS. Wails/WebView2 composition hosting
owns native edge/corner resize and caption hit testing; there are no HTML
resize grips. The header registers its physical-pixel caption/button rectangles
at runtime readiness and after resize/layout changes. This explicit registration
keeps native drag and caption double-click maximize/restore working for inline
HTML, where the default runtime region tracker was insufficient.

Caption right-click opens the native system menu. Maximize/Restore uses Wails'
native window actions; maximize-button hit testing supports the OS Snap UI.
Minimize hides the existing session to the tray. Minimum size remains 600×380;
initial size is 920×600. Closing loses navigation,
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
