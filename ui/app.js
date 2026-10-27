'use strict';
(() => {
const $ = id => document.getElementById(id);
// Messages come from Go (internal/i18n/messages.json); %s are filled in order.
const t = (key, ...args) => { let index = 0; return (window.I18N.messages[key] ?? key).replace(/%[sd]/g, () => String(args[index++])); };
// Desktop window: no browser zoom, reload, context menu or file drop.
const editable = target => target.closest && target.closest('input,textarea');
document.addEventListener('contextmenu', event => { if (!editable(event.target)) event.preventDefault(); });
document.addEventListener('wheel', event => { if (event.ctrlKey) event.preventDefault(); }, {passive: false});
document.addEventListener('keydown', event => {
  const key = event.key.toLowerCase();
  if (key === 'f5' || (event.ctrlKey && ['r', 'p', 'f', 'g', 'u', 's', '+', '-', '=', '0'].includes(key))) event.preventDefault();
});
for (const type of ['dragover', 'drop']) document.addEventListener(type, event => { if (!editable(event.target)) event.preventDefault(); });
// Go answers vauth_call asynchronously through window.vauthDone.
const pending = new Map(); let sequence = 0;
const pageId = Array.from(crypto.getRandomValues(new Uint8Array(16)), byte => byte.toString(16).padStart(2, '0')).join('');
window.vauthDone = ({id, page, error, expired, result}) => { if (page !== pageId) return; const request = pending.get(id); if (!request) return; pending.delete(id); if (error) { const failure = new Error(error); failure.expired = expired; request.reject(failure); } else request.resolve(result); };
const call = (method, ...args) => new Promise((resolve, reject) => { const id = ++sequence; pending.set(id, {resolve, reject}); window.vauth_call(id, pageId, method, args); });
let session = null, database = {values: {}, version: 0}, identity = {}, editing = null;
let databaseLoaded = false, authEpoch = 0, activeTab = 'secrets';
const kv = {mounts: [], mount: '', folder: '', keys: [], doc: null, path: '', request: 0, dirty: true};
let kvEditor = null;
// Notifications float over the window (top layer, above dialogs) and never move
// the layout. At most `visible` at once; the rest wait their turn.
const toasts = {visible: 4, queue: []};
function notice(message, error = false, action = null) { toasts.queue.push({message, error, action}); flushToasts(); }
function flushToasts() {
  const box = $('toasts'), host = [...document.querySelectorAll('dialog[open]')].pop() || document.body;
  if (box.parentElement !== host) { if (box.matches(':popover-open')) box.hidePopover(); host.append(box); }
  while (box.childElementCount < toasts.visible && toasts.queue.length) box.append(toast(toasts.queue.shift()));
  if (!box.childElementCount) { if (box.matches(':popover-open')) box.hidePopover(); return; }
  // Re-open so the stack stays above a dialog opened since.
  if (box.matches(':popover-open')) box.hidePopover();
  box.showPopover();
}
// A toast with an action stays until it is used or closed.
function toast({message, error, action}) {
  const item = document.createElement('div'); item.className = error ? 'toast error' : 'toast';
  const text = document.createElement('p'); text.textContent = message; text.title = message;
  const close = document.createElement('button'); close.type = 'button'; close.className = 'tool'; close.title = t('close'); close.setAttribute('aria-label', t('close')); close.textContent = '×';
  let timer = 0;
  const remove = () => { clearTimeout(timer); item.remove(); flushToasts(); };
  const arm = () => { clearTimeout(timer); if (!action) timer = setTimeout(remove, error ? 9000 : 4500); };
  close.addEventListener('click', remove);
  item.addEventListener('mouseenter', () => clearTimeout(timer));
  item.addEventListener('mouseleave', arm);
  item.append(text);
  if (action) {
    const button = document.createElement('button'); button.type = 'button'; button.className = 'small'; button.textContent = action.label;
    button.addEventListener('click', () => run(button, async () => {
      button.classList.add('busy'); button.setAttribute('aria-busy','true');
      try { if(action.busy) text.textContent=action.busy; await action.run(); remove(); }
      catch(error) {text.textContent=message;throw error;}
      finally {button.classList.remove('busy');button.removeAttribute('aria-busy');}
    }));
    item.append(button);
  }
  item.append(close); arm();
  return item;
}
async function run(button, action) {
  button.disabled = true;
  try { await action(); } catch (error) { if (error.expired) clearSession(); notice(error.message, true); } finally { button.disabled = false; }
}
function confirmed(message) {
  $('confirm-message').textContent = message; $('confirm').returnValue = '';
  return new Promise(resolve => { $('confirm').addEventListener('close', () => resolve($('confirm').returnValue === 'ok'), {once: true}); showDialog($('confirm')); $('confirm-ok').focus(); });
}
function showDialog(dialog) { dialog.showModal(); flushToasts(); }
for (const dialog of document.querySelectorAll('dialog')) dialog.addEventListener('close', flushToasts);
async function copy(text) {
  try { await navigator.clipboard.writeText(text); return; } catch {}
  const area = document.createElement('textarea'); area.value = text; document.body.append(area); area.select();
  const done = document.execCommand('copy'); area.remove();
  if (!done) throw new Error(t('copy.failed'));
}
async function load(renew = false) { await startSession(await call('connect', renew)); }
async function startSession(next) {
  authEpoch++; kv.request++; kv.mounts = []; kv.mount = ''; kv.keys = []; kv.doc = null; kv.dirty = true;
  kv.folder = ''; kv.path = ''; for (const id of ['kv-values','kv-folders','kv-mount','kv-breadcrumbs']) $(id).replaceChildren();
  renderKVDocument();
  session = next; identity = {...session.profile}; database = {values: {}, version: 0}; databaseLoaded = false;
  $('greeting').textContent = t('greeting', identity.VAUTH_USER_NAME || identity.VAUTH_USER_LOGIN);
  $('account').textContent = identity.VAUTH_USER_MAIL || identity.VAUTH_USER_LOGIN;
  $('vault-addr').textContent = session.vault_addr.replace(/^https?:\/\//, '').replace(/\/$/, ''); $('vault-place').hidden = false;
  render(); $('login').hidden = true; $('workspace').hidden = false; $('continue-ssh').hidden = true;
  selectTab(session.auth_mode === 'sso' ? 'ssh' : 'secrets', false);
  await refreshEnvironment(false);
}
async function refreshEnvironment(profile = true) {
  const epoch = authEpoch;
  try {
    if (profile) { const next = await call('connect', false); if (epoch !== authEpoch) return; session = next; identity = {...next.profile}; }
    const values = await call('read'); if (epoch !== authEpoch) return;
    database = values; databaseLoaded = true; render();
  } catch (error) { if (epoch !== authEpoch) return; if (error.expired) clearSession(); else { databaseLoaded = false; render(); } notice(error.message, true); }
}
function clearSession() {
  authEpoch++; kv.request++; session = null; identity = {}; database = {values: {}, version: 0}; databaseLoaded = false;
  kv.mounts = []; kv.mount = ''; kv.keys = []; kv.doc = null; kv.path = ''; kvEditor = null;
  for (const id of ['entries', 'agent-keys', 'kv-values', 'kv-folders', 'kv-mount']) $(id).replaceChildren();
  for (const id of ['value', 'ssh-key', 'kv-editor-json', 'kv-editor-path']) $(id).value = '';
  for (const id of ['greeting','account','vault-addr']) $(id).textContent = '';
  renderKVDocument();
  for (const name of ['editor', 'kv-editor', 'confirm']) if ($(name).open) $(name).close();
  $('workspace').hidden = true; $('login').hidden = false; $('vault-place').hidden = true;
}
function render() {
  const fixed = session.fixed_vars;
  const editable = Object.fromEntries(Object.entries(database.values).filter(([key]) => !fixed.includes(key) && !Object.hasOwn(identity, key)));
  const combined = {...editable, ...identity};
  const search = $('search').value.toLowerCase();
  $('entries').replaceChildren(); $('add').disabled = !databaseLoaded;
  const entries = Object.entries(combined).sort(([a], [b]) => a.localeCompare(b)).filter(([key]) => key.toLowerCase().includes(search));
  $('count').textContent = entries.length;
  if (!entries.length) { const empty = document.createElement('p'); empty.className = 'empty'; empty.textContent = search ? t('empty.search') : t('empty.vault'); $('entries').append(empty); }
  for (const [key, value] of entries) {
    const isFixed = Object.hasOwn(identity, key);
    const row = document.createElement('article'); row.className = 'entry';
    const name = document.createElement('strong'); name.textContent = key; name.title = key;
    const field = valueField(String(value), key, {secret: !isFixed, copied: t('copied.secret'),
      remove: isFixed ? null : async () => { if (!await confirmed(t('delete.confirm', key))) return; const values = {...database.values}; delete values[key]; await save(values); notice(t('deleted')); }});
    const actions = document.createElement('div'); actions.className = 'actions';
    if (isFixed) { const badge = document.createElement('span'); badge.className = 'badge'; badge.textContent = t('readonly'); actions.append(badge); }
    else { const edit = document.createElement('button'); edit.className = 'secondary small'; edit.type = 'button'; edit.textContent = t('edit'); edit.addEventListener('click', () => openEditor(key)); actions.append(edit); }
    row.append(name, field, actions); $('entries').append(row);
  }
}
// Masked fields hold a fixed placeholder: neither the value nor its length is in the page.
const masked = '············';
const icons = {
  eye: '<path d="M1 8s2.5-5 7-5 7 5 7 5-2.5 5-7 5-7-5-7-5z"/><circle cx="8" cy="8" r="2"/>',
  'eye-off': '<path d="M1 8s2.5-5 7-5 7 5 7 5-2.5 5-7 5-7-5-7-5z"/><circle cx="8" cy="8" r="2"/><path d="M2 2l12 12"/>',
  copy: '<rect x="5" y="5" width="9" height="9" rx="1.5"/><path d="M11 5V3.5A1.5 1.5 0 0 0 9.5 2h-6A1.5 1.5 0 0 0 2 3.5v6A1.5 1.5 0 0 0 3.5 11H5"/>',
  wrench: '<path d="M10.6 1.8a3.6 3.6 0 0 0-3.4 4.7l-5 5a1.5 1.5 0 0 0 2.1 2.1l5-5a3.6 3.6 0 0 0 4.7-3.4l-2.1 2.1-2-.5-.5-2z"/>',
  trash: '<path d="M2.5 4h11M6 4V2.5h4V4M4 4l.7 9.5h6.6L12 4M6.5 6.5v4.5M9.5 6.5v4.5"/>',
};
// Fixed-size read-only field with icon tools. Clicking a secret field toggles
// its visibility, unless the user is selecting part of the revealed value.
function valueField(value, label, {secret = false, copied = t('copied'), remove = null} = {}) {
  const field = document.createElement('div'); field.className = 'field';
  const input = document.createElement('input'); input.readOnly = true; input.type = secret ? 'password' : 'text'; input.value = secret ? masked : value; input.spellcheck = false; input.setAttribute('aria-label', label); if (secret) input.title = t('toggle.help');
  const doCopy = async () => { await copy(value); notice(copied); };
  let reveal = null;
  field.append(input);
  if (secret) field.append(reveal = tool('eye', t('show'), button => { const show = input.type === 'password'; input.type = show ? 'text' : 'password'; input.value = show ? value : masked; button.replaceChildren(icon(show ? 'eye-off' : 'eye')); button.title = show ? t('hide') : t('show'); button.classList.toggle('on', show); }));
  if (secret) input.addEventListener('click', () => { if (input.type === 'text' && input.selectionStart !== input.selectionEnd) return; reveal.click(); });
  field.append(tool('copy', t('copy'), doCopy));
  if (remove) field.append(tool('trash', t('delete'), remove, 'danger'));
  return field;
}
function icon(name) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', '0 0 16 16'); svg.setAttribute('aria-hidden', 'true'); svg.innerHTML = icons[name];
  return svg;
}
function tool(name, label, action, extra = '') {
  const button = document.createElement('button'); button.type = 'button'; button.className = ('tool ' + extra).trim(); button.title = label; button.setAttribute('aria-label', label);
  button.append(icon(name)); button.addEventListener('click', () => run(button, () => action(button)));
  return button;
}
function openEditor(key = null) { editing = key; $('editor-title').textContent = key ? t('editor.edit') : t('editor.add'); $('key').value = key || ''; $('key').readOnly = !!key; $('value').value = key ? String(database.values[key]) : ''; showDialog($('editor')); $('key').focus(); }
async function save(values) { const editable = Object.fromEntries(Object.entries(values).filter(([key]) => !Object.hasOwn(identity, key) && !session.fixed_vars.includes(key))); await call('write', editable, database.version); kv.request++; kv.dirty = true; kv.doc = null; database = await call('read'); render(); }
async function loadSSH() {
  const [keys, agent] = await Promise.all([call('ssh_read'), call('agent_keys').catch(error => error)]);
  const authorized = new Set(keys.map(key => key.split(' ').slice(0, 2).join(' ')));
  $('ssh-key').value = keys[0] || '';
  $('ssh-status').textContent = keys.length ? t('ssh.status', keys.length) : t('ssh.status.none');
  $('agent-keys').replaceChildren();
  if (agent instanceof Error) { const message = document.createElement('p'); message.className = 'muted'; message.textContent = agent.message; $('agent-keys').append(message); return; }
  for (const item of agent) {
    const current = authorized.has(item.key.split(' ').slice(0, 2).join(' '));
    const row = document.createElement('article'); row.className = current ? 'entry agent-key current' : 'entry agent-key';
    const label = item.comment || item.key.split(' ')[0];
    const name = document.createElement('strong'); name.textContent = label; name.title = label;
    row.append(name, valueField(item.fingerprint, label, {copied: t('copied.fingerprint')}));
    const actions = document.createElement('div'); actions.className = 'actions';
    if (current) { const badge = document.createElement('span'); badge.className = 'badge'; badge.textContent = t('authorized'); actions.append(badge); }
    const use = document.createElement('button'); use.type = 'button'; use.className = 'secondary small'; use.textContent = t('use.key');
    use.addEventListener('click', () => { $('ssh-key').value = item.key; $('ssh-key').focus(); });
    actions.append(use); row.append(actions); $('agent-keys').append(row);
  }
}
$('search').addEventListener('input', render);
$('refresh').addEventListener('click', () => run($('refresh'), refreshEnvironment));
$('add').addEventListener('click', () => openEditor());
$('close-editor').addEventListener('click', () => $('editor').close());
$('editor-form').addEventListener('submit', event => { event.preventDefault(); run(event.submitter, async () => {
  const key = editing || await call('variable_name', $('key').value);
  if (!editing && Object.hasOwn(database.values, key)) throw new Error(t('exists'));
  await save({...database.values, [key]: $('value').value}); $('editor').close(); notice(t('saved'));
}); });
function emptyMessage(container, message) { const p = document.createElement('p'); p.className = 'empty'; p.textContent = message; container.append(p); }
function permitted(caps, action) { return caps && !caps.includes('deny') && (caps.includes(action) || caps.includes('root')); }
async function loadKV() {
  if (!kv.mounts.length) {
    const epoch = authEpoch;
    let mounts;
    try { mounts = await call('kv_mounts'); } catch (error) { if (epoch !== authEpoch) return; if (error.expired) throw error; $('kv-folders').replaceChildren(); emptyMessage($('kv-folders'), error.message); return; }
    if (epoch !== authEpoch) return;
    kv.mounts = mounts; $('kv-mount').replaceChildren(...mounts.map(mount => new Option(mount.path, mount.path)));
    kv.mount = mounts[0]?.path || ''; kv.folder = ''; kv.dirty = true;
  }
  if (!kv.mount) { $('kv-folders').replaceChildren(); emptyMessage($('kv-folders'), t('kv.nomounts')); kv.doc = null; renderKVDocument(); return; }
  if (kv.dirty) await openFolder(kv.folder);
}
function renderKVDirectory() {
  $('kv-folders').replaceChildren();
  const filter = $('kv-filter').value.toLowerCase();
  const keys = kv.keys.filter(key => key.toLowerCase().includes(filter));
  if (!keys.length) emptyMessage($('kv-folders'), kv.keys.length ? t('empty.search') : t('kv.empty'));
  for (const key of keys) {
    const button = document.createElement('button'); button.type = 'button'; button.textContent = (key.endsWith('/') ? '▸ ' : '· ') + key; button.title = key;
    if (kv.folder + key === kv.path) button.classList.add('active');
    button.addEventListener('click', () => run(button, () => key.endsWith('/') ? openFolder(kv.folder + key) : openDocument(kv.folder + key)));
    $('kv-folders').append(button);
  }
}
function renderBreadcrumbs() {
  const box = $('kv-breadcrumbs'); box.replaceChildren();
  const crumb = (label, path) => { const button = document.createElement('button'); button.type = 'button'; button.className = 'secondary small'; button.textContent = label; button.title = path; button.addEventListener('click', () => run(button, () => openFolder(path))); box.append(button); };
  crumb(kv.mount, ''); let path = '';
  for (const piece of kv.folder.split('/').filter(Boolean)) { path += piece + '/'; crumb(piece, path); }
}
function renderKVDocument(message = '') {
  $('kv-values').replaceChildren(); $('kv-document-path').textContent = kv.path; $('kv-document-path').title = kv.mount + kv.path;
  const doc = kv.doc;
  $('kv-edit').hidden = !doc?.exists || !permitted(doc.capabilities, 'update');
  $('kv-delete').hidden = !doc?.exists || !permitted(doc.capabilities, 'delete');
  $('kv-message').textContent = message || (!doc ? t('kv.noselection') : !doc.exists ? t('kv.missing') : '');
  if (!doc?.exists) return;
  for (const [key, value] of Object.entries(doc.values).sort(([a], [b]) => a.localeCompare(b))) {
    const row = document.createElement('article'); row.className = 'entry'; const name = document.createElement('strong'); name.textContent = key; name.title = key;
    row.append(name, valueField(typeof value === 'string' ? value : JSON.stringify(value), key, {secret: true})); $('kv-values').append(row);
  }
}
async function openFolder(path) {
  const request = ++kv.request; kv.folder = path; kv.path = ''; kv.doc = null; kv.keys = []; kv.dirty = true;
  renderBreadcrumbs(); renderKVDocument(); $('kv-folders').replaceChildren(); emptyMessage($('kv-folders'), t('kv.loading'));
  try { const folder = await call('kv_list', kv.mount, path); if (request !== kv.request) return; kv.keys = folder.keys; kv.dirty = false; renderKVDirectory(); }
  catch (error) { if (request !== kv.request) return; if (error.expired) throw error; $('kv-folders').replaceChildren(); emptyMessage($('kv-folders'), error.message); }
}
async function openDocument(path) {
  const request = ++kv.request; kv.path = path; kv.doc = null; renderKVDocument(t('kv.loading'));
  try { const doc = await call('kv_read', kv.mount, path); if (request !== kv.request) return; kv.doc = doc; renderKVDocument(); renderKVDirectory(); }
  catch (error) { if (request !== kv.request) return; if (error.expired) throw error; renderKVDocument(error.message); }
}
async function openKVEditor(create) {
  if (!kv.mount) return;
  const doc = kv.doc; if (!create && !doc?.exists) return;
  const text = JSON.stringify(create ? {} : doc.values, null, 2);
  kvEditor = {mount: kv.mount, path: create ? kv.folder : kv.path, version: create ? 0 : doc.version, create, original: text, capsRequest: 0};
  $('kv-editor-path').value = kvEditor.path; $('kv-editor-path').readOnly = !create;
  $('kv-editor-json').value = text; $('kv-editor-warning').textContent = kv.mounts.find(m => m.path === kv.mount)?.version === 1 ? t('kv.v1') : '';
  $('kv-editor-json').readOnly = false; for (const id of ['kv-editor-close','kv-editor-cancel','kv-editor-reload']) $(id).disabled = false;
  $('kv-editor-error').hidden = true; $('kv-editor-reload').hidden = true; showDialog($('kv-editor'));
  await editorCapabilities();
}
async function editorCapabilities() {
  const editor = kvEditor; if (!editor) return;
  const request = ++editor.capsRequest; $('kv-editor-save').disabled = true;
  try { const caps = await call('kv_capabilities', editor.mount, $('kv-editor-path').value); if (editor !== kvEditor || request !== editor.capsRequest) return; $('kv-editor-save').disabled = !permitted(caps, editor.create ? 'create' : 'update'); }
  catch (error) { if (error.expired) clearSession(); }
}
async function closeKVEditor(force = false) {
  if (!force && kvEditor?.saving) return;
  if (!force && kvEditor && ($('kv-editor-json').value !== kvEditor.original || $('kv-editor-path').value !== kvEditor.path) && !await confirmed(t('kv.draft'))) return;
  $('kv-editor').close(); $('kv-editor-json').value = ''; $('kv-editor-path').value = ''; kvEditor = null;
}
$('kv-mount').addEventListener('change', () => { kv.mount = $('kv-mount').value; $('kv-filter').value = ''; run($('kv-mount'), () => openFolder('')); });
$('kv-filter').addEventListener('input', renderKVDirectory);
$('kv-refresh').addEventListener('click', () => run($('kv-refresh'), async () => { const path = kv.path, mount = kv.mount, ticket = kv.request + 1; await openFolder(kv.folder); if (path && mount === kv.mount && ticket === kv.request) await openDocument(path); }));
$('kv-new').addEventListener('click', () => run($('kv-new'), () => openKVEditor(true)));
$('kv-edit').addEventListener('click', () => run($('kv-edit'), () => openKVEditor(false)));
$('kv-delete').addEventListener('click', () => run($('kv-delete'), async () => { const path = kv.path; if (!await confirmed(t('kv.delete.confirm', kv.mount + path))) return; await call('kv_delete', kv.mount, path); databaseLoaded = false; await openFolder(kv.folder); notice(t('kv.deleted')); }));
$('kv-editor-path').addEventListener('input', editorCapabilities);
for (const id of ['kv-editor-close', 'kv-editor-cancel']) $(id).addEventListener('click', () => closeKVEditor());
$('kv-editor').addEventListener('cancel', event => { event.preventDefault(); closeKVEditor(); });
$('kv-editor-form').addEventListener('submit', event => { event.preventDefault(); run(event.submitter, async () => {
  const editor = kvEditor, epoch = authEpoch; let values;
  try { values = JSON.parse($('kv-editor-json').value); if (!values || Array.isArray(values) || typeof values !== 'object') throw new Error(); } catch { throw new Error(t('kv.invalid')); }
  const path = $('kv-editor-path').value;
  editor.saving = true; $('kv-editor-json').readOnly = true; $('kv-editor-path').readOnly = true;
  for (const id of ['kv-editor-close','kv-editor-cancel','kv-editor-reload']) $(id).disabled = true;
  try { await call('kv_write', editor.mount, path, values, editor.version, editor.create); }
  catch (error) { if (error.expired) throw error; if (editor !== kvEditor || epoch !== authEpoch) return; $('kv-editor-error').textContent = error.message + ' ' + t('kv.conflict'); $('kv-editor-error').hidden = false; $('kv-editor-reload').hidden = false; await editorCapabilities(); return; }
  if (editor !== kvEditor || epoch !== authEpoch) return;
  await closeKVEditor(true); databaseLoaded = false; await openFolder(kv.folder); await openDocument(path); notice(t('kv.saved'));
}).finally(() => { if (kvEditor) { kvEditor.saving = false; $('kv-editor-json').readOnly = false; $('kv-editor-path').readOnly = !kvEditor.create; for (const id of ['kv-editor-close','kv-editor-cancel','kv-editor-reload']) $(id).disabled = false; editorCapabilities(); } }); });
$('kv-editor-reload').addEventListener('click', () => run($('kv-editor-reload'), async () => {
  if (!await confirmed(t('kv.draft'))) return;
  const doc = await call('kv_read', kvEditor.mount, $('kv-editor-path').value);
  kvEditor.version = doc.version; kvEditor.create = !doc.exists && !doc.version; kvEditor.original = JSON.stringify(doc.values, null, 2);
  $('kv-editor-json').value = kvEditor.original; $('kv-editor-error').hidden = true; $('kv-editor-reload').hidden = true; await editorCapabilities();
}));
function selectTab(tab, refresh = true) {
  activeTab = tab;
  for (const item of ['secrets', 'ssh', 'kv']) { $(item + '-panel').hidden = item !== tab; $(item + '-tab').classList.toggle('active', item === tab); }
  if (tab === 'ssh') run($('ssh-tab'), loadSSH);
  if (tab === 'kv') run($('kv-tab'), loadKV);
  if (tab === 'secrets' && !databaseLoaded && refresh) run($('secrets-tab'), refreshEnvironment);
}
for (const tab of ['secrets', 'ssh', 'kv']) $(tab + '-tab').addEventListener('click', () => selectTab(tab));
$('ssh-form').addEventListener('submit', event => { event.preventDefault(); run(event.submitter, async () => { if (!await confirmed(t('ssh.publish.confirm'))) return; await call('ssh_publish', $('ssh-key').value); if (session.auth_mode === 'sso') $('continue-ssh').hidden = false; notice(t('ssh.published')); await loadSSH(); }); });
// Sign-in failures stay inline. SSO never blocks public-key entry on an agent failure.
async function signIn(sso = false) {
  const button = sso ? $('sso-login') : $('connect'); $('connect').disabled = true; $('sso-login').disabled = true;
  $('sso-cancel').hidden = !sso; $('sso-wait').hidden = !sso; $('login-error').hidden = true;
  try { if (sso) await startSession(await call('sso_login')); else await load(); }
  catch (error) { $('login-error-text').textContent = error.message; $('login-error').hidden = false; if (!sso) button.firstChild.textContent = t('retry') + ' '; }
  finally { $('connect').disabled = false; $('sso-login').disabled = false; $('sso-cancel').hidden = true; $('sso-wait').hidden = true; }
}
$('connect').addEventListener('click', () => signIn());
$('sso-login').addEventListener('click', () => signIn(true));
$('sso-cancel').addEventListener('click', () => call('sso_cancel'));
$('reconnect').addEventListener('click', () => run($('reconnect'), async () => { if (session.auth_mode === 'sso') { clearSession(); await signIn(true); } else { await load(true); notice(t('reconnected')); } }));
$('continue-ssh').addEventListener('click', () => run($('continue-ssh'), () => load(true)));
setInterval(() => { if (session) call('session_check').catch(error => { if (error.expired) { clearSession(); notice(error.message, true); } }); }, 30000);
call('version').then(version => { $('version').textContent = ` / ${version}`; $('current-version').textContent = version; }).catch(() => {});
call('session').then(current => current ? startSession(current) : signIn()).catch(error => { if (error.expired) clearSession(); });
// Settings dialog (wrench): language, update channel and manual check. The
// channel list comes from manifest.json; the automatic check stays silent.
$('settings-button').append(icon('wrench'));
$('settings-button').addEventListener('click', () => showDialog($('settings')));
$('close-settings').addEventListener('click', () => $('settings').close());
// A click on the grayed backdrop closes the settings, like Escape.
$('settings').addEventListener('click', event => { if (event.target === $('settings')) $('settings').close(); });
let offered = '';
function updates(release) {
  $('update-enable').checked = release.enabled;
  $('update-disabled').hidden = release.enabled; $('update-fields').hidden = !release.enabled;
  $('update-error').hidden = !release.error; $('update-error').textContent = release.error || '';
  const channels = release.channels || [];
  $('channel').replaceChildren(...channels.map(name => new Option(name, name, false, name === release.channel)));
  $('channel').disabled = !channels.length;
  $('current-version').textContent = release.current;
  $('latest-version').textContent = release.latest || '—';
  $('upstream').textContent = release.upstream.replace(/^https:\/\//, ''); $('upstream').title = release.upstream;
  $('update-dev').hidden = !release.development;
  $('open-release').hidden = !(release.development && release.url);
  if (release.latest) $('open-release').textContent = t('update.download', release.latest);
  if (!release.available || offered === release.latest) return;
  offered = release.latest;
  notice(t('update.available', release.latest, release.current), false, {label: t('update.install'), busy:t('update.downloading',release.latest), run: async () => {
    await call('update_install', release.latest);
  }});
}
call('update_check').then(updates).catch(() => {}); // silent: errors stay in the panel
$('check-update').addEventListener('click', () => run($('check-update'), async () => {
  offered = '';
  const release = await call('update_check', true);
  updates(release);
  if (release.error) throw new Error(release.error);
  if (release.development && release.latest) notice(t('update.dev.notice', release.latest));
  else if (release.enabled && !release.available) notice(t('update.uptodate', release.current, release.channel, release.latest || t('update.none')));
}));
$('open-release').addEventListener('click', () => run($('open-release'), () => call('update_open')));
$('update-enable').addEventListener('change', () => run($('update-enable'), async () => {
  offered = '';
  const release = await call('update_enable', $('update-enable').checked);
  updates(release);
  if (release.error) throw new Error(release.error);
}));
$('channel').addEventListener('change', () => run($('channel'), async () => {
  const release = await call('update_channel', $('channel').value);
  updates(release);
  if (!release.available) notice(t('update.channel.selected', release.channel, release.latest || t('update.none')));
}));
// The language applies at once: Go renders the page again.
const languageNames = {'fr-fr': 'Français', 'en-us': 'English'};
$('language').replaceChildren(...window.I18N.languages.map(name => new Option(languageNames[name] || name, name, false, name === window.I18N.language)));
$('language').addEventListener('change', () => run($('language'), () => call('set_language', $('language').value)));
})();
