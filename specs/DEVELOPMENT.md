# Development

Read the [development guideline](dev_guideline.md): boring is good.

## Build and tests

```sh
gofmt -l .
go vet ./... && go test -race ./... && GOOS=windows go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-H windowsgui -s -w -X main.version=<version>" -o vauth-desktop.exe .
```

The window uses the [WebView2 runtime](https://developer.microsoft.com/microsoft-edge/webview2/)
(shipped with Windows 10 and 11) through `github.com/jchv/go-webview2`, without
cgo. Vault calls run in Go, off the UI thread. `VAUTH_DESKTOP_DEBUG=1` enables
the developer tools. WebView2 data lives in the user cache directory
(`vauth-desktop/webview`).

## Layout

| Path | |
|------|-|
| `app.go`, `main_windows.go` | UI methods and the window |
| `ui/` | page, style and script, embedded and inlined into one document |
| `internal/i18n/messages.json` | every user-facing string, French and English, shared by Go and `ui/` |
| `internal/vault` | Vault client over vauth-login |
| `internal/update` | channels, download, `SHA256SUMS` and Authenticode checks |
| `internal/tray` | notification area icon, single instance |
| `settings/` | `settings.yml` ([SETTINGS.md](SETTINGS.md)) |
| `winres/` | icon and Windows manifest |
| `specs/` | these documents |

## Translations

`internal/i18n/messages.json` holds `fr-fr` (default) and `en-us`. Go uses
`i18n.T` / `i18n.Errorf`; the page gets the catalog as `window.I18N` and
`{{key}}` placeholders in `ui/index.html`. Tests check that both catalogs have
the same keys and that every key used exists.

## Icon and manifest

The icon (`winres/icon.py`) and the Windows manifest are linked through
`rsrc_windows_amd64.syso`. After changing them, regenerate it:

```sh
(cd winres && python3 icon.py)
go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64
```

## Browser smoke tests

With Puppeteer available in the development module search path:

```sh
node ui/browser_test.cjs
```

The test serves the embedded UI on loopback with synthetic RPC responses. It
covers merged profile search, masked values, JSON/CAS edits, retained drafts,
exact-path ACLs, stale document responses and first enrollment without database
access. The older development Chromium uses a notification-popover shim and
programmatic clicks; this does not replace a native Windows/WebView2 test.
`UI_SCREENSHOT` optionally captures a synthetic workspace preview.

Go tests reject internal operator/organization identifiers in repository content.
Before release, additionally scan the executable as UTF-8 and UTF-16 text, then
verify Windows SSO and an explicitly confirmed public-key enrollment. No release,
version bump or channel promotion happens as part of the development build.

## Native dispatch and update feedback

The tray dispatch queue posts to the HWND so callbacks run inside native modal
loops. Their completion also wakes WebView2 binding callbacks. Update actions
remain disabled with a spinner until download, checksum/signature verification
and relaunch finish. Toasts move to the active dialog host so a backdrop does
not intercept the first action click.
