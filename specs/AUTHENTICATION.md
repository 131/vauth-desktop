# Authentication — design v2

## Review status and intent

Workspace authentication for version 2.0.0.
**Login with SSO** on the disconnected screen obtains an uncached user
Vault session and enables initial SSH public-key enrollment. The workspace/KV behavior is in
[WORKSPACE.md](WORKSPACE.md). The operator explicitly requested targeted callback
deployment to Entra and Vault Primary after acknowledgement. That authorization
does not extend to unrelated infrastructure, broadened policies or a release.

## Two authentication modes

- **SSH agent**: keep the current vauth-login flow, trust/configuration rules,
  encrypted CLI-compatible token cache and renewal/re-login behavior.
- **SSO**: Vault OIDC through Entra, initiated explicitly by the user, including
  when the SSH agent is absent or the key is not yet enrolled. The provider
  browser session may already be signed in; the application still obtains a
  fresh Vault session without reading or writing the CLI token cache.

“Offline” means disconnected from Vault. SSO needs network access to Vault and
Entra; it is not an offline credential or a replacement for enrolling an agent
key. Resolve the Vault address/trust from the existing `.vauthrc`; never ship
an Entra client secret or perform a direct desktop-to-Entra token exchange.
The initial deployment reuses the existing Vault `oidc` mount and `oidc` role.

## Callback and challenge

Use `http://localhost:8250/oidc/callback`, the standard Vault CLI-style loopback
callback. This exact URI must be accepted by both Entra and the Vault OIDC
role. It is not an Identity-hosted callback or a new public HTTPS service.

1. On click, create a fresh cryptographically random client nonce, start one
   pending attempt and bind a temporary loopback-only listener before opening
   the browser. Serve only `/oidc/callback`; bind failure is an explicit error,
   not a switch to an arbitrary unregistered port or a public interface.
2. Go POSTs to `auth/oidc/oidc/auth_url`, without a Vault credential, with
   `role: oidc`, the exact `redirect_uri` and the fresh `client_nonce`.
3. Validate the returned authorization URL against the configured/trusted
   Entra issuer, retain its state in Go and open it in the system browser.
   No authorization URL containing a Vault token is produced.
4. The loopback callback receives the authorization code and state, never the
   Vault token. Validate the pending attempt, exact route/state, provider error
   and required code; reject mismatched/replayed callbacks. Callback requests
   and queries must not be logged or included in rendered error details.
5. Go exchanges `code`, `state` and `client_nonce` at
   `auth/oidc/oidc/callback`, as Identity does. Vault performs the provider code
   exchange and validates the challenge. Accept only a user `auth.client_token`
   with a valid Vault identity; retain the existing rejection of root tokens.
6. Close the listener and discard challenge material on success, cancellation,
   timeout, failure or application exit. Return only a generic completion page
   to the browser, without the code, state, nonce or token. No remote content,
   session token or OIDC challenge state enters WebView JavaScript.

Allow only one pending attempt; disable the SSO button while it is pending,
provide cancellation, and use a finite three-minute timeout.
Failures stay inline on the disconnected screen with a retry action, matching
the existing login error presentation. Starting a new attempt invalidates the
old one; unrelated local requests cannot consume a valid pending callback.

## Token lifecycle and first enrollment

`auth.client_token` is a **Vault token**. The operator calls it `VAUTH_TOKEN`;
it is not a separate Entra access token and neither `VAUTH_TOKEN` nor
`VAULT_TOKEN` is exported or stored by the SSO flow.

- Keep the token only in the Go session, not settings, WebView storage/DOM,
  browser URLs, disk cache, environment, logs, handoffs or diagnostics.
- Use a separate authentication mode/session path. With the pinned
  vauth-login v1.1.7 client, disable `TokenCache` before any lookup/request and
  do not call its SSH/JWT `Connect` to install or renew the SSO token. No
  environment token or cached CLI token may substitute for the SSO result.
- Fetch the Vault entity from the token, not the local Windows username, then
  open **SSH Keys**. Capability-check the current user's own SSH role path;
  publish only the selected/pasted public key to that role with explicit
  confirmation. Preserve existing public-key validation and never accept
  private key material. No modification of another user's role or its policies.
- A missing/unreadable personal KV document must not prevent profile-based
  enrollment. Agent failure leaves manual public-key input available.
