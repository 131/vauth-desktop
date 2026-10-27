# Development guideline

## Boring is good

Use an obvious implementation that fits the desktop/Vault contract:

- One visible list order, one KV mount registry, one release channel manifest.
- Standard formats and tools: JSON documents, YAML preferences, Vault APIs,
  vauth-login, ppackage and GoReleaser.
- Guard-first phases: validate inputs, return errors early, then perform the
  requested operation. Keep related fields readable, not packed into one line.
- Few dependencies; prefer the standard library. No framework for a small UI.
- No speculative options, aliases, cross-user policy changes or extra services.
- Fail loudly rather than guessing. Keep drafts on failed writes and expose
  unsupported engines, missing manifests and denied operations explicitly.
- Keep display fields read-only, dialogs explicit and layout compact/stable.
- Work stays outside the GUI thread. Replies target the native window, including
  while native menus/modal loops run; their end also wakes WebView2 completions.
- Notifications belong to the active modal host. An update action stays disabled
  with busy feedback until verification/restart finishes; failures allow retry.
- Tokens and secret values stay out of logs, persistent UI storage and builds.

Version and tag through ppackage after validation and a clean commit. Publish
signed artifacts and checksums before promoting channels. Development binaries
are not a substitute for verifying the signed release/update path.
