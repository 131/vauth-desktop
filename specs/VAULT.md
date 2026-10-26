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

## Sign-in

Sign-in is silent, through [vauth-login](https://github.com/131/vauth-login)
and the SSH agent: no browser, no OIDC, no password prompt. The agent is found
as in vauth-login: `SSH_AUTH_SOCK`, then the Pageant (0.75+) and OpenSSH agent
pipes. Tokens share vauth-login's encrypted cache
([ssh-agent-crypt](https://github.com/131/ssh-agent-crypt)) and are renewed in
the last third of their lease.

The SSH key must already be authorized on the Vault SSH role (the first
enrollment happens elsewhere, for example from GitLab).

## What the application does in Vault

- **Secrets**: reads and writes `.vauth_database` with compare-and-set writes;
  names are upper-cased like `vauth set`. Identity variables are read-only.
- **Profile**: the Vault identity information of the token.
- **SSH key**: publishes the chosen agent key to
  `auth/<ssh path>/role/<login>` (replaces `public_keys`).
- **Browse Vault**: opens the Vault UI in the browser with a one-use wrapped
  child token (60 s); the current token never appears in a URL.

## Security

- Whoever controls the SSH agent can sign in as the user.
- Secrets are only fetched from Vault and kept in memory while the window
  runs; copying one puts it on the clipboard, like any copy.
- The page runs no remote content (strict CSP); Vault calls are made from Go,
  never from the WebView.
- The selected `.vauthrc` decides which Vault the application talks to.