- Use the SSO token for normal Vault actions under its ACLs. Renew it in memory
  through Vault's token renewal endpoint if renewable. Expiry/revocation returns
  to the disconnected screen and requires another explicit login; do not
  silently fall back to an SSH-cache session or trigger a surprise SSO browser.
- After successful key publication, offer an explicit **Continue with SSH**
  action. Only that user-triggered SSH login resumes the normal CLI-compatible
  token cache flow. If it fails, show the error without claiming enrollment or
  SSH authentication succeeded; do not overwrite the SSO session prematurely.
- Closing the window to the tray keeps the active in-memory session, like
  today. Actual application exit clears it. Any revocation of the separately
  owned SSO token must never affect a shared CLI token.

## Callback owners and deployed configuration

The inspected local checkouts/remotes match these owners:

| Owner | File and deployed change |
|-------|--------------------------|
| `ips/devops/iac/services/entra/applications` | Add the loopback URI to the existing `Web` `redirect_uri` list in `apps/vault-primary.yml`; preserve the Vault UI and Identity callback URIs. |
| `ips/devops/secrets/l1/vault-l1-authority` | Add the same URI to `vault_jwt_auth_backend_role.azure_oidc_user.allowed_redirect_uris` in `primary/oidc_auth_backend.tf`; preserve existing URIs, role and identity aliases. |

Fetched owner branches match their current tracked `master`. Neither contained
the loopback URI at initial inspection. Entra has a pre-existing untracked
`.gitlab-ci.yml`, outside this task. The two callback source files now contain
the URI, with no other tracked file changes in these owners. The targeted
Vault role apply succeeded; an unauthenticated `auth_url` request accepts the
exact callback. Entra was applied from pa4-services using that shell's existing
`venv`/ARM credentials, avoiding the revoked Azure CLI session on WSL. Its
checkout was clean, fast-forwarded to the tracked `master`, then received only
the localhost callback diff. Each apply updated one existing resource; refreshed
targeted plans show no remaining changes. No browser login or provider code
exchange has been performed. A separate read-only Microsoft Graph query from
pa4 confirmed the localhost URI and both original Web callbacks (HTTP 200),
without persisting or rendering credentials/tokens. Infrastructure source
changes are committed and pushed on `master`: Entra `90116f6`, Vault L1
authority `8bf424a`. The pa4 Entra checkout is fast-forwarded to the published
commit and clean. The unrelated untracked `.gitlab-ci.yml` on WSL was excluded.
Do not create another Entra application, desktop client secret, public callback
service, Identity deployment or broader Vault policy for this design.

Changes require a reviewed Terraform diff/plan in each owning checkout. The
operator explicitly delegated these two targeted callback applies, including
Entra execution from pa4. Callback registration is verified by live-refresh
plans, but that alone does not establish an end-to-end SSO login. Existing source
identity aliases do not prove a freshly issued SSO token can publish the SSH
key: verify that path's capabilities and the real self-enrollment operation.
Any ACL/plugin limitation is reported to its owner, not bypassed by the desktop.

## Verification before release

Mock tests cover both login modes, nonce/state mismatch, missing/error callback,
replay, listener port occupied, timeout/cancel, root/missing-entity tokens,
expiry/renewal, no-cache behavior and first enrollment without an authorized
agent key. Verify the normal shared SSH cache remains unaffected.

After operator-approved callback applies, run a real Windows SSO test and
user-confirmed public-key enrollment. Do not replace a real SSH role's keys
merely to test the specs. Record outcomes without tokens, codes, private key
material or sensitive identifiers; release remains operator-gated.

## Defining references

- Vault [JWT/OIDC authentication](https://developer.hashicorp.com/vault/docs/auth/jwt):
  loopback callback registration for both the provider and Vault role.
- Microsoft [redirect URI restrictions](https://learn.microsoft.com/en-us/entra/identity-platform/reply-url):
  loopback HTTP handling; retain the existing Vault web application registration
  because Vault, not the desktop, owns the confidential provider code exchange.
- Local defining implementation: `ips/domyks/ivscs/identity/www/app.js` and
  `www/vault.js`; local adapter: `internal/vault/client.go` and the pinned
  `github.com/131/vauth-login` v1.1.7 session/cache interfaces.
