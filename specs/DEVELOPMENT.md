# Development

Read [dev_guideline.md](dev_guideline.md): boring is good.

## Build and tests

Go 1.26 and Wails v3.1 are pinned in `go.mod`. Windows builds do not need cgo:

```sh
gofmt -l .
go vet ./... && go test ./... && GOOS=windows CGO_ENABLED=0 go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-H windowsgui -s -w -X main.version=<version>" -o vauth-desktop.exe .
```

Linux compilation/tests need GTK4/WebKitGTK development libraries
(`libgtk-4-dev libwebkitgtk-6.0-dev`); CI installs them. Only Windows amd64
artifacts are released. Linux/macOS have a shared Wails entrypoint/runtime,
with their desktop behavior, packaging and update trust policy still
unvalidated and unreleased.

## Resident runtime and disposable window

[TRAY.md](TRAY.md) defines lifetime, input and frame behavior. `main.go` creates
one `App` and `Runtime`; Wails owns the UI thread, single-instance lock, native
tray, WebView2/WebKit and hybrid-caption, natively resizable window. `Runtime` retains the Vault client
and optional `WindowSession`. Close destroys that window/browser, not the
Vault session or tray. Quit closes the resident client and its OIDC listener.
The native Minimize control/event hides the existing session.

The UI is inline HTML/CSS/JS, without generated bindings or a frontend build.
It waits for `wails:runtime-config-ready`, posts raw `{id,page,method,args}`
messages, and receives `{id,page,error,expired,result}` through `vauthDone`.
Only the resident runtime's current window can enter the bridge; the page CSP
blocks remote/nested content. Wails' inline-HTML origin flags are not used as
window identity.
Backend work runs off-thread; replies stay bound to the original window and
are dropped after Close. The random page ID rejects replies from an earlier
HTML document after language reload. Vault expiry still reaches the UI through
`expired`; session/auth epochs and retained-draft guards are unchanged.

`VAUTH_DESKTOP_DEBUG=1` enables DevTools. WebView2 data stays in the user cache
under `vauth-desktop/webview`. A known Wails v3.1 limitation remains: Quit during
WebView2 controller initialization can exit with code 1; lifecycle tests report
this explicitly as a warning, not a clean exit.

## Layout

| Path | Responsibility |
| --- | --- |
| `app.go`, `main.go` | Backend API and shared bootstrap |
| `runtime.go`, `window_session.go` | Resident ownership and disposable page-aware bridge |
| `ui/` | Embedded workspace, translations, custom frame and native RPC |
| `internal/vault` | Existing vauth-login, SSO, KV, wrapping and expiry policies |
| `internal/tray` | Native icon and Open/Quit menu |
| `internal/clipboard` | Windows private clipboard and platform fallback |
| `internal/update` | Serialized Wails updater, static channel feed, Authenticode and cleanup |
| `internal/i18n`, `settings/` | Three catalogs and preferences |
| `winres/`, `scripts/` | Icons/manifest and native integration checks |

## Native integration checks

Windows CI ensures WebView2, runs `TestNativeSessionBridge` with
`VAUTH_DESKTOP_NATIVE_TESTS=1`, then `scripts/test-webview-lifecycle.ps1` against
its own test executable. The probe uses synthetic HTML/version RPCs, never
Vault or an agent. The lifecycle script isolates APPDATA/LOCALAPPDATA, disables
update checks and uses an unavailable agent. It verifies tray-only startup,
native caption/edge hit testing, double-click maximize/restore and client-area resize, three close/reopen
cycles with browser process exit, 14-click native tray bursts during creation,
while visible and after minimize (counting hidden and visible GUI HWNDs), second
launch, native minimize/hide with the same browser retained, rapid reopening and
Quit. Cold controller creation and second-launch forwarding use a 90s timeout.
Never point the script at the installed executable; isolate the single-instance
ID with a temporary Go overlay for local tests beside a running installation.

Unit tests force concurrent/reentrant 14-call bursts during creation and showing,
verify session reuse and replacement after Close, and reject publication of an
already-closed construction. They also preserve page IDs/expiry flags and
closed-session guards, serialize
channel changes through download/verification/restart, reject bad digests and
signatures, and exercise bounded `.old` cleanup with a real Windows sharing
lock. See [RELEASES.md](RELEASES.md#windows-update-leftovers).

## Translations and icon

`internal/i18n/messages.json` holds `fr-fr`, `en-us` and `zh-cn`. Tests compare
all three catalogs and every UI key. `winres/icon.png` is the shared tray/app
icon; the Windows icon/manifest are linked through `rsrc_windows_amd64.syso`.
After editing resources:

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
access, text-only wrapping, automatic token copy, separate single-use unwrap, plain
message output, TTL choices, single-line tokens, copied key names, retained
drafts and reconnect clearing. The older development Chromium uses a notification-popover shim and
programmatic clicks; this does not replace a native Windows/WebView2 test.
`UI_SCREENSHOT` optionally captures a synthetic workspace preview. Chinese
screenshot rendering requires a CJK font on the development host; use an
existing system font rather than bundling fonts with the application.

Go tests reject internal operator/organization identifiers in repository content.
Before release, additionally scan the executable as UTF-8 and UTF-16 text, then
verify Windows SSO and an explicitly confirmed public-key enrollment. No release,
version bump or channel promotion happens as part of the development build.

## Hybrid frame integration

`WindowSession` enables frameless mode with Windows decorations retained,
WebView2 composition hosting and non-client region support. Header appearance
is CSS; window movement, border resize and caption/system-menu behavior are
native. `syncCaptionRegions` publishes only the compact header/button rectangles
to the Wails native handler at readiness and after layout/resize. Caption actions
are session-local RPCs to Wails; closed sessions reject them. No Win32 host or
Wails fork is maintained by this application.
