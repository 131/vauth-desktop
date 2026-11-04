# Ad-hoc response wrapping

## Purpose and UI

**Wrap & Unwrap** is the fourth signed-in workspace tab. It shares data entered
on the spot, without creating or reading a KV document. It remains usable when
personal KV access is denied. No policy, mount or authentication changes are
made by the feature.

Two independent forms provide:

- **Wrap**: plain text or a JSON object, validity in whole minutes (default five),
  and an explicit submit. Text becomes `{"value": "<exact entered text>"}`;
  JSON preserves nested values, arrays, booleans, numbers and null under a
  top-level object. Arrays/scalars/null at the top level are rejected. Blank
  text is rejected by the form; an empty JSON object is allowed.
- The returned wrapping token, its actual TTL in seconds and creation timestamp
  appear with a copy action. The actual TTL returned by Vault is authoritative.
  Successful wrapping clears the submitted draft unless it was edited during
  the request. A failure preserves both the draft and any previously issued
  token; a subsequent success replaces the displayed token.
- **Unwrap**: paste a token issued by the configured Vault and click **Unwrap
  once**. The warning explains that this consumes the token and cannot be
  undone. The app never unwraps automatically, on paste or when opening a tab.
- Display the complete recovered Vault response as formatted JSON, masked by
  default, with explicit reveal/hide and copy actions. Leaving the tab masks
  it again. A new unwrap attempt clears the previous result, including on
  failure; success clears the submitted token unless the input was edited
  during the request. Invalid, expired and already-used tokens have one clear
  error; other HTTP/network failures remain explicit.
- **Clear all**, reconnect, session clearing and disposable window destruction
  discard drafts, issued/pasted tokens and recovered data. A response arriving
  after clear/reconnect cannot repopulate the page. Clearing the UI does not
  cancel an in-flight Vault operation or revoke an issued token.

Tokens and data are kept only in window memory, never persisted to settings,
WebView storage, logs, build resources or task handoffs. Copying explicitly
places a token/response on the system clipboard; clear/window close does not
clear that clipboard or revoke the token. Whoever holds the token can recover
its payload once before expiry. The wrapping token is deliberate page
input/output, **not** the application's authentication token, which remains
in Go. There is no claim that wrapping is local encryption or a permanent
secret store.

![Ad-hoc wrapping workspace with synthetic data](img/wrapping.png)

## Vault API and RPC

Sources: HashiCorp's [wrapping wrap API](https://developer.hashicorp.com/vault/api-docs/system/wrapping-wrap)
and [wrapping unwrap API](https://developer.hashicorp.com/vault/api-docs/system/wrapping-unwrap).

- `wrap(object, ttlSeconds)` validates a non-null object and positive integral
  TTL in Go. It sends `POST /v1/sys/wrapping/wrap`, the object as the exact JSON
  payload, the active session token as `X-Vault-Token` and the selected seconds
  as `X-Vault-Wrap-TTL`. Normal authentication-specific refresh/error behavior
  applies. Return only `wrap_info.token`, `ttl`, `creation_time` and
  `creation_path`; reject missing tokens and nonpositive TTLs. Vault ACLs and
  limits remain authoritative; no admin fallback is introduced.
- The extra wrapping header is attached through a per-request copy of the
  vauth-login HTTP client and transport. SDK destination trust and the configured
  HTTP transport, timeout and redirect policy remain in effect; the session's
  SDK/client is not modified. No upstream dependency changes are required.
- `unwrap(token)` trims and rejects blank input. It sends exactly one
  `POST /v1/sys/wrapping/unwrap` with the wrapping token **only** in
  `X-Vault-Token`, no JSON `token` parameter and no wrapping TTL header. Using
  the token in both places is prohibited by the Vault API because it causes
  double-use/revocation without recovering the value.
- Unwrap uses the configured SDK destination, but does not renew/check the
  desktop session, retry on HTTP/network failure or replace the desktop's
  token. The window normally already has a configured SDK; if absent, normal
  connection initializes it before the unwrap request. The wrapping token
  carries its own unwrap permission, independent of the desktop's KV ACLs.
- Return the complete response object, including any `data`, `auth`, metadata
  and warnings, rather than assuming it came from this form or a KV engine.
  Non-object/malformed responses fail. Server error bodies are not rendered.

## Verification

Mock Go tests cover exact routes/headers/payload types, response metadata,
validation, HTTP errors, malformed replies, single use, absence of session
refresh/token mutation and absence of retries after ambiguous network failure.
RPC tests reject missing/wrong argument types. Browser tests cover text/typed
JSON, invalid top-level JSON, failed drafts, explicit unwrap, masked/revealed
results, remasking, stale-result removal, clear and late-response rejection.
Both language catalogs cover the new interface and errors.

No live Vault data is written as part of these tests. Native Windows/WebView2
interaction and a live disposable wrapping token remain operator validation;
mock/browser and cross-build checks do not claim these were exercised.
