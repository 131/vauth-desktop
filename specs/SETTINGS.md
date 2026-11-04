# Settings

Preferences live in `%APPDATA%\vauth-desktop\settings.yml`
(`os.UserConfigDir()` on other platforms). Missing keys use defaults; unreadable
YAML is reported and never overwritten. Settings contain no Vault token,
wrapping token or secret. Vault/SSH configuration still comes from `.vauthrc`.

```yaml
update:
  enable: true
  upstream: https://github.com/131/vauth-desktop
  channel: production
language: fr-fr
```

| Key | Default | Behavior |
| --- | --- | --- |
| `language` | `fr-fr` | `fr-fr`, `en-us`, `zh-cn`; applied by reloading the page; unknown values fall back to French |
| `update.enable` | `true` | Disables all GitHub update requests when false; local startup cleanup still runs |
| `update.upstream` | `https://github.com/131/vauth-desktop` | HTTPS GitHub repository supplying static manifests/releases |
| `update.channel` | `production` | `production` or `beta`; unknown values follow production |

Settings list supported languages/channels and the version/check controls.
Every check is fresh: static [channel manifests](RELEASES.md#channels) replace
the former root-manifest TTL cache. Existing YAML `remote_channels` entries
are ignored on load and disappear on the next settings write. GUI update
preference changes wait while an install transaction completes verification
and restart; they cannot replace the release being installed.
