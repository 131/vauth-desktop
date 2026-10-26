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
window.vauthDone = ({id, error, result}) => { const call = pending.get(id); if (!call) return; pending.delete(id); error ? call.reject(new Error(error)) : call.resolve(result); };
const call = (method, ...args) => new Promise((resolve, reject) => { const id = ++sequence; pending.set(id, {resolve, reject}); window.vauth_call(id, method, args); });
let session = null, database = {values: {}, version: 0}, identity = {}, editing = null;
// Notifications float over the window (top layer, above dialogs) and never move
// the layout. At most `visible` at once; the rest wait their turn.
const toasts = {visible: 4, queue: []};
function notice(message, error = false, action = null) { toasts.queue.push({message, error, action}); flushToasts(); }
function flushToasts() {
  const box = $('toasts');
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
    button.addEventListener('click', () => run(button, async () => { await action.run(); remove(); }));
    item.append(button);
  }
  item.append(close); arm();
  return item;
}
async function run(button, action) {
  button.disabled = true;
  try { await action(); } catch (error) { notice(error.message, true); } finally { button.disabled = false; }
}
function confirmed(message) {
  $('confirm-message').textContent = message; $('confirm').returnValue = '';
  return new Promise(resolve => { $('confirm').addEventListener('close', () => resolve($('confirm').returnValue === 'ok'), {once: true}); $('confirm').showModal(); $('confirm-ok').focus(); });
}
async function copy(text) {
  try { await navigator.clipboard.writeText(text); return; } catch {}
  const area = document.createElement('textarea'); area.value = text; document.body.append(area); area.select();
  const done = document.execCommand('copy'); area.remove();
  if (!done) throw new Error(t('copy.failed'));
}
async function load(renew = false) {
  session = await call('connect', renew);
  identity = {...session.profile};
  $('greeting').textContent = t('greeting', identity.VAUTH_USER_NAME || identity.VAUTH_USER_LOGIN);
  $('account').textContent = identity.VAUTH_USER_MAIL || identity.VAUTH_USER_LOGIN;
  $('vault-addr').textContent = session.vault_addr.replace(/^https?:\/\//, '').replace(/\/$/, ''); $('vault-place').hidden = false;
  database = await call('read'); render(); $('login').hidden = true; $('workspace').hidden = false;
}
function render() {
  const fixed = session.fixed_vars;
  const editable = Object.fromEntries(Object.entries(database.values).filter(([key]) => !fixed.includes(key) && !Object.hasOwn(identity, key)));
  const combined = {...editable, ...identity};
  const search = $('search').value.toLowerCase();
  $('count').textContent = Object.keys(editable).length;
  $('entries').replaceChildren(); $('profile-entries').replaceChildren();
  const entries = Object.entries(combined).sort(([a], [b]) => a.localeCompare(b)).filter(([key]) => Object.hasOwn(identity, key) || key.toLowerCase().includes(search));
  if (!Object.keys(editable).some(key => key.toLowerCase().includes(search))) { const empty = document.createElement('p'); empty.className = 'empty'; empty.textContent = Object.keys(database.values).length ? t('empty.search') : t('empty.vault'); $('entries').append(empty); }
  for (const [key, value] of entries) {
    const isFixed = Object.hasOwn(identity, key);
    const row = document.createElement('article'); row.className = 'entry';
    const name = document.createElement('strong'); name.textContent = key; name.title = key;
    const field = valueField(String(value), key, {secret: !isFixed, copied: t('copied.secret'),
      remove: isFixed ? null : async () => { if (!await confirmed(t('delete.confirm', key))) return; const values = {...database.values}; delete values[key]; await save(values); notice(t('deleted')); }});
    const actions = document.createElement('div'); actions.className = 'actions';
    if (isFixed) { const badge = document.createElement('span'); badge.className = 'badge'; badge.textContent = t('readonly'); actions.append(badge); }
    else { const edit = document.createElement('button'); edit.className = 'secondary small'; edit.type = 'button'; edit.textContent = t('edit'); edit.addEventListener('click', () => openEditor(key)); actions.append(edit); }
    row.append(name, field, actions); $(isFixed ? 'profile-entries' : 'entries').append(row);
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
function openEditor(key = null) { editing = key; $('editor-title').textContent = key ? t('editor.edit') : t('editor.add'); $('key').value = key || ''; $('key').readOnly = !!key; $('value').value = key ? String(database.values[key]) : ''; $('editor').showModal(); $('key').focus(); }
async function save(values) { await call('write', values, database.version); database = await call('read'); render(); }
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
$('refresh').addEventListener('click', () => run($('refresh'), async () => { database = await call('read'); render(); }));
$('add').addEventListener('click', () => openEditor());
$('close-editor').addEventListener('click', () => $('editor').close());
$('editor-form').addEventListener('submit', event => { event.preventDefault(); run(event.submitter, async () => {
  const key = editing || await call('variable_name', $('key').value);
  if (!editing && Object.hasOwn(database.values, key)) throw new Error(t('exists'));
  await save({...database.values, [key]: $('value').value}); $('editor').close(); notice(t('saved'));
}); });
for (const tab of ['secrets', 'profile', 'ssh']) $(tab + '-tab').addEventListener('click', () => { for (const item of ['secrets', 'profile', 'ssh']) { $(item + '-panel').hidden = item !== tab; $(item + '-tab').classList.toggle('active', item === tab); } if (tab === 'ssh') run($('ssh-tab'), loadSSH); });
$('ssh-form').addEventListener('submit', event => { event.preventDefault(); run(event.submitter, async () => { if (!await confirmed(t('ssh.publish.confirm'))) return; await call('ssh_publish', $('ssh-key').value); await loadSSH(); notice(t('ssh.published')); }); });
$('browse-vault').addEventListener('click', () => run($('browse-vault'), async () => { await call('browse'); notice(t('browse.opened')); }));
// Sign-in failures stay on the login page (no toast) and the button offers a retry.
async function signIn() {
  const button = $('connect'); button.disabled = true;
  try { await load(); $('login-error').hidden = true; }
  catch (error) { $('login-error-text').textContent = error.message; $('login-error').hidden = false; button.firstChild.textContent = t('retry') + ' '; }
  finally { button.disabled = false; }
}
$('connect').addEventListener('click', signIn);
$('reconnect').addEventListener('click', () => run($('reconnect'), async () => { await load(true); notice(t('reconnected')); }));
call('version').then(version => { $('version').textContent = ` / ${version}`; $('current-version').textContent = version; }).catch(() => {});
// Settings dialog (wrench): language, update channel and manual check. The
// channel list comes from manifest.json; the automatic check stays silent.
$('settings-button').append(icon('wrench'));
$('settings-button').addEventListener('click', () => $('settings').showModal());
$('close-settings').addEventListener('click', () => $('settings').close());
// A click on the grayed backdrop closes the settings, like Escape.
$('settings').addEventListener('click', event => { if (event.target === $('settings')) $('settings').close(); });
let offered = '';
function updates(release) {
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
  notice(t('update.available', release.latest, release.current), false, {label: t('update.install'), run: async () => {
    notice(t('update.downloading', release.latest));
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
$('channel').addEventListener('change', () => run($('channel'), async () => {
  const release = await call('update_channel', $('channel').value);
  updates(release);
  if (!release.available) notice(t('update.channel.selected', release.channel, release.latest || t('update.none')));
}));
// The language applies at once: Go renders the page again.
const languageNames = {'fr-fr': 'Français', 'en-us': 'English'};
$('language').replaceChildren(...window.I18N.languages.map(name => new Option(languageNames[name] || name, name, false, name === window.I18N.language)));
$('language').addEventListener('change', () => run($('language'), () => call('set_language', $('language').value)));
signIn();
})();
