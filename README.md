# vauth-desktop

## Your personal Vault space, one click from the tray

A small Windows app for your personal [Vault](https://developer.hashicorp.com/vault/docs)
space: your secrets, your profile and your SSH key. It signs you in with your
SSH agent (Pageant or ssh-agent), like [vauth](https://github.com/131/vauth-login):
no browser, no password.

## Install

Download `vauth-desktop-windows-amd64.exe` from the
[latest release](https://github.com/131/vauth-desktop/releases/latest), put it
wherever you like and run it. One signed file, nothing to install.

Your SSH key must already be allowed on Vault (your first sign-in is set up
elsewhere); you can then change it from the app.

## What you can do

- **Secrets**: search, add, edit, copy and delete your variables. Values stay
  hidden until you reveal them.
- **Profile**: see your Vault identity.
- **SSH key**: pick a key from your agent and publish it to Vault.
- **Browse Vault**: open the Vault web UI, already signed in.

The app lives in the notification area: closing the window keeps it there,
click the icon to bring it back, right-click > Quit to exit.

## Updates

vauth-desktop updates itself: when a new version is out, a notification offers
to install it. Only versions signed by the same publisher are accepted.
Choose the release channel (`production` by default) and check for updates
from the settings (wrench, top right).

## Settings

Language and release channel are set from the app. Everything else lives in
`%APPDATA%\vauth-desktop\settings.yml`: see [settings](specs/SETTINGS.md).

Your Vault address and sign-in method come from your usual `.vauthrc`, as for
vauth: see [vauth-login](https://github.com/131/vauth-login#configuration).

## Security

Whoever uses your SSH agent can sign in as you: lock your session. Secrets are
only kept in memory while the app runs.

## Specifications

[Settings](specs/SETTINGS.md), [releases and auto-update](specs/RELEASES.md),
[Vault access](specs/VAULT.md), [development](specs/DEVELOPMENT.md).

## Credits

- [François Leurent/131](https://github.com/131)
- [OpenAI](https://openai.com) Codex, in collaboration
- [vauth-login](https://github.com/131/vauth-login) and
  [ssh-agent-crypt](https://github.com/131/ssh-agent-crypt): Vault login and
  token cache
- [go-webview2](https://github.com/jchv/go-webview2): WebView2 window without cgo
- Code signing, courtesy of Essilor International

vauth-desktop is not affiliated with HashiCorp: see [CREDITS.md](CREDITS.md)
for trademarks.

ISC license.
