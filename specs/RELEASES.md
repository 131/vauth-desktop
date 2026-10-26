# Releases and auto-update

## Publishing a release

1. Push a `v*` tag (`-beta.N` tags are published as prereleases):

   ```sh
   git tag v0.1.0 && git push origin v0.1.0
   ```

2. The `release` workflow (`.github/workflows/release.yml`, GitHub environment
   `production`) runs the tests, then GoReleaser
   ([`.goreleaser.yaml`](../.goreleaser.yaml)) builds
   `vauth-desktop-windows-amd64.exe`, signs it with Authenticode through
   `SIGNING_SERVER` ([131/gh-actions](https://github.com/131/gh-actions)
   `sign-binaries`) and publishes it with `SHA256SUMS`.

3. Promote the tag to a channel in [`manifest.json`](../manifest.json) and push
   to the default branch: installed applications follow it.

The `production` environment protects the signing secret (restrict it to `v*`
tags); it is not a release channel.

## Channels

`manifest.json`, at the root of the upstream default branch, maps each channel
to a release tag:

```json
{
  "production": "v0.1.0",
  "beta": "v0.2.0-beta.1"
}
```

The application reads it from
`https://raw.githubusercontent.com/<owner>/<repo>/HEAD/manifest.json` and keeps
it in its settings cache (see [SETTINGS.md](SETTINGS.md#channels)). The channel
selector of the settings panel lists its keys.

## Update

At startup (and on *Check for updates*), when the followed channel points to a
newer version than the running one, a notification offers the update. The
binary is downloaded next to the executable and installed only if:

- its SHA-256 matches the release `SHA256SUMS`;
- Windows trusts its Authenticode signature (whole chain, revocation checked);
- its signer is the same as the running executable's.

The new version then replaces the old one and restarts; the previous binary is
removed at the next start. The executable's folder must be writable by the user.

- Downgrades are never offered: switching from `beta` back to `production`
  waits for the next production release.
- Unsigned or development builds never update themselves: the first install
  must be a signed release, and an end-to-end update can only be observed from
  the second release.
- `update.enable: false` in the settings disables every request to GitHub.

`VAUTH_VERIFY_SIGNED=<signed exe> go test ./internal/update` runs the
Authenticode check against a real signed binary on Windows.
