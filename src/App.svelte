<script>
  import { App } from 'vauth-ui';
  const t = (key, ...args) => {
    let index = 0;
    return (window.I18N.messages[key] ?? key).replace(/%[sd]/g, () => String(args[index++]));
  };
</script>

{#snippet header()}
  <span class="brand">identity<span><span id="version"></span></span></span>
  <span class="caption-drag" aria-hidden="true"></span>
  <span class="header-tools">
    <span class="badge">{t('badge')}</span>
    <button id="settings-button" type="button" class="tool" title={t('settings')} aria-label={t('settings')}></button>
  </span>
  <span class="caption-buttons">
    <button id="caption-minimize" type="button" title={t('window.minimize')} aria-label={t('window.minimize')}>−</button>
    <button id="caption-maximize" type="button" title={t('window.maximize')} aria-label={t('window.maximize')}>□</button>
    <button id="caption-close" type="button" title={t('window.close')} aria-label={t('window.close')}>×</button>
  </span>
{/snippet}

{#snippet login()}
  <div class="login-card">
    <p class="eyebrow">{t('eyebrow')}</p>
    <h1>{t('hero.1')}<br>{t('hero.2')} <span>{t('hero.3')}</span></h1>
    <p class="lead">{t('lead')}</p>
    <div class="login-action">
      <button id="connect">{t('connect')} <span>↗</span></button>
      <button id="sso-login" class="secondary">{t('sso.login')}</button>
      <button id="sso-cancel" class="secondary" hidden>{t('sso.cancel')}</button>
    </div>
    <p id="sso-wait" class="muted" hidden>{t('sso.wait')}</p>
    <div id="login-error" class="login-error" role="alert" hidden>
      <strong>{t('login.failed')}</strong><span id="login-error-text"></span>
    </div>
    <p class="muted">{t('connect.help')}</p>
    <div class="intro">
      <article><h3>{t('intro.place')}</h3><p>{t('intro.place.text')}</p></article>
      <article><h3>{t('intro.key')}</h3><p>{t('intro.key.text')}</p></article>
    </div>
  </div>
{/snippet}

{#snippet settings()}
  <div class="settings-body">
    <div class="section-head">
      <h2>{t('settings')}</h2>
      <button type="button" id="close-settings" class="icon" aria-label={t('close')}>×</button>
    </div>
    <label class="setting">{t('language')}<select id="language"></select></label>
    <h3>{t('updates')}</h3>
    <label class="setting">{t('update.enable')}<input id="update-enable" class="switch" type="checkbox" role="switch"></label>
    <p id="update-disabled" class="muted" hidden>{t('update.disabled.ui')}</p>
    <p id="update-error" class="muted error" hidden></p>
    <div id="update-fields">
      <label class="setting">{t('channel')}<select id="channel"></select></label>
      <dl>
        <dt>{t('version')}</dt><dd id="current-version"></dd>
        <dt>{t('update.latest')}</dt><dd id="latest-version">—</dd>
        <dt>{t('update.upstream')}</dt><dd id="upstream">—</dd>
      </dl>
      <p id="update-dev" class="muted" hidden>{t('update.dev')}</p>
      <div class="actions">
        <button id="check-update" type="button" class="secondary">{t('update.check')}</button>
        <button id="open-release" type="button" aria-label={t('update.latest')} hidden></button>
      </div>
    </div>
  </div>
{/snippet}

{#snippet workspaceActions()}
  <button id="continue-ssh" class="secondary" hidden>{t('sso.continue')}</button>
  <button id="reconnect" class="secondary">{t('reconnect')}</button>
{/snippet}

{#snippet sshIntro()}
  <p class="muted">{t('ssh.help')}</p>
{/snippet}

{#snippet sshSource()}
  <div id="agent-keys"></div>
{/snippet}

{#snippet footer()}
  <span>{t('footer')}<span id="vault-place" hidden> · <span id="vault-addr"></span></span></span>
{/snippet}

<App {t} brand="identity" {header} {login} {settings} {workspaceActions} {sshIntro} {sshSource} {footer} />
