// Optional browser regression check. Requires an externally installed playwright package.
// Build bin/twikko, then run with NODE_PATH pointing to that node_modules directory.
const {chromium}=require('playwright');
const {spawn}=require('node:child_process');
const fs=require('node:fs');
const assert=require('node:assert/strict');
const cwd=process.cwd();
(async()=>{
 let child;
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 try {
 const dir=fs.mkdtempSync('/tmp/twikko-ui-');
 const view=`version: 1
schema:
  id_field: id
  fields:
    - {name: id, type: text}
    - {name: title, type: text, required: true}
    - {name: n, type: number}
    - {name: due, type: date}
    - {name: flag, type: bool}
    - {name: tags, type: multienum, options: [a, b]}
    - {name: notes, type: longtext}
view:
  show_labels: false
  cards:
    header:
      - {type: text, source: title, show_label: true}
`;
 fs.writeFileSync(dir+'/v.yaml',view);
 fs.writeFileSync(dir+'/d.json',JSON.stringify([{id:'PROJ-101',title:'Before',n:5,due:'2026-11-13',flag:true,tags:['a'],notes:'Text',hidden:{nested:5}}]));
 child=spawn(cwd+'/bin/twikko',['--view',dir+'/v.yaml','--data',dir+'/d.json','--output',dir+'/out.yaml']);
 const url=await new Promise((resolve,reject)=>{child.stdout.on('data',b=>{const m=String(b).match(/url=(\S+)/);if(m)resolve(m[1])});child.on('exit',c=>reject(Error('early exit '+c)))});
 const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(String(e)));page.on('console',m=>{if(m.type()==='error')errors.push(m.text())});
 await page.route('**/entities/**/fields/**',async route=>{await new Promise(r=>setTimeout(r,100));await route.continue()});
 await page.goto(url);
 assert.equal(await page.locator('#field-0-id').isDisabled(),true);
 await page.locator('#field-0-title').focus();
 await page.evaluate(()=>window.originalEditor=document.querySelector('#field-0-title'));
 await page.locator('#field-0-title').fill('Focus check');
 await page.waitForFunction(()=>document.querySelector('#save-status').textContent.includes('1 changed'));
 assert.equal(await page.evaluate(()=>window.originalEditor===document.activeElement),true);
 // Input and action in the same JS turn: action queues behind edits.
 await page.locator('#field-0-title').fill('Changed immediately');
 await page.getByRole('button',{name:'Table',exact:true}).click();
 await page.locator('table').waitFor();
 assert.equal(await page.locator('#field-0-title').inputValue(),'Changed immediately');
 assert.equal(await page.locator('th .sr-only').count(),7);
 assert.equal(await page.getByRole('button',{name:'Table',exact:true}).getAttribute('aria-pressed'),'true');
 await page.locator('#field-0-n').fill('1e-');
 await page.getByRole('button',{name:'Cards',exact:true}).click();
 await page.locator('section#entity-0').waitFor();
 assert.equal(await page.locator('#field-0-n').inputValue(),'1e-');
 assert.match(await page.locator('#status-0-n').innerText(),/number/);
 await page.locator('#field-0-due').fill('2026-');
 await page.getByRole('button',{name:'Table',exact:true}).click();
 await page.locator('table').waitFor();
 assert.equal(await page.locator('#field-0-due').inputValue(),'2026');
 await page.getByRole('button',{name:'Reset',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('#field-0-title')?.value==='Before');
 assert.equal(await page.locator('#field-0-n').inputValue(),'5');
 await page.waitForFunction(()=>!document.querySelector('.htmx-request,[inert]')); 
 // Both view requests are queued while an edit is in flight.
 await page.evaluate(()=>{window.finishedModes=0;document.addEventListener('htmx:afterRequest',e=>{if(e.detail.xhr.responseURL.includes('/mode?'))window.finishedModes++})});
 const locked=await page.evaluate(()=>{const input=document.querySelector('#field-0-title');input.value='Rapid switches';input.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('[data-mode-button="cards"]').click();document.querySelector('[data-mode-button="table"]').click();return document.querySelector('#entities').inert});
 assert.equal(locked,true);
 await page.waitForFunction(()=>window.finishedModes>=2 && !document.querySelector('#entities').inert && document.querySelector('[data-mode-button="table"]').getAttribute('aria-pressed')==='true' && document.querySelector('#field-0-title')?.value==='Rapid switches');
 await page.waitForFunction(()=>!document.querySelector('.htmx-request,[inert]'));
 await page.locator('#field-0-flag').selectOption('');
 await page.locator('td details').filter({has:page.locator('#group-0-tags')}).locator('summary').click();
 await page.locator('#group-0-tags input[value="a"]').uncheck();
 await page.locator('#field-0-title').fill('Saved immediately');
 await page.getByRole('button',{name:'Save',exact:true}).click();
 await page.getByRole('heading',{name:'Saved',exact:true}).waitFor();
 const out=fs.readFileSync(dir+'/out.yaml','utf8');assert.match(out,/title: Saved immediately/);assert.match(out,/flag: null/);assert.match(out,/tags: \[\]/);assert.match(out,/nested: 5/);
 assert.equal(errors.length,0,errors.join('\n'));
 await page.close();
 fs.writeFileSync(dir+'/enum.view.yaml',`version: 1
schema:
  fields:
    - {name: title, type: text}
    - {name: status, type: enum, required: true, options: [todo]}
    - {name: "odd / name", type: text}
`);
 fs.writeFileSync(dir+'/enum.json',JSON.stringify([{title:'Before',status:'invalid-original','odd / name':'before'}]));
 child=spawn(cwd+'/bin/twikko',['--view',dir+'/enum.view.yaml','--data',dir+'/enum.json','--output',dir+'/enum-out.yaml']);
 const enumURL=await new Promise((resolve,reject)=>{child.stdout.on('data',b=>{const m=String(b).match(/url=(\S+)/);if(m)resolve(m[1])});child.on('error',reject)});
 const second=await browser.newPage();
 await second.goto(enumURL);
 assert.equal(await second.locator('#field-0-status').inputValue(),'invalid-original');
 await second.locator('#field-0-status').focus();
 await second.keyboard.press('ArrowDown');await second.keyboard.press('Enter');
 await second.waitForFunction(()=>!document.querySelector('#status-0-status').textContent.includes('must be'));
 assert.equal(await second.locator('#field-0-status').inputValue(),'todo');
 await second.route('**/mode?value=table',async route=>{await new Promise(r=>setTimeout(r,500));await route.continue()});
 await second.locator('#field-0-title').focus();
 await second.getByRole('button',{name:'Table',exact:true}).click();
 assert.equal(await second.locator('#entities').evaluate(el=>el.inert),true);
 await second.keyboard.type('Cannot edit during replacement');
 await second.locator('table').waitFor();
 assert.equal(await second.locator('#field-0-title').inputValue(),'Before');
 await second.getByRole('textbox',{name:'odd / name',exact:true}).fill('escaped');
 await second.getByRole('button',{name:'Save',exact:true}).click();
 await second.getByRole('heading',{name:'Saved',exact:true}).waitFor();
 const enumOut=fs.readFileSync(dir+'/enum-out.yaml','utf8');assert.match(enumOut,/status: todo/);assert.match(enumOut,/record_index: 0/);assert.match(enumOut,/odd \/ name: escaped/);
 await second.close();
 console.log('PASS UI: native single-option enum correction and transition lock');
 console.log('PASS UI: immediate switch/save, drafts, focus, reset, nullable values, readonly and labels');
 } finally {if(child)child.kill('SIGTERM');await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
