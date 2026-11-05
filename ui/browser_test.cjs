// Run with the development browser tool available on the module search path.
// All Vault RPCs are simulated; this test never uses credentials or a real Vault.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const puppeteer = require('puppeteer');

const messages = JSON.parse(fs.readFileSync(path.join(__dirname, '../internal/i18n/messages.json')));
const escape = text => text.replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function page(language, offline = false) {
  const catalog = messages[language];
  let html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8').replace(/\{\{([^}]+)\}\}/g, (_, key) => escape(catalog[key]));
  html = html.replace('<link rel="stylesheet" href="style.css">', '<style>' + fs.readFileSync(path.join(__dirname, 'style.css'), 'utf8') + '</style>');
  const mock = `
  window.I18N = ${JSON.stringify({language, languages: Object.keys(messages), messages: catalog})};
  // Older development Chromium lacks popovers; only the notification host is shimmed.
  if (!HTMLElement.prototype.showPopover) {
    HTMLElement.prototype.showPopover = function() {this.setAttribute('data-open', 'true')};
    HTMLElement.prototype.hidePopover = function() {this.removeAttribute('data-open')};
    const original = Element.prototype.matches;
    Element.prototype.matches = function(selector) {return selector === ':popover-open' ? this.hasAttribute('data-open') : original.call(this, selector)};
  }
  window.testState = {session:null, readError:false, conflict:false, version:1, trace:[], documents:{}, expires:false,sshError:${offline}};
  const user = {login:'jdoe',vault_addr:'https://vault.example.test',profile:{VAUTH_USER_LOGIN:'jdoe',VAUTH_USER_NAME:'John Doe',VAUTH_USER_MAIL:'jdoe@example.test',CUSTOM_ID:'profile'},fixed_vars:['VAUTH_USER_LOGIN','VAUTH_USER_NAME','VAUTH_USER_MAIL'],auth_mode:'ssh'};
  const typed = {text:'synthetic-hidden-value',flag:true,number:2,array:[1,null],nested:{ok:false}};
  window._wails = {invoke: message => {
    if (message.startsWith('wails:non-client-region:')) { window.testCaptionRegions = JSON.parse(message.slice('wails:non-client-region:'.length)); return; }
    if (message.startsWith('wails:')) return;
    const {id, page, method, args} = JSON.parse(message);
    const state = window.testState; state.trace.push({method,args});
    const respond = (result, error='', expired=false) => window.vauthDone({id,page,result,error,expired});
    setTimeout(() => {
      switch(method) {
        case 'copy': return navigator.clipboard.writeText(args[0]).then(() => respond(null), error => respond(null, error.message));
        case 'version': return respond('dev');
        case 'session': return respond(state.session);
        case 'connect': if(state.sshError) return respond(null,'SSH agent unavailable'); state.session={...user,auth_mode:'ssh'}; return respond(state.session);
        case 'sso_login': state.session={...user,auth_mode:'sso'}; return respond(state.session);
        case 'sso_cancel': return respond(null);
        case 'session_check': return respond(null,state.expires?'Expired':'',state.expires);
        case 'read': return state.readError ? respond(null,'Access denied') : respond({values:{TOKEN:'synthetic-environment',VAUTH_USER_NAME:'must-not-win'},version:1});
        case 'write': return respond(null);
        case 'wrap': if(state.wrapError) return respond(null,'Wrap denied'); return respond('synthetic-wrapping-token');
        case 'unwrap': if(state.unwrapError) return respond(null,'Invalid or consumed token'); if(state.unwrapSlow) return setTimeout(()=>respond('late message'),160); return respond('synthetic-unwrapped-text');
        case 'agent_keys': return respond([]);
        case 'ssh_read': return respond([]);
        case 'ssh_publish': return respond(null);
        case 'kv_mounts': return respond([{path:'secrets/',version:2},{path:'legacy/',version:1}]);
        case 'kv_list': return respond({keys:args[1]?['new','slow']:['folder/','typed','denied']});
        case 'kv_read': {
          if (args[1] === 'denied') return respond(null,'Access denied');
          const doc = {values:state.documents[args[1]] || (args[1].endsWith('slow')?{old:'stale'}:typed),version:state.version,exists:true,capabilities:['read','update','delete','create']};
          return setTimeout(() => respond(doc),args[1].endsWith('slow')?160:0);
        }
        case 'kv_capabilities': return respond(args[1].endsWith('denied')?['deny']:['read','create','update','delete']);
        case 'kv_write': if (state.conflict) return respond(null,'CAS conflict'); state.documents[args[1]]=args[2]; state.version++; return respond(null);
        case 'kv_delete': return respond(null);
        case 'update_check': return respond({enabled:!!state.available,channels:['production'],channel:'production',upstream:'https://github.com/131/vauth-desktop',current:'v0.1.3',latest:'v2.0.0',available:!!state.available,development:false});
        case 'update_install': return setTimeout(()=>respond(null,'Simulated install error'),120);
        default: return respond(null);
      }
    },0);
  }};
  `;
  return html.replace('<script src="app.js"></script>', '<script>'+mock+'</script><script>'+fs.readFileSync(path.join(__dirname,'app.js'),'utf8')+'</script>');
}

