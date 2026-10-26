# vauth-desktop settings

The desktop preferences live in one YAML file, outside of any repository:

```
%APPDATA%\vauth-desktop\settings.yml
```

(`os.UserConfigDir()`, so `~/.config/vauth-desktop/settings.yml` elsewhere.)
A missing file means defaults; a missing key takes its default. The file is
written when you change a setting from the settings panel (language, channel)
and when the channel list is refreshed ([releases](RELEASES.md)). An unreadable file is reported and
never overwritten: fix it or delete it.

It holds preferences and caches only, never secrets nor tokens. Vault access
is configured by your [`.vauthrc`](https://github.com/131/vauth-login#configuration).

## Example

```yaml
# vauth desktop settings: https://github.com/131/vauth-desktop/blob/master/specs/SETTINGS.md
update:
  enable: true
  upstream: https://github.com/131/vauth-desktop
  channel: production
language: en-us
remote_channels:
  upstream: https://github.com/131/vauth-desktop
  ttl: 86400
  fetched: 2026-10-26T12:00:00Z
  list:
    beta: v0.2.0-beta.1
    production: v0.1.0
```

## Keys

| Key | Default | Configurable in UI | |
|-----|---------|:------------------:|-|
| `language` | `fr-fr` | yes | `fr-fr` or `en-us`, for the window, notifications, errors and tray menu; applied at once. Unknown values fall back to `fr-fr`. |
| `update.channel` | `production` | yes | Channel followed, one of the manifest keys. An unknown channel falls back to `production`. |
| `update.enable` | `true` | no | `false` disables auto-update: no request to GitHub; the settings panel says so. |
| `update.upstream` | `https://github.com/131/vauth-desktop` | no | GitHub repository providing `manifest.json` and the releases, `https://github.com/<owner>/<repo>`. Shown in the settings panel. |
| `remote_channels` | | no | Cache of the upstream `manifest.json`, managed by the application. |

The settings panel is the wrench at the top right of the window. It also shows
the running and latest versions and has a *Check for updates* button, which
fetches `manifest.json` again whatever the cache.

## Channels

The upstream [`manifest.json`](RELEASES.md#channels) is kept in
`remote_channels` for `ttl` seconds (default 86400, one day; lower it to follow
releases sooner) after `fetched`; *Check for updates* ignores it. When the
upstream changes, the cache is ignored. When GitHub is unreachable, a stale
cache of the same upstream is still used. Delete `remote_channels` to force a
refresh.
