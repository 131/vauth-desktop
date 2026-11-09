# Workspace — LOCAL specialization

## Defining sources

The shared UI owns the functional workspace contract:
[vauth-ui Vault UX](https://github.com/131/vauth-ui/blob/main/specs/VAULT.md)
and [one-time secret sharing](https://github.com/131/vauth-ui/blob/main/specs/TOKEN_WRAP.md).
Those sources define section order and labels, merged environment data,
KV discovery/navigation, JSON types, CAS, ACLs, editing and shared state handling.
This document specializes that contract for the native desktop; it is not a
second functional spec or translation table.

## Native shell and navigation

The shared workspace navigation and KV folder sidebar stay embedded in the
disposable WebView. KV browsing/editing does not open a remote Vault UI or
create a token-bearing browser link. Authentication and release links use the
system browser through Go-owned actions instead.

The native caption and tray determine window lifetime, not the shared shell.
Initial size is 920×600, minimum size 600×380; native resize, caption actions,
Open/Close/Minimize/Quit and single-instance behavior are defined in
[TRAY.md](TRAY.md). Native typography, selection and caption clearance live in
`src/desktop.css`. Browser zoom/reload shortcuts and non-input context menus
and file drops are suppressed by `ui/app.js`; text fields retain selection.
Language changes reload the whole document, as defined in
[SETTINGS.md](SETTINGS.md): page-only navigation/drafts are lost while the
resident Go session survives, rather than updating a browser-persisted session.

## Disconnected screen and native SSH enrollment

The disconnected screen offers SSH-agent login and **Login with SSO**, even
without an authorized agent key or reachable agent. Disconnected does not mean
network-free authentication. The SSO action opens the system browser and uses
the temporary localhost callback described in
[AUTHENTICATION.md](AUTHENTICATION.md); it does not embed the provider login.

After SSO, the native controller selects **SSH Keys** for first enrollment.
Normal SSH login selects the environment section. An unavailable personal
database must not block profile-based enrollment. Agent-list failure leaves
manual public-key entry available; publication errors do not silently switch
authentication methods. The explicit **Continue with SSH** action resumes the
CLI-compatible SSH login/cache flow only after the user's choice.

`agent_keys` lists public keys from the local SSH agent/Pageant, with comments
and SHA-256 fingerprints. Rows mark keys already authorized on the role;
**Use key** fills and focuses the public-key input without publishing it.
Publication uses the selected/pasted public key and confirmation. Native role
resolution and RPC validation are in [VAULT.md](VAULT.md#native-go-adapter);
challenge, token lifecycle and first-enrollment mechanics remain in
[AUTHENTICATION.md](AUTHENTICATION.md).

## Native profile visibility exception

In `ui/app.js`, profile rows render with `secret: !isFixed`: actual identity
metadata is visible by default, selectable and copyable. Identity's web
consumer masks it by default. This is an existing native exception, not a new
UX behavior. Profile rows keep the read-only badge and no Add/Edit/Delete
controls; native Go writes also reject profile collisions. Personal values
use the shared masking/editing contract.

Native environment refresh obtains the current resident session/profile through
the `session` RPC and rereads the personal dictionary, preserving the search.

The native environment view does not resolve project `.vauthrc` macros or
launch a shell. Configuration and data access stay in the
[Go adapter](VAULT.md); common merge/search/CRUD requirements stay in the
[shared Vault spec](https://github.com/131/vauth-ui/blob/main/specs/VAULT.md).

## Native clipboard and destructive lifecycle

Copy goes through the resident `copy` RPC and `internal/clipboard`, not browser
clipboard storage. Windows copies exclude history and cloud synchronization.
Closing the window does not clear the platform clipboard; see the
[sharing specialization](TOKEN_WRAP.md#native-clipboard).

Close destroys the window/browser and loses its navigation, dialogs, revealed
values, drafts and sharing inputs/outputs. The resident Go Vault session/profile
and saved settings survive until Quit or expiry; Minimize only hides and retains
the same WebView. Reopening creates a fresh page and restores only the resident
session, subject to expiry, not the old page's values or drafts.

Native reconnect starts a new authentication epoch: clear page KV data,
mount/permission caches and sharing forms, then rediscover for the new session.
SSO reconnect clears the page session and requests explicit SSO again; SSH
reconnect uses the normal forced SSH login. Session clearing also drops profile,
personal values, agent rows, editor inputs and open dialogs. Go and page IDs
reject replies to destroyed windows or older documents, while auth/request
identity prevents old-session replies repopulating the current page. The bridge
and whole-document reload behavior are detailed in
[DEVELOPMENT.md](DEVELOPMENT.md#resident-runtime-and-disposable-window).

## Native verification

Backend/browser tests cover the shared contract plus native profile visibility,
agent selection, enrollment without database access, clipboard RPC and page/
session cleanup. [DEVELOPMENT.md](DEVELOPMENT.md) defines offline and native
Windows checks. Real Windows SSO and user-confirmed public-key enrollment remain
operator acceptance; synthetic tests do not establish a live enrollment.
Real Vault writes require a separately agreed disposable target. Versioning,
publication and promotion follow [RELEASES.md](RELEASES.md) only on request.