(async () => {
  const server = http.createServer((req,res) => {res.setHeader('Content-Type','text/html; charset=utf-8');res.end(page(req.url.includes('/zh')?'zh-cn':req.url.includes('fr')?'fr-fr':'en-us',req.url.includes('offline')))});
  await new Promise(resolve => server.listen(0,'127.0.0.1',resolve));
  let browser;
  try {
    browser = await puppeteer.launch({headless:true,args:['--no-sandbox','--disable-dev-shm-usage']});
    const tab = await browser.newPage(); await tab.setViewport({width:920,height:600});
    const errors=[]; tab.on('pageerror',error=>errors.push(error.message));
    const click = selector => tab.$eval(selector, node => node.click());
    await tab.goto('http://127.0.0.1:'+server.address().port+'/en');
    if(process.env.UI_DEBUG) console.log('Initial UI',await tab.evaluate(()=>['connect','login','workspace','toasts'].map(id=>{const n=document.getElementById(id),r=n.getBoundingClientRect();return {id,display:getComputedStyle(n).display,rect:[r.x,r.y,r.width,r.height]}})),errors);
    await tab.waitForSelector('#workspace:not([hidden])');
    await tab.waitForFunction(() => document.querySelectorAll('#entries .entry').length===5);
    assert.equal(await tab.$eval('#count',node=>node.textContent),'5');
    // Notifications stay in the modal host and retain the same action on error.
    await click('#settings-button'); await tab.evaluate(()=>window.testState.available=true); await click('#check-update');
    await tab.waitForSelector('#settings #toasts .toast button.small');
    await click('#settings #toasts .toast button.small');
    await tab.waitForFunction(()=>document.querySelector('#settings #toasts button.small')?.getAttribute('aria-busy')==='true');
    assert.equal(await tab.$eval('#settings #toasts button.small',n=>n.disabled),true);
    await tab.waitForFunction(()=>!document.querySelector('#settings #toasts button.small')?.disabled);
    await click('#close-settings'); await tab.waitForFunction(()=>document.getElementById('toasts').parentElement===document.body);
    assert.equal(await tab.$eval('#profile-tab',node=>!!node).catch(()=>false),false);
    assert.equal(await tab.$eval('#browse-vault',node=>!!node).catch(()=>false),false);
    assert.equal(await tab.$eval('#entries',node=>node.textContent.includes('must-not-win')),false);
    await tab.type('#search','CUSTOM'); await tab.waitForFunction(()=>document.querySelectorAll('#entries .entry').length===1);
    assert.equal(await tab.$eval('#count',node=>node.textContent),'1');
    await tab.$eval('#search',node=>{node.value='';node.dispatchEvent(new Event('input'))});
    await click('#kv-tab'); await tab.waitForSelector('#kv-folders button');
    await tab.evaluate(()=>[...document.querySelectorAll('#kv-folders button')].find(x=>x.textContent.includes('typed')).click());
    await tab.waitForFunction(()=>document.querySelectorAll('#kv-values .entry').length===5);
    assert.equal(await tab.$eval('#kv-values',node=>node.textContent.includes('synthetic-hidden-value')),false);
    assert.equal(await tab.$eval('#kv-values input',node=>node.value),'············');
    await click('#kv-edit'); await tab.waitForSelector('#kv-editor[open]');
    const draft=JSON.stringify({flag:false,array:[null,2],nested:{new:true}});
    await tab.evaluate(text=>{document.getElementById('kv-editor-json').value=text;window.testState.conflict=true},draft);
    await tab.waitForFunction(()=>!document.getElementById('kv-editor-save').disabled); await click('#kv-editor-save');
    await tab.waitForSelector('#kv-editor-error:not([hidden])');
    assert.equal(await tab.$eval('#kv-editor-json',node=>node.value),draft);
    await tab.evaluate(()=>window.testState.conflict=false); await tab.waitForFunction(()=>!document.getElementById('kv-editor-save').disabled); await click('#kv-editor-save');
    await tab.waitForFunction(()=>!document.getElementById('kv-editor').open);
    const write=await tab.evaluate(()=>window.testState.trace.filter(x=>x.method==='kv_write').pop());
    assert.deepEqual(write.args[2],JSON.parse(draft)); assert.equal(write.args[3],1); assert.equal(write.args[4],false);
    // Access denial must clear values from the previous document.
    await tab.evaluate(()=>[...document.querySelectorAll('#kv-folders button')].find(x=>x.textContent.includes('denied')).click());
    await tab.waitForFunction(()=>document.getElementById('kv-message').textContent==='Access denied');
    assert.equal(await tab.$$eval('#kv-values .entry',nodes=>nodes.length),0);
    await click('#kv-new'); await tab.waitForSelector('#kv-editor[open]');
    await tab.$eval('#kv-editor-path',node=>{node.value='denied';node.dispatchEvent(new Event('input'))});
    await new Promise(resolve=>setTimeout(resolve,50)); assert.equal(await tab.$eval('#kv-editor-save',node=>node.disabled),true);
    await click('#kv-editor-cancel'); await tab.waitForSelector('#confirm[open]'); await click('#confirm-ok');
    await tab.waitForFunction(()=>!document.getElementById('kv-editor').open);
    // Older document responses cannot repaint a newer folder selection.
    await tab.evaluate(()=>[...document.querySelectorAll('#kv-folders button')].find(x=>x.textContent.includes('folder/')).click());
    await tab.waitForFunction(()=>document.querySelector('#kv-folders button')?.textContent.includes('new'));
    await tab.evaluate(()=>[...document.querySelectorAll('#kv-folders button')].find(x=>x.textContent.includes('slow')).click());
    await click('#kv-breadcrumbs button'); await new Promise(resolve=>setTimeout(resolve,220));
    assert.equal(await tab.$eval('#kv-document-path',node=>node.textContent),''); assert.equal(await tab.$$eval('#kv-values .entry',nodes=>nodes.length),0);
    // Clicking a key copies only its name, never its masked value.
    await tab.evaluate(()=>{window.testCopied=[];Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>window.testCopied.push(text)}})});
    await click('#secrets-tab');
    await tab.evaluate(()=>[...document.querySelectorAll('#entries .key-name')].find(n=>n.textContent==='TOKEN').click());
    await tab.waitForFunction(()=>window.testCopied.length>0);
    assert.equal(await tab.evaluate(()=>window.testCopied.pop()),'TOKEN');
    await click('#kv-tab');
    await tab.evaluate(()=>[...document.querySelectorAll('#kv-folders button')].find(n=>n.textContent.includes('typed')).click());
    await tab.waitForSelector('#kv-values .key-name');
    await tab.evaluate(()=>[...document.querySelectorAll('#kv-values .key-name')].find(n=>n.textContent==='flag').click());
    await tab.waitForFunction(()=>window.testCopied.length>0);
    assert.equal(await tab.evaluate(()=>window.testCopied.pop()),'flag');
    // Wrap has a TTL dropdown but no JSON options; unwrap is a separate text-only tab.
    await click('#wrap-tab'); await tab.waitForSelector('#wrap-panel:not([hidden])');
    assert.equal(await tab.$eval('#unwrap-panel',n=>n.hidden),true);
    assert.equal(await tab.$eval('#wrap-tab',n=>n.textContent),"Send (wrap) a secret");
    assert.equal(await tab.$eval('#unwrap-tab',n=>n.textContent),"Open (unwrap) a secret");
    assert.equal(await tab.$eval('#wrap-panel h2',n=>n.textContent),"Send (wrap) a secret");
    assert.equal(await tab.$eval('#unwrap-panel h2',n=>n.textContent),"Open (unwrap) a secret");
    await tab.setViewport({width:620,height:600});
    assert.equal(await tab.evaluate(()=>[...document.querySelectorAll('nav .tab')].every(n=>n.getBoundingClientRect().right<=innerWidth)),true,'Full tab labels fit a narrow window');
    await tab.setViewport({width:920,height:600});

    assert.equal(await tab.$('#wrap-mode'),null);
    assert.deepEqual(await tab.$$eval('#wrap-ttl option',nodes=>nodes.map(n=>n.value)),['900','21600','259200']);
    assert.equal(await tab.$eval('#wrap-ttl',n=>n.value),'900');
    assert.equal(await tab.$eval('#wrap-token',n=>n.tagName),'INPUT');
    assert.equal(await tab.$eval('#unwrap-token',n=>n.tagName),'INPUT');
    assert.equal(await tab.$eval('#unwrap-data',n=>n.tagName),'TEXTAREA');
    assert.equal(await tab.$eval('#wrap-submit',n=>n.textContent),"Wrap your secret into a one-time-token");
    assert.equal(await tab.$eval('#unwrap-submit',n=>n.textContent),"Unwrap your one-time-token");
    assert.equal(await tab.$eval('#wrap-token',n=>n.closest('label').textContent),'Your one-time-token');
    assert.equal(await tab.$eval('#unwrap-token',n=>n.closest('label').textContent),'Token to open');
    assert.equal(await tab.evaluate(()=>document.getElementById('wrap-ttl').getBoundingClientRect().bottom < document.getElementById('wrap-data').getBoundingClientRect().top),true,'TTL precedes the textarea');
    assert.equal(await tab.$eval('#wrap-submit',n=>n.getBoundingClientRect().height<=32),true,'Classic button height');
    const editorBefore=await tab.$eval('#wrap-data',n=>({width:n.getBoundingClientRect().width,height:n.getBoundingClientRect().height}));
    await tab.setViewport({width:1100,height:760});
    const editorAfter=await tab.$eval('#wrap-data',n=>({width:n.getBoundingClientRect().width,height:n.getBoundingClientRect().height}));
    assert.ok(editorAfter.width>editorBefore.width+150 && editorAfter.height>editorBefore.height+120,'Send editor fills both dimensions');
    await tab.setViewport({width:920,height:600});

    assert.equal(await tab.$eval('#wrap-created',n=>n.hidden),false,'Token area is reserved before wrapping');
    assert.equal(await tab.$eval('#wrap-token',n=>n.disabled && n.readOnly),true);
    assert.equal(await tab.$eval('#wrap-copy',n=>n.disabled),true);
    const geometryBefore=await tab.evaluate(()=>['wrap-data','wrap-submit','wrap-token'].map(id=>{const r=document.getElementById(id).getBoundingClientRect();return [r.top,r.left,r.width,r.height]}));
    const messageDraft='adhoc\ntext {"value":false}';
    await tab.type('#wrap-data',messageDraft); await click('#wrap-submit');
    await tab.waitForFunction(()=>!document.getElementById('wrap-token').disabled && !document.getElementById('wrap-submit').disabled);
    const wrapped=await tab.evaluate(()=>window.testState.trace.filter(x=>x.method==='wrap').pop());
    assert.deepEqual(wrapped.args,[messageDraft,900],'Plain text and selected TTL; JSON-like text is literal');
    assert.equal(await tab.$eval('#wrap-data',n=>n.value),messageDraft,'Wrapping preserves the secret draft');
    assert.equal(await tab.$eval('#wrap-token',n=>n.value),'synthetic-wrapping-token');
    assert.equal(await tab.$eval('#wrap-token',n=>!n.disabled && n.readOnly),true);
    assert.deepEqual(await tab.evaluate(()=>['wrap-data','wrap-submit','wrap-token'].map(id=>{const r=document.getElementById(id).getBoundingClientRect();return [r.top,r.left,r.width,r.height]})),geometryBefore,'No layout shift after wrapping');

    assert.equal(await tab.evaluate(()=>window.testCopied.pop()),'synthetic-wrapping-token','Wrap copies automatically');
    assert.equal(await tab.evaluate(()=>window.testState.trace.some(x=>x.method==='unwrap')),false,'No automatic unwrap');
    for(const duration of ['21600','259200']) {
      await tab.select('#wrap-ttl',duration); await tab.$eval('#wrap-data',n=>n.value='TTL test'); await click('#wrap-submit');
      await tab.waitForFunction(()=>!document.getElementById('wrap-submit').disabled);
      assert.equal(await tab.evaluate(()=>window.testState.trace.filter(x=>x.method==='wrap').pop().args[1]),Number(duration));
    }

    await tab.evaluate(()=>{document.getElementById('wrap-data').value='failed draft';window.testState.wrapError=true});
    await click('#wrap-submit'); await tab.waitForFunction(()=>!document.getElementById('wrap-submit').disabled);
    assert.equal(await tab.$eval('#wrap-data',n=>n.value),'failed draft');
    assert.equal(await tab.$eval('#wrap-token',n=>n.value),'synthetic-wrapping-token','Keep issued token on failure');
    // Copy failure must retain the generated token, without an automatic re-wrap.
    const beforeCopyFailure=await tab.evaluate(()=>window.testState.trace.filter(x=>x.method==='wrap').length);
    await tab.evaluate(()=>{window.testState.wrapError=false;navigator.clipboard.writeText=async()=>{throw new Error('clipboard unavailable')};window.savedExecCommand=document.execCommand;document.execCommand=()=>false;document.getElementById('wrap-data').value='clipboard failure'});
    await click('#wrap-submit'); await tab.waitForFunction(()=>!document.getElementById('wrap-submit').disabled);
    assert.equal(await tab.$eval('#wrap-token',n=>n.value),'synthetic-wrapping-token');
    assert.equal(await tab.evaluate(()=>window.testState.trace.filter(x=>x.method==='wrap').length),beforeCopyFailure+1);
    assert.equal(await tab.$eval('#wrap-data',n=>n.value),'clipboard failure','Secret is retained after clipboard failure');
    await tab.evaluate(()=>{navigator.clipboard.writeText=async text=>window.testCopied.push(text);document.execCommand=window.savedExecCommand});
    await click('#unwrap-tab'); await tab.waitForSelector('#unwrap-panel:not([hidden])');
    assert.equal(await tab.$eval('#wrap-panel',n=>n.hidden),true);
    await tab.type('#unwrap-token','  synthetic-wrapping-token  '); await click('#unwrap-submit');
    await tab.waitForSelector('#unwrap-result:not([hidden])');
    assert.equal(await tab.$eval('#unwrap-data',n=>n.value),'synthetic-unwrapped-text','Plain message, no JSON or reveal step');
    assert.equal(await tab.$eval('#unwrap-token',n=>n.value),'');
    const readBefore=await tab.$eval('#unwrap-data',n=>n.getBoundingClientRect().height);
    await tab.setViewport({width:1100,height:760});
    assert.ok(await tab.$eval('#unwrap-data',n=>n.getBoundingClientRect().height)>readBefore+120,'Read textarea fills height');
    assert.equal(await tab.evaluate(()=>{const field=document.getElementById('unwrap-data').getBoundingClientRect(),panel=document.getElementById('unwrap-panel').getBoundingClientRect();return Math.abs(field.left-panel.left)<1 && Math.abs(field.right-panel.right)<1 && Math.abs(field.bottom-panel.bottom)<12}),true,'Read textarea anchors left/right/bottom');
    await tab.setViewport({width:920,height:600});

    await click('#unwrap-copy'); await tab.waitForFunction(()=>!document.getElementById('unwrap-copy').disabled); assert.equal(await tab.evaluate(()=>window.testCopied.pop()),'synthetic-unwrapped-text');
    await tab.type('#unwrap-token','already-used'); await tab.evaluate(()=>window.testState.unwrapError=true);
    await click('#unwrap-submit'); await tab.waitForFunction(()=>!document.getElementById('unwrap-submit').disabled);
    assert.equal(await tab.$eval('#unwrap-result',n=>n.hidden),true);
    assert.equal(await tab.$eval('#unwrap-token',n=>n.value),'already-used');
    // Reconnect clears both forms; late responses from the prior session are ignored.
    await tab.evaluate(()=>{window.copyDone=false;navigator.clipboard.writeText=()=>new Promise(resolve=>setTimeout(()=>{window.copyDone=true;resolve()},160));document.getElementById('wrap-copy').click()});
    await tab.evaluate(()=>{window.testState.unwrapError=false;window.testState.unwrapSlow=true;document.getElementById('unwrap-token').value='late';document.getElementById('unwrap-form').dispatchEvent(new Event('submit',{cancelable:true}))});
    await click('#reconnect'); await tab.waitForSelector('#secrets-panel:not([hidden])');
    await tab.waitForFunction(()=>!document.getElementById('unwrap-submit').disabled && window.copyDone);
    for(const id of ['wrap-data','wrap-token','unwrap-token','unwrap-data']) assert.equal(await tab.$eval('#'+id,n=>n.value),'');
    assert.equal(await tab.$eval('#wrap-token',n=>n.disabled),true); assert.equal(await tab.$eval('#wrap-copy',n=>n.disabled),true);
    assert.equal(await tab.$eval('#wrap-created',n=>n.hidden),false);
    // SSO can reach first-enrollment UI even with no database permission.
    await tab.goto('http://127.0.0.1:'+server.address().port+'/fr?offline');
    await tab.evaluate(()=>window.testState.readError=true); await click('#sso-login');
    await tab.waitForSelector('#ssh-panel:not([hidden])'); assert.equal(await tab.$eval('#ssh-key',node=>node.disabled),false);
    assert.equal(await tab.$eval('#sso-login',node=>node.textContent),'Login with SSO');
    await click('#secrets-tab'); await tab.waitForFunction(()=>document.querySelectorAll('#entries .entry').length===4);
    assert.equal(await tab.$eval('#add',node=>node.disabled),true);
    assert.equal(await tab.$eval('#wrap-tab',n=>n.textContent),'Envoyer (wrap) un secret');
    assert.equal(await tab.$eval('#unwrap-tab',n=>n.textContent),'Ouvrir (unwrap) un secret');
    assert.equal(await tab.$eval('#wrap-submit',n=>n.textContent),'Créer (wrap) un token à usage unique pour votre secret');
    assert.equal(await tab.$eval('#unwrap-submit',n=>n.textContent),'Ouvrir (unwrap) votre token à usage unique');
    assert.equal(await tab.$eval('#wrap-token',n=>n.closest('label').textContent),'Votre token à usage unique');
    assert.equal(await tab.$eval('#unwrap-token',n=>n.closest('label').textContent),'Token à ouvrir');
    await click('#wrap-tab');
    await tab.evaluate(()=>{Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async()=>{}}})});
    await tab.type('#wrap-data','Un secret synthétique'); await click('#wrap-submit');
    await tab.waitForFunction(()=>!document.getElementById('wrap-submit').disabled);
    assert.equal(await tab.$eval('#wrap-data',n=>n.value),'Un secret synthétique');
    assert.equal(await tab.$eval('#wrap-token',n=>n.value),'synthetic-wrapping-token');


    if (process.env.UI_SCREENSHOT) {
      await tab.goto('http://127.0.0.1:'+server.address().port+'/en');
      await tab.waitForFunction(()=>document.querySelectorAll('#entries .entry').length===5);
      await tab.setViewport({width:920,height:600}); await tab.screenshot({path:process.env.UI_SCREENSHOT});
    }
    if (process.env.UI_WRAP_SCREENSHOT) {
      await tab.goto('http://127.0.0.1:'+server.address().port+'/en');
      await tab.waitForFunction(()=>document.querySelectorAll('#entries .entry').length===5);
      await click('#wrap-tab'); await tab.type('#wrap-data','A temporary password to share'); await click('#wrap-submit');
      await tab.waitForFunction(()=>!document.getElementById('wrap-token').disabled && !document.getElementById('wrap-submit').disabled); await click('#unwrap-tab'); await tab.type('#unwrap-token','synthetic-wrapping-token'); await click('#unwrap-submit');
      await tab.waitForSelector('#unwrap-result:not([hidden])');
      await tab.evaluate(()=>document.querySelectorAll('#toasts .toast .tool').forEach(button=>button.click()));
      await tab.setViewport({width:920,height:600}); await click('#wrap-tab'); await tab.screenshot({path:process.env.UI_WRAP_SCREENSHOT});
      await click('#unwrap-tab'); if(process.env.UI_UNWRAP_SCREENSHOT) await tab.screenshot({path:process.env.UI_UNWRAP_SCREENSHOT});
    }
    if (process.env.UI_WRAP_FR_SCREENSHOT) {
      await tab.goto('http://127.0.0.1:'+server.address().port+'/fr');
      await tab.waitForFunction(()=>document.querySelectorAll('#entries .entry').length===5);
      await click('#wrap-tab'); await tab.type('#wrap-data','Un secret temporaire à transmettre');
      await tab.evaluate(()=>{Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async()=>{}}})});
      await tab.setViewport({width:920,height:600});
      if (process.env.UI_WRAP_EMPTY_SCREENSHOT) await tab.screenshot({path:process.env.UI_WRAP_EMPTY_SCREENSHOT});
      await click('#wrap-submit'); await tab.waitForFunction(()=>!document.getElementById('wrap-token').disabled && !document.getElementById('wrap-submit').disabled);
      await tab.evaluate(()=>document.querySelectorAll('#toasts .toast .tool').forEach(button=>button.click()));
      await tab.screenshot({path:process.env.UI_WRAP_FR_SCREENSHOT});
      if (process.env.UI_UNWRAP_FR_SCREENSHOT) {
        await click('#unwrap-tab'); await tab.type('#unwrap-token','synthetic-wrapping-token'); await click('#unwrap-submit');
        await tab.waitForSelector('#unwrap-result:not([hidden])'); await tab.screenshot({path:process.env.UI_UNWRAP_FR_SCREENSHOT});
      }
    }
    // Chinese is selectable and renders a complete, working secret-sharing UI.
    await tab.goto('http://127.0.0.1:'+server.address().port+'/zh');
    await tab.waitForFunction(()=>document.querySelectorAll('#entries .entry').length===5);
    assert.equal(await tab.$eval('html',n=>n.lang),'zh-CN');
    assert.deepEqual(await tab.$$eval('#language option',nodes=>nodes.map(n=>[n.value,n.textContent])),[['fr-fr','Français'],['en-us','English'],['zh-cn','简体中文']]);
    await click('#wrap-tab');
    assert.equal(await tab.$eval('#wrap-tab',n=>n.textContent),'发送 (wrap) 机密');
    assert.equal(await tab.$eval('#unwrap-tab',n=>n.textContent),'打开 (unwrap) 机密');
    assert.equal(await tab.$eval('#wrap-submit',n=>n.textContent),'为您的机密创建 (wrap) 一次性 token');
    assert.equal(await tab.$eval('#unwrap-submit',n=>n.textContent),'打开 (unwrap) 您的一次性 token');
    assert.equal(await tab.$eval('#wrap-token',n=>n.disabled),true);
    await tab.evaluate(()=>{Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async()=>{}}})});
    await tab.type('#wrap-data','用于测试的机密'); await click('#wrap-submit');
    await tab.waitForFunction(()=>!document.getElementById('wrap-submit').disabled);
    assert.equal(await tab.$eval('#wrap-data',n=>n.value),'用于测试的机密');
    assert.equal(await tab.$eval('#wrap-token',n=>n.value),'synthetic-wrapping-token');
    await tab.evaluate(()=>document.querySelectorAll('#toasts .toast .tool').forEach(button=>button.click()));
    await tab.setViewport({width:920,height:600});
    if (process.env.UI_WRAP_ZH_SCREENSHOT) await tab.screenshot({path:process.env.UI_WRAP_ZH_SCREENSHOT});
    await click('#unwrap-tab'); await tab.type('#unwrap-token','synthetic-wrapping-token'); await click('#unwrap-submit');
    await tab.waitForSelector('#unwrap-result:not([hidden])');
    assert.equal(await tab.$eval('#unwrap-data',n=>n.value),'synthetic-unwrapped-text');
    if (process.env.UI_UNWRAP_ZH_SCREENSHOT) await tab.screenshot({path:process.env.UI_UNWRAP_ZH_SCREENSHOT});
    await tab.waitForFunction(()=>window.testCaptionRegions?.regions?.length===5);
    const regions=await tab.evaluate(()=>window.testCaptionRegions);
    assert.equal(regions.version,1);
    assert.deepEqual(regions.regions.map(r=>r.kind),['caption','caption','minimize','maximize','close']);
    assert.ok(regions.regions.every(r=>r.right>r.left&&r.bottom>r.top),'Physical native caption rectangles');
    assert.deepEqual(errors,[],'Browser script errors');
    console.log('Browser UI: merged rows/search, KV masking, typed CAS/drafts, exact-path ACLs, stale responses first enrollment, copied key names and simple message wrapping OK');
  } finally {if(browser) await browser.close();await new Promise(resolve=>server.close(resolve))}
})().catch(error=>{console.error(error);process.exitCode=1});
