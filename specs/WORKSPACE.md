# Workspace tabs

## Development status

This describes the 2.0.0 workspace implementation.
The desktop uses the approved labels and layout below. Callback configuration
is deployed and published in both owning systems; see
[AUTHENTICATION.md](AUTHENTICATION.md). Mock backend and browser tests validate
behavior without accessing live secrets. Real Windows SSO and user-confirmed
public-key enrollment remain part of the release acceptance checklist.

## Navigation

The signed-in workspace has three tabs, in this order:

1. **Vauth Secured Environnement (venv)**: personal environment variables and
   Vault identity metadata in one list; selected by default.
2. **SSH Keys**: agent-key selection and publication, including first enrollment
   after SSO.
3. **Secrets browser**: embedded browser/editor for user-accessible KV mounts.

Use these exact operator-selected tab labels and order, including
“Environnement”. Labels remain catalog-backed; supporting help/errors are
translated in French and English.
The environment includes the profile. Authentication and release links can
open the system browser; KV browsing/editing is embedded in the desktop.

## Disconnected screen and first SSH enrollment

The disconnected screen offers SSH-agent login and **Login with SSO**; it works even when no authorized SSH
key exists or no agent is reachable. “Offline” here means not signed in to
Vault, not an authentication method that works without network access.

The SSO action opens the system browser for the Vault/Entra OIDC challenge and
uses a temporary local callback listener. Once SSO completes, open **SSH Keys** to allow initial public-key
publication; do not require the environment database to load successfully
before making the enrollment UI available. Preserve manual public-key entry
when the agent cannot supply a key. Publication errors are explicit and do not
silently switch authentication methods.

## Vauth Secured Environnement (venv)

- Merge the personal `.vauth_database` values and the connected entity profile
  into one list. Profile values take precedence on a duplicate name, matching
  the existing vauth environment contract. Show each name once.
- Search filters the entire merged list by variable name. Identify profile
  rows with the existing read-only badge; counts and empty states describe
  the visible merged list, not just editable variables.
- Preserve the compact desktop rows: aligned names, fixed-size read-only
  value fields, icon tools, fixed right-hand actions and a scrolling list.
- Personal values stay masked by default with the fixed placeholder (neither
  their value nor their length in the masked field). Keep reveal/hide,
  selection and explicit copy. Keep Add, Edit and Delete through the existing
  dialogs/actions, never inline editing; writes retain compare-and-set.
- Profile values remain visible, selectable and copyable, with no Add/Edit/
  Delete action for those names. Profile collisions cannot be overwritten or
  removed through an environment write, including a direct RPC call.
- “Read-only style” means read-only display fields, not removal of the existing
  personal-variable CRUD functionality. Identity metadata is actually immutable.
- Refresh reloads the personal database and profile without losing the search.
  This tab does not resolve project `.vauthrc` macros or launch a shell; it
  shows the personal variables and identity sources used by venv.

## Secrets: embedded KV browser/editor

### Scope and navigation

- “KV store only” selects mounts by their Vault engine type `kv`, not by their
  name. Discover mounts visible to the current user's token, filter out every
  non-KV engine, and offer a mount selector. No fixed `kv/` path or configured
  allowlist of the operator's current mount names.
- Discover using the user-facing mount metadata endpoint, not administrative
  mount enumeration. Keep each mount's full path and KV version; support KV
  v1/v2 and explicitly report unsupported versions or discovery failures.
- Start at the selected mount's root. List the current folder, distinguish
  folders from documents, and provide clickable breadcrumbs, parent navigation
  and refresh. A local name filter applies to the current folder only.
- Selecting a folder lists it; selecting a document reads its key/value pairs.
  Show the mount, current path and selected document clearly. Fetch lazily; no
  recursive crawl or global search. Switching mount clears the old selection.
- Documents are not `.vauth_database` environment entries: preserve their key
  names and JSON value types; do not uppercase names or flatten nested objects.
  Display non-string values as JSON and copy their displayed representation.
- Values are masked by default and reuse the read-only reveal/hide and copy
  fields. Read-only display styling does not impose read-only storage access.

### Writing and permissions

