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
  window.I18N = ${JSON.stringify({language, languages: [{value:'fr-fr', label:'Français'}, {value:'en-us', label:'English'}], messages: catalog})};
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
  window.vauth_call = (id, page, method, args) => {
    const state = window.testState; state.trace.push({method,args});
    const respond = (result, error='', expired=false) => window.vauthDone({id,page,result,error,expired});
    setTimeout(() => {
      switch(method) {
        case 'version': return respond('dev');
        case 'session': return respond(state.session);
        case 'connect': if(state.sshError) return respond(null,'SSH agent unavailable'); state.session={...user,auth_mode:'ssh'}; return respond(state.session);
        case 'sso_login': state.session={...user,auth_mode:'sso'}; return respond(state.session);
        case 'sso_cancel': return respond(null);
        case 'session_check': return respond(null,state.expires?'Expired':'',state.expires);
        case 'read': return state.readError ? respond(null,'Access denied') : respond({values:{TOKEN:'synthetic-environment',VAUTH_USER_NAME:'must-not-win'},version:1});
        case 'write': return respond(null);
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
  };
  `;
  return html.replace('<script src="app.js"></script>', '<script>'+mock+'</script><script>'+fs.readFileSync(path.join(__dirname,'app.js'),'utf8')+'</script>');
}

(async () => {
  const server = http.createServer((req,res) => {res.setHeader('Content-Type','text/html; charset=utf-8');res.end(page(req.url.includes('fr')?'fr-fr':'en-us',req.url.includes('offline')))});
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
    // SSO can reach first-enrollment UI even with no database permission.
    await tab.goto('http://127.0.0.1:'+server.address().port+'/fr?offline');
    await tab.evaluate(()=>window.testState.readError=true); await click('#sso-login');
    await tab.waitForSelector('#ssh-panel:not([hidden])'); assert.equal(await tab.$eval('#ssh-key',node=>node.disabled),false);
    assert.equal(await tab.$eval('#sso-login',node=>node.textContent),'Login with SSO');
    await click('#secrets-tab'); await tab.waitForFunction(()=>document.querySelectorAll('#entries .entry').length===4);
    assert.equal(await tab.$eval('#add',node=>node.disabled),true);
    if (process.env.UI_SCREENSHOT) {
      await tab.goto('http://127.0.0.1:'+server.address().port+'/en');
      await tab.waitForFunction(()=>document.querySelectorAll('#entries .entry').length===5);
      await tab.setViewport({width:920,height:600}); await tab.screenshot({path:process.env.UI_SCREENSHOT});
    }
    assert.deepEqual(errors,[],'Browser script errors');
    console.log('Browser UI: merged rows/search, KV masking, typed CAS/drafts, exact-path ACLs, stale responses and first enrollment OK');
  } finally {if(browser) await browser.close();await new Promise(resolve=>server.close(resolve))}
})().catch(error=>{console.error(error);process.exitCode=1});
