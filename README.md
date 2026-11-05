# vauth-desktop

## Your personal Vault space, one click from the tray

A small Windows app for your personal [Vault](https://developer.hashicorp.com/vault/docs)
space: your environment, Vault secrets and SSH keys. It signs you in with your
SSH agent (Pageant or ssh-agent), like [vauth](https://github.com/131/vauth-login):
no browser, no password.

![Workspace preview with a demo profile](specs/img/screenshot.png)

## How it works

![SSH agent, vauth-desktop, Vault and GitHub](specs/img/architecture.svg)

## Install

Download `vauth-desktop-windows-amd64.exe` from the
[latest release](https://github.com/131/vauth-desktop/releases/latest), put it
wherever you like and run it. One signed file, nothing to install.

If your SSH key is not enrolled yet, use **Login with SSO** and publish a public
key from **SSH Keys**. SSO needs the configured Vault/Entra callback and keeps
its Vault token only in memory; it does not populate the CLI token cache.

## What you can do

- **Vauth Secured Environnement (venv)**: search personal variables and your
  read-only identity in one list. Add, edit, copy and delete personal variables.
- **SSH Keys**: select an agent key or paste a public key, then publish it to
  your own Vault role. After SSO enrollment, continue with the normal SSH login.
- **Secrets browser**: navigate accessible KV stores, reveal/copy values and
  create, edit or delete documents according to your Vault permissions. KV v2
  writes use compare-and-set; KV v1 has no equivalent protection.
- **Send (wrap) a secret**: write a secret, click **Wrap your secret into a one-time-token**, send the copied token.
  Choose 15 minutes, 6 hours or 72 hours of validity; it can be used only once.
- **Open (unwrap) a secret**: paste a received token, click **Unwrap your one-time-token**, read the secret.

Click a key's name in the environment or KV document list to copy its name,
independently of its hidden value.

Secret values stay hidden until revealed. Changes use explicit dialogs; display
fields remain read-only. The workspace image uses synthetic demonstration data.

The app starts in the notification area, without a browser running. Click the
icon (or launch the app again) to open the workspace. Closing the window destroys
the WebView2 session and frees its browser processes; the tray and Vault session
stay available. The next click creates a fresh window. The window has a compact header with Minimize/Maximize/Close controls,
Windows border/shadow/rounded corners, and native drag and edge resizing.
Double-click the header to maximize or restore. Minimizing hides the window to the tray, retaining its browser. Right-click > Quit exits the app.

## Updates

vauth-desktop updates itself: when a new version is out, a notification offers
to install it. Only versions signed by the same publisher are accepted.
Choose the release channel (`production` by default) and check for updates
from the settings (wrench, top right). The install transaction checks the
requested release, verifies it and restarts through the Wails helper. Old
Windows executables left by replacement are cleaned at startup with bounded
retries; see [release/cleanup details](specs/RELEASES.md).

## Settings

Language and release channel are set from the app. Everything else lives in
`%APPDATA%\vauth-desktop\settings.yml`: see [settings](specs/SETTINGS.md).

Your Vault address and sign-in method come from your usual `.vauthrc`, as for
vauth: see [vauth-login](https://github.com/131/vauth-login#configuration).

Only Windows amd64 is released. The entrypoint and Wails runtime are shared across platforms, but Linux/macOS
behavior, packaging and update verification remain future work.

## Security

Whoever uses your SSH agent can sign in as you: lock your session. Secrets are
only kept in memory while the app runs.

## Specifications

[Settings](specs/SETTINGS.md), [releases and auto-update](specs/RELEASES.md),
[Vault access](specs/VAULT.md), [development](specs/DEVELOPMENT.md), [tray/window](specs/TRAY.md).

[Wrapping](specs/TOKEN_WRAP.md), [workspace](specs/WORKSPACE.md), [authentication and first enrollment](specs/AUTHENTICATION.md).
Version 2.0.0 introduces the workspace and SSO enrollment flow.
Live Windows SSO/enrollment validation remains operator-confirmed.

## Credits

- [131](https://github.com/131)
- [OpenAI](https://openai.com) Codex, in collaboration
- [vauth-login](https://github.com/131/vauth-login) and
  [ssh-agent-crypt](https://github.com/131/ssh-agent-crypt): Vault login and
  token cache
- [Wails v3](https://github.com/wailsapp/wails) (MIT): native runtime, tray, disposable windows and updater
- Code signing, courtesy of Essilor International

vauth-desktop is not affiliated with HashiCorp: see [CREDITS.md](CREDITS.md)
for trademarks.

ISC license.
