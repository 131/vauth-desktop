# One-time secret sharing

## Two tabs, one action each

**Send (wrap) a secret** and **Open (unwrap) a secret** are separate workspace
tabs, after the secrets browser. These canonical en-us labels have natural fr-fr equivalents for the tabs and
headings; see [Language & terminology](#language--terminology). Navigation wraps at narrow window widths.
The interface and RPC contract deal only in text and tokens, not JSON documents.

- **Send (wrap) a secret**: write any nonblank text secret and click **Wrap your secret into a one-time-token**. The app creates
  a single-use Vault token, displays it and copies it to the clipboard. Send
  that token to the recipient. The lifetime dropdown offers **15 min** (default),
  **6 h** and **72 h**. Vault policies remain authoritative and can refuse a
  selected lifetime. There is no JSON format selector.
- **Open (unwrap) a secret**: paste the received token and click **Unwrap your one-time-token**. Display the original
  secret directly as read-only text, with a copy icon. No JSON response,
  metadata or additional reveal action is shown. The token is consumed by this
  action; it cannot be used again.
- The generated token area is always reserved. Its empty read-only input is
  grayed out and disabled, as is its copy action. Successful wrapping fills and
  enables it without shifting the textarea or button. The secret draft stays
  in the textarea after wrapping, including if clipboard copying fails.
- Token input/output are single-line text fields. The secret to send and the
  recovered secret use textareas, with recovered text read-only.
- The TTL dropdown precedes the send textarea. Both textareas grow with the
  window in both dimensions instead of remaining fixed-size/manual-resize.
  The en-us input label is **Token to open**, and the output label is
  **Your one-time-token**; fr-fr uses the translated terms below.
- Both forms have one compact primary button. Output copy icons allow copying
  again without issuing or consuming another token. A clipboard failure after
  wrapping retains the issued token for manual copying.
- Failed requests preserve the input and a previously issued wrapping token.
  A new unwrap attempt clears the previous secret even if it fails. Success
  clears the submitted unwrap token unless it was edited during the request;
  wrapping never clears the secret draft.
- Reconnect, session clearing and window destruction discard both forms and
  outputs. Replies from a previous session cannot repopulate the new session.
  Tokens/secrets are not written to settings, logs or WebView storage.
  Clipboard contents are not cleared when the window closes.

Anyone holding the token can read the message once before expiry. This uses
Vault response wrapping, not local encryption or a permanent secret store.
No KV document is read or created and no ACL or authentication policy changes.
The application authentication token stays in Go; only the deliberate wrapping
input/output token enters the page and clipboard.

![Wrap with synthetic data](img/wrapping.png)
![Unwrap with synthetic data](img/unwrapping.png)

## Language & terminology

The operator-provided **en-us wording is canonical**, not a fallback to render
in French. `fr-fr` translates the intent into natural French rather than copying
English labels or mechanically transliterating Vault operations.

| en-us | fr-fr |
| --- | --- |
| Send (wrap) a secret | Envoyer un secret |
| Open (unwrap) a secret | Ouvrir un secret |
| Wrap your secret into a one-time-token | Créer un jeton à usage unique pour votre secret |
| Unwrap your one-time-token | Ouvrir votre jeton à usage unique |
| Your one-time-token | Votre jeton à usage unique |
| Token to open | Jeton à ouvrir |
| Your secret | Votre secret |
| The secret | Le secret |
| Valid for | Durée de validité |
| Copy secret | Copier le secret |

Use **secret**, not "message", for the user's text throughout labels, help,
notifications and errors. In en-us, **one-time-token** is the canonical label;
"token" is acceptable in surrounding explanations where its purpose is clear.
In fr-fr, use **jeton à usage unique**, shortened to **jeton** in context. Do not
mix untranslated "token", "wrap", "unwrap" or English action sentences into
French controls. Keep the exact technical names in API/code documentation.

The shareable one-time-token is distinct from the application's Vault session
token, which never enters the page. Do not describe the wrapping token as an
encrypted copy of the secret: it grants a single retrieval from Vault.

The visible catalog lives in `internal/i18n/messages.json`. Every new field,
placeholder, copy action, status and error has both locales; tests cover the
canonical actions and their French equivalents. This convention is scoped to
secret sharing and does not rename other operator-selected workspace labels.

![Reserved empty token area in French](img/wrapping-empty-fr.png)
![Retained secret and generated token in French](img/wrapping-fr.png)
![Opening a secret in French](img/unwrapping-fr.png)

## Text-only API

- `wrap(text, ttlSeconds) -> token`: reject blank text, send `POST sys/wrapping/wrap` with
  the internal JSON object `{"value": text}` and `X-Vault-Wrap-TTL` set to 900, 21600 or 259200 seconds. Text,
  including whitespace, Unicode and JSON-like content, is preserved exactly.
  Use the session's normal authentication and return only the wrapping token.
- `unwrap(token) -> text`: trim/reject blank tokens, send one
  `POST sys/wrapping/unwrap` with the wrapping token only in `X-Vault-Token`.
  Do not put it in a JSON body, renew the desktop session, retry the one-use
  request or replace the application's session token. Extract only the string
  `data.value`; incompatible payloads fail explicitly after consumption.
- JSON encoding/decoding and Vault metadata are private to Go. Go rejects
  lifetimes outside the three supported dropdown values before any request.
  SDK destination trust, transport, timeout and redirect policy are preserved.
  HTTP errors do not expose server response bodies.

Sources: HashiCorp [wrap](https://developer.hashicorp.com/vault/api-docs/system/wrapping-wrap)
and [unwrap](https://developer.hashicorp.com/vault/api-docs/system/wrapping-unwrap) APIs.

## Verification

Mock Go tests cover exact text/envelope/headers, all three TTL choices and invalid TTLs, blank/wrong input,
malformed or incompatible responses, HTTP/network failures, one use, no retry,
no session mutation and redirect policy. Browser tests cover the separate tabs,
text/TTL RPCs, automatic token copying, direct text output, retained drafts,
reserved token geometry before/after wrapping, translated terminology,
copying names independently of secret values, and reconnect clearing.
No real Vault data or secrets are used by these tests. Native Windows usage
and live secret sharing remain operator validation.