- Provide create-document, edit-document and delete-document actions according
  to the current token's capabilities on the exact target API path. Evaluate
  capabilities when selecting a document or preparing a new target; root-level
  capabilities do not establish rights for all descendants.
- Create/edit use a modal key/value editor with explicit Save/Cancel. Preserve
  JSON types and validate the entire document before submitting; never coerce
  objects, arrays, booleans, numbers or null into strings by accident. Adding,
  changing or removing a field saves the resulting whole document.
- KV v2 writes use the version read from Vault for compare-and-set; new documents
  use CAS zero. Conflicts retain the unsaved draft and offer reload; never
  silently retry by overwriting newer data. KV v1 has no equivalent CAS: make
  that limitation visible before saving, without claiming conflict protection.
- Deletion requires confirmation showing the mount and document path. KV v2
  deletion is the current-version soft delete; no permanent destroy, metadata
  purge, restore or version-history workflow is introduced. KV v1 deletion
  removes the document, with its irreversible nature explicit in confirmation.
- Capability lookup controls offered actions; Vault remains authoritative.
  Handle a denied operation without losing an edit draft, and update permissions
  after reconnect or an authorization failure. Do not grant permissions, change
  policies, bypass ACLs or switch to an administrator token.
- The environment tab and raw KV editor address the same underlying document
  when the personal `.vauth_database` is selected. Invalidate/reload affected
  views after writes; environment edits still enforce profile immutability.

### States and session handling

- Provide loading, no accessible KV mounts, empty folder, no filter match,
  no selected document, missing document and access-denied states. A failed
  request must not label old document values as belonging to a new path.
- Folder navigation, mount changes and refresh clear revealed values; responses
  from an older selection cannot overwrite a newer one. Reconnect clears all
  browser data and permissions and re-discovers mounts for the current session.
- Warn before abandoning an unsaved edit; successful writes/deletes refresh the
  affected folder/document and notify the user. Persist neither secrets nor
  drafts to settings, disk, logs or task handoffs.
- Vault operations use the active Go session and its authentication-specific
  renewal behavior. No token reaches the page or remote Vault UI. The new SSO
  authorization URL is only for authentication, not a token-bearing Vault UI
  link or a wrapped child token.
- Go validates the selected mount against current discovered KV mounts and
  constructs API routes from that mount and a logical relative path. Reject
  traversal, arbitrary/API routes and any request crossing into another or a
  non-KV mount, including encoded bypasses. UI filtering alone is insufficient.

## Acceptance before release

- Default merged tab, all profile metadata, duplicate precedence, immutable
  profile RPC validation, full-list search and existing CAS CRUD are tested.
- KV discovery/type filtering, mount switching, navigation, typed values,
  masking/copy, create/edit/delete, CAS conflicts, drafts, empty/404/403 states,
  refresh/reconnect and stale-response handling are tested for KV v1/v2.
- Backend tests reject non-KV mounts and route escapes for reads and writes;
  exact-path capability checks are covered. No new Vault policy or administrator
  token is needed. Real Vault verification does not write without a separately
  agreed disposable test document.
- Browser sign-in and updater release links work independently of embedded KV
  navigation; KV operations do not create child tokens or open remote content.
- SSO without an authorized SSH key, local callback validation, timeout/cancel,
  profile loading, first public-key publication and explicit transition to
  SSH-agent login are tested. The SSO token never enters the CLI token cache,
  environment, page, settings, logs or crash-report payloads. An unavailable
  personal database cannot block first key enrollment.
- Callback registration is reviewed in both Entra and Vault owners; applies
  remain operator-owned. Real SSO/first-enrollment validation is pending.
- Both language catalogs cover all new labels and errors. Preserve the compact
  layout, minimum window size, tray behavior, SSH key tab and update settings.
- Go tests/vet, Windows cross-build and a Windows UI smoke test pass. Real Vault
  validation records permissions and engine version without recording secrets.
- Update the user README and illustrations to the implemented interface before
  release; do not present this proposal as a shipped feature.
- Version bump, tag, release publication and channel promotion wait for the
  operator's explicit release instruction after implementation review.
