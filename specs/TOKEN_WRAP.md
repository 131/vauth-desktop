# One-time secret sharing — LOCAL specialization

## Defining source

[vauth-ui one-time secret sharing](https://github.com/131/vauth-ui/blob/main/specs/TOKEN_WRAP.md)
is the canonical functional spec, including text/TTL interaction, wrap/unwrap
semantics, failure/draft handling and the full en-us/fr-fr/zh-cn label table.
The shared [Vault UX spec](https://github.com/131/vauth-ui/blob/main/specs/VAULT.md)
defines its place in the workspace. This document records native integration
only; it does not redefine the common API, navigation or terminology.

## Native Go transport

`app.go` dispatches `wrap` and `unwrap` RPCs to `internal/vault/wrapping.go`.
Vault JSON encoding/decoding and metadata remain private to Go; the page handles
only the deliberate sharing text and wrapping input/output tokens. The
application's authentication token stays in Go and, for SSH login, the shared
encrypted SSH cache; SSO never writes that cache. See
[VAULT.md](VAULT.md#authentication-modes).

The native wrapping transport adds the TTL header through a cloned HTTP client
without bypassing vauth-login's destination trust, transport, timeout or redirect
policy. HTTP errors do not expose server response bodies. The common payload,
header and one-use requirements are defined by the canonical source, not here.

## Native clipboard

Copy uses the resident `copy` RPC and `internal/clipboard`. On Windows the
`CanIncludeInClipboardHistory` and `CanUploadToCloudClipboard` formats are set
to zero, excluding copied text/tokens from history and cloud synchronization.
Clipboard contents are not cleared when the window closes.

Native Close destroys both forms and their outputs along with the WebView,
while retaining the resident authentication session. Reconnect and session/page
replacement use the native cleanup and stale-reply guards in
[WORKSPACE.md](WORKSPACE.md#native-clipboard-and-destructive-lifecycle).
Sharing values are not written to settings, logs or WebView storage.

## Offline terminology regression data

The visible catalog is `internal/i18n/messages.json`. The
[golden fixture](../ui/testdata/token-wrap-terms.json) is offline regression test
data derived from the original desktop table, with the canonical source URL.
It is not a second normative specification. `TestTokenWrapLanguageAndTerminology`
compares all ten existing en-us/fr-fr/zh-cn strings exactly, checks French
translation and retention of the word token, and verifies this local canonical
reference. New common wording belongs in the shared spec; deliberate catalog
changes must update the regression snapshot after checking that source.

The pinned `vauth-ui` v0.1.1 archive predates the shared spec documents. Tests
therefore require neither remote fetching nor a sibling checkout.

## Verification and native screenshots

Mock backend tests verify native transport/trust, errors and session isolation;
browser tests verify the shared sharing contract with native copy and reconnect
integration. No real secrets are used. Native Windows and live sharing remain
operator acceptance; see [DEVELOPMENT.md](DEVELOPMENT.md).

These screenshots are synthetic examples, not another functional contract:

![Wrap with synthetic data](img/wrapping.png)
![Unwrap with synthetic data](img/unwrapping.png)
![Reserved empty token area in French](img/wrapping-empty-fr.png)
![Retained secret and generated token in French](img/wrapping-fr.png)
![Opening a secret in French](img/unwrapping-fr.png)
![Chinese secret sending controls](img/wrapping-zh.png)
![Chinese secret opening controls](img/unwrapping-zh.png)
