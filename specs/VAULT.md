# Vault access — LOCAL native adapter

## Defining sources

The functional data/environment, KV/JSON/CAS/ACL and navigation contract belongs
to [vauth-ui Vault UX](https://github.com/131/vauth-ui/blob/main/specs/VAULT.md).
Common wrap/unwrap behavior and terminology belong to
[vauth-ui one-time secret sharing](https://github.com/131/vauth-ui/blob/main/specs/TOKEN_WRAP.md).
This document defines native configuration, authentication, cache, Go transport
and trust boundaries. [WORKSPACE.md](WORKSPACE.md) and
[TOKEN_WRAP.md](TOKEN_WRAP.md) specialize only the desktop integration.

## Configuration

vauth-desktop uses the usual `.vauthrc`, first found wins: `VAUTHRC`,
`./.vauthrc`, then `~/.vauthrc`. See
[vauth-login](https://github.com/131/vauth-login#configuration) for the keys
and the Vault trust rules.

```yaml
vault_addr: https://vault.example.test
ssh_auth:
  path: ssh
  role: $${env.VAUTH_USER_LOGIN}
```

At initialization a nonempty `VAUTH_USER_LOGIN` is preserved. Otherwise the OS
account name, lowercased and without a Windows domain prefix, supplies it to
`$${env.VAUTH_USER_LOGIN}` macros. SSO uses that same client environment snapshot;
missing identity is an error, not a guessed/shared role. See
[AUTHENTICATION.md](AUTHENTICATION.md#two-authentication-modes).

## SSH-agent sign-in

SSH sign-in is silent, through
[vauth-login](https://github.com/131/vauth-login) and the SSH agent: no browser,
OIDC or password prompt in this mode. The agent is found as in vauth-login:
`SSH_AUTH_SOCK`, then Pageant (0.75+) and OpenSSH agent pipes. Tokens share
vauth-login's encrypted cache
([ssh-agent-crypt](https://github.com/131/ssh-agent-crypt)) and are renewed in
the last third of their lease.

The resolved `ssh_auth.role` is normalized to lowercase by vauth-login before
authentication and cache identity calculation, regardless of its literal/macro
source. Other fields and JWT/OIDC roles in vauth-login keep their original case;
the desktop SSO adapter separately lowercases its username role.

This mode requires an authorized public key on the user's Vault SSH role.
**Login with SSO** supports native first enrollment instead, independently of
the SSH cache; see [AUTHENTICATION.md](AUTHENTICATION.md).

## Authentication modes

SSH-agent login uses the shared encrypted vauth-login cache. SSO keeps its
freshly issued Vault token only in Go memory, renews through Vault if renewable
and requires explicit login on expiry/revocation. It neither calls SSH/JWT
`Connect` nor reads/writes the CLI cache. Challenge/callback, enrollment and
explicit transition back to SSH are defined in
[AUTHENTICATION.md](AUTHENTICATION.md).

Session tokens and callback codes never enter the page, settings, logs or
credential-bearing URLs. Shareable wrapping tokens are deliberate page input/
output, unlike the Go-only authentication token; see the
[native sharing specialization](TOKEN_WRAP.md).

## Native Go adapter

Vault calls run in `internal/vault` through vauth-login's configured/trusted
client, never directly from the WebView. `app.go` exposes typed RPC arguments
and sanitized results/errors. The native adapter retains the configured
transport, timeout and redirect policy; wrapping's header adaptation is in
[TOKEN_WRAP.md](TOKEN_WRAP.md#native-go-transport).

The native `databasePath` resolves `private/<login>/data/.vauth_database` from
the connected Vault entity login, not the OS account fallback used at startup.
The dictionary/entity shape and merge/write behavior use the shared contract.
Go enforces profile/fixed-name immutability even for a direct environment-write RPC;
[WORKSPACE.md](WORKSPACE.md#native-profile-visibility-exception) records the
native display exception without changing those data rules.

For native SSH publication, Go resolves `ssh_auth.path` (default `ssh`) and the
connected entity login into `auth/<ssh path>/role/<login>`, never a
caller-supplied role/user route. `ReadSSH` returns public keys; `PublishSSH`
validates the selected/pasted authorized-key line and checks the resolved
role's capabilities before replacing `public_keys`. Agent enumeration
stays native, separate from Vault role access; selection/confirmation and first
enrollment are described in [WORKSPACE.md](WORKSPACE.md#disconnected-screen-and-native-ssh-enrollment).
The SSH adapter accepts no private key material and exposes no other users'
roles or policies.

`internal/vault/kv.go` keeps the discovered mount/version registry in Go and
constructs Vault routes from a registered KV mount plus a logical relative path.
RPC callers cannot supply arbitrary API routes or cross into another/non-KV
mount. Native validation rejects traversal, leading slashes, backslashes,
percent-encoded escapes, query/fragment delimiters and control characters;
unsupported versions fail explicitly. UI filtering is not the trust boundary.
The shared spec owns discovery, version-specific API semantics and exact-target
ACL requirements; this local section defines their Go enforcement boundary,
not a second KV route/permission table.

## Native security and lifetime

- Whoever controls the SSH agent can sign in as the user; lock the OS session.
- The selected `.vauthrc` determines the Vault destination and its trust policy.
- The standalone page has a strict CSP and no remote content or Vault calls.
- The resident Vault session/profile survives GUI Close but not Quit/expiry.
  Displayed/draft secrets belong to the disposable page; native cleanup and
  private clipboard behavior are defined in
  [WORKSPACE.md](WORKSPACE.md#native-clipboard-and-destructive-lifecycle), with
  window ownership in [TRAY.md](TRAY.md).
