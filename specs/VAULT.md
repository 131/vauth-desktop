# Vault access

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

When `VAUTH_USER_LOGIN` is unset, the Windows account name provides it to
`$${env.VAUTH_USER_LOGIN}` macros.

## SSH-agent sign-in

Sign-in is silent, through [vauth-login](https://github.com/131/vauth-login)
and the SSH agent: no browser, no OIDC, no password prompt. The agent is found
as in vauth-login: `SSH_AUTH_SOCK`, then the Pageant (0.75+) and OpenSSH agent
pipes. Tokens share vauth-login's encrypted cache
([ssh-agent-crypt](https://github.com/131/ssh-agent-crypt)) and are renewed in
the last third of their lease.

This mode requires an authorized public key on the user's Vault SSH role.
**Login with SSO** supports first enrollment in the desktop instead; its token
is kept in memory, independently of the normal SSH cache.

## Workspace data sources

- **Vauth Secured Environnement (venv)** combines the current Vault entity
  metadata with the personal KV v2 dictionary at
  `private/<login>/data/.vauth_database`. Identity takes precedence and remains
  immutable; personal writes are uppercase and use compare-and-set.
- **SSH Keys** reads/publishes public keys only on the connected user's
  `auth/<ssh path>/role/<login>`; the selected agent key or pasted public key
  replaces `public_keys` after confirmation and a capability check.
- **Secrets browser** discovers visible engines using `sys/internal/ui/mounts`
  and filters by type `kv`, not the mount name. Go retains the mount/version
  registry and rejects arbitrary mount/API routes, traversal and encoded
  escapes. Unsupported versions and access failures are explicit.

For `<mount>/<path>`, KV v1 uses the direct route. KV v2 lists its `metadata/`
route and reads/writes its `data/` route. Folder listing is lazy; documents keep
JSON key names and value types. Each operation checks `sys/capabilities-self`
for the exact target and remains subject to Vault's actual response.

KV v2 POST writes use CAS zero for new documents and the read version for edits.
Conflicts preserve the editor draft. DELETE soft-deletes the current KV v2
version; no metadata purge/permanent destroy is offered. KV v1 writes lack CAS
and its DELETE is permanent, with this distinction shown in the editor and
confirmation. Mount settings, policies and other users' roles are not writable.

## Authentication modes

SSH-agent login uses the existing shared encrypted vauth-login cache. SSO holds
the freshly issued Vault token only in Go memory, renews it through Vault if
renewable and requires explicit login on expiry/revocation. It does not call
SSH/JWT `Connect` or read/write the CLI cache; see
[AUTHENTICATION.md](AUTHENTICATION.md). Tokens and callback codes never enter
the page, settings, logs or URLs containing a Vault credential.

Vault calls run in Go; [WORKSPACE.md](WORKSPACE.md) defines the list/editor UI.
This describes the 2.0.0 implementation.

### Operator profile verification — October 27, 2026

Read-only verification with the current operator's non-root token confirmed
its entity matches the operator environment. User-facing mount discovery
returned exactly two KV engines, both version 2:

| Mount | Root metadata capabilities | Root data capabilities |
|-------|----------------------------|------------------------|
| `private/<login>/` | create, delete, list, read, update | create, delete, list, read, update |
| `secrets/` | list, read | deny |

The actual personal mount is named `private`, not `personal`. These are
`sys/capabilities-self` observations at the roots only: a `deny` at the shared
data root does not establish rights on descendant document paths. The UI must
query each actual target's capabilities. No secret document values were read,
no secret paths were enumerated, and no Vault data or ACLs were modified.

## Security

- Whoever controls the SSH agent can sign in as the user.
- Secrets are only fetched from Vault and kept in memory while the window
  runs; copying one puts it on the clipboard, like any copy.
- The page runs no remote content (strict CSP); Vault calls are made from Go,
  never from the WebView.
- The selected `.vauthrc` decides which Vault the application talks to.
