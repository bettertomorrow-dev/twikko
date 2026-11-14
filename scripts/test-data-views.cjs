// Optional regression check. Build bin/twikko and install Playwright externally.
// NODE_PATH=/path/to/node_modules node scripts/test-data-views.cjs
const { chromium } = require('playwright');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const assert = require('node:assert/strict');

(async () => {
  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox'] });
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'twikko-ui-'));
  const view = path.join(dir, 'view.yaml');
  const data = path.join(dir, 'data.json');
  fs.writeFileSync(view, `version: 1
schema:
  id_field: id
  fields:
    - {name: id, type: text}
    - {name: title, type: text, required: true}
    - {name: n, type: number}
    - {name: due, type: date}
    - {name: flag, type: bool}
    - {name: enabled, type: bool, required: true}
    - {name: tags, type: multienum, options: [a, b]}
    - {name: notes, type: longtext}
    - {name: "odd / name", type: text}
view:
  mode: cards
  show_labels: false
`);
  fs.writeFileSync(data, JSON.stringify([{ id: 'PROJ-101', title: 'Before', n: 5,
    due: '2026-11-13', flag: true, enabled: false, tags: ['a'], notes: 'Text',
    'odd / name': 'before', hidden: { nested: 5 } }]));
  let child;
  const executable = path.join(process.cwd(), 'bin', process.platform === 'win32' ? 'twikko.exe' : 'twikko');
  async function open(mode, input = data) {
    const output = path.join(dir, `${mode}-out.yaml`);
    child = spawn(executable, ['--view', view, '--data', input, '--mode', mode, '--output', output]);
    const url = await new Promise((resolve, reject) => {
      child.stdout.on('data', b => { const m = String(b).match(/url=(\S+)/); if (m) resolve(m[1]); });
      child.on('error', reject);
      child.on('exit', code => reject(Error(`Exited before printing URL: ${code}`)));
    });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    const errors = [];
    page.on('pageerror', e => errors.push(String(e)));
    page.on('console', m => { if (m.type() === 'error') errors.push(m.text()); });
    await page.route('**/entities/**/fields/**', async route => {
      await new Promise(resolve => setTimeout(resolve, 80));
      await route.continue();
    });
    await page.goto(url);
    return { page, output, errors };
  }
  try {
    for (const mode of ['cards', 'table']) {
      const { page, output, errors } = await open(mode);
      assert.equal(await page.locator('#mode-switch,[data-mode-button]').count(), 0);
      assert.equal(await page.locator('table').count(), mode === 'table' ? 1 : 0);
      assert.equal(await page.locator('#field-0-id').isDisabled(), true);
      const actionBar = page.locator('#save-bar');
      for (const [corner, x, y] of [['top-left', 30, 30], ['top-right', 1400, 30], ['bottom-left', 30, 870], ['bottom-right', 1400, 870]]) {
        const box = await page.locator('[data-action-drag]').boundingBox();
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        await page.mouse.down();
        await page.mouse.move(x, y, { steps: 8 });
        await page.mouse.up();
        assert.equal(await actionBar.getAttribute('data-corner'), corner);
        assert.equal(await actionBar.locator('details').getAttribute('open'), null);
        const snapped = await actionBar.boundingBox();
        assert.ok(Math.abs((corner.endsWith('left') ? snapped.x : 1440 - snapped.x - snapped.width) - 20) < 1);
        assert.ok(Math.abs((corner.startsWith('top') ? snapped.y : 900 - snapped.y - snapped.height) - 20) < 1);
        await actionBar.locator('summary').click();
        const menu = await page.locator('[data-action-menu]').boundingBox();
        assert.ok(menu.x >= 0 && menu.y >= 0 && menu.x + menu.width <= 1440 && menu.y + menu.height <= 900);
        await actionBar.locator('summary').click();
      }
      await actionBar.locator('summary').focus();
      await page.keyboard.press('Alt+ArrowUp');
      assert.equal(await actionBar.getAttribute('data-corner'), 'top-right');
      await page.keyboard.press('Alt+ArrowDown');

      await page.locator('#save-bar summary').click();
      await page.locator('#save-bar button[data-confirm-reset]').click();
      await page.waitForFunction(() => !document.querySelector('[inert]'));
      assert.equal(await page.locator('#reset-confirmation').isVisible(), false);
      await page.locator('#save-bar summary').click();
      assert.equal(await page.getByRole('switch', { name: 'enabled', exact: true }).count(), 1);
      let resizedWidth;
      if (mode === 'table') {
        const cellStyle = await page.locator('td').first().evaluate(el => {
          const s = getComputedStyle(el); return [s.borderTopWidth, s.borderRightWidth];
        });
        assert.deepEqual(cellStyle, ['1px', '1px']);
        for (const selector of ['#field-0-title', '#field-0-due', '#field-0-flag']) {
          assert.equal(await page.locator(selector).evaluate(el => getComputedStyle(el).borderTopWidth), '0px');
        }
        assert.equal(await page.locator('th .sr-only').count(), 9);
        const handle = page.locator('[data-column-resize="1"]');
        const box = await handle.boundingBox();
        const neighbor = await page.locator('col').nth(2).evaluate(el => el.style.width);
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        await page.mouse.down();
        await page.mouse.move(box.x + box.width / 2 + 80, box.y + box.height / 2, { steps: 6 });
        await page.mouse.up();
        assert.equal(await handle.getAttribute('aria-valuenow'), '280');
        assert.equal(await page.locator('col').nth(2).evaluate(el => el.style.width), neighbor);
        await handle.focus(); await handle.press('Home');
        assert.equal(await handle.getAttribute('aria-valuenow'), '96');
        await handle.press('ArrowRight');
        resizedWidth = await page.locator('col').nth(1).evaluate(el => el.style.width);
        assert.equal(resizedWidth, '112px');
      }
      await page.locator('#field-0-title').focus();
      await page.evaluate(() => { window.originalEditor = document.querySelector('#field-0-title'); });
      await page.locator('#field-0-title').fill('Focus check');
      await page.waitForFunction(() => document.querySelector('#save-status').textContent.includes('1 changed'));
      assert.equal(await page.evaluate(() => window.originalEditor === document.activeElement), true);
      const modifier = process.platform === 'darwin' ? 'Meta' : 'Control';
      await page.keyboard.press(`${modifier}+z`);
      await page.waitForFunction(() => document.querySelector('#field-0-title').value === 'Before' && !document.querySelector('[inert]'));
      await page.keyboard.press(`${modifier}+Shift+z`);
      await page.waitForFunction(() => document.querySelector('#field-0-title').value === 'Focus check' && !document.querySelector('[inert]'));
      await page.waitForFunction(() => document.activeElement.id === 'field-0-title');
      await page.locator('#field-0-title').press('End');
      await page.locator('#field-0-title').pressSequentially(' typed');
      await page.locator('#save-bar summary').click();
      await page.getByRole('button', { name: 'Reset', exact: true }).last().click();
      await page.locator('#reset-confirmation button[value=cancel]').click();
      assert.equal(await page.locator('#field-0-title').inputValue(), 'Focus check typed');
      await page.locator('#save-bar summary').click();
      await page.getByRole('button', { name: 'Reset', exact: true }).last().click();
      await page.locator('#reset-confirmation button[value=confirm]').click();
      await page.waitForFunction(() => document.querySelector('#field-0-title').value === 'Before' && !document.querySelector('[inert]'));
      await page.keyboard.press(`${modifier}+z`);
      await page.waitForFunction(() => document.querySelector('#field-0-title').value === 'Focus check typed' && !document.querySelector('[inert]'));
      await page.keyboard.press(`${modifier}+z`);
      await page.waitForFunction(() => document.querySelector('#field-0-title').value === 'Focus check' && !document.querySelector('[inert]'));
      if (mode === 'table') assert.equal(await page.locator('col').nth(1).evaluate(el => el.style.width), resizedWidth);
      await page.reload();
      await page.keyboard.press(`${modifier}+z`);
      await page.waitForFunction(() => document.querySelector('#field-0-title').value === 'Before' && !document.querySelector('[inert]'));
      if (mode === 'table') resizedWidth = '200px';
      await page.locator('#field-0-n').fill('1e-');
      await page.waitForFunction(() => document.querySelector('#status-0-n').textContent.includes('number'));
      assert.equal(await page.locator('#field-0-n').inputValue(), '1e-');
      await page.locator('#field-0-due').fill('2026-');
      await page.waitForFunction(() => document.querySelector('#status-0-due').textContent.includes('date'));
      if (mode === 'cards') await page.locator('#entity-0 details[data-menu] summary').click();
      await page.getByRole(mode === 'cards' ? 'menuitem' : 'button', { name: 'Reset', exact: true }).click();
      await page.locator('#reset-confirmation button[value=confirm]').click();
      await page.waitForFunction(() => document.querySelector('#field-0-n').value === '5' && !document.querySelector('.htmx-request,[inert]'));
      if (mode === 'table') {
        assert.equal(await page.locator('col').nth(1).evaluate(el => el.style.width), resizedWidth);
        await page.locator('td details').filter({ has: page.locator('#group-0-tags') }).locator('summary').click();
        assert.equal(await page.locator('#group-0-tags').evaluate(el => getComputedStyle(el).borderTopWidth), '0px');
      }
      if (mode === 'table') await page.locator('td details').filter({ has: page.locator('#field-0-notes') }).locator('summary').click();
      await page.locator('#field-0-notes').fill('Longer note');
      await page.keyboard.press(`${modifier}+z`);
      await page.waitForFunction(() => document.querySelector('#field-0-notes').value === 'Text' && document.activeElement.id === 'field-0-notes');
      await page.keyboard.press(`${modifier}+Shift+z`);
      await page.waitForFunction(() => document.querySelector('#field-0-notes').value === 'Longer note' && document.activeElement.id === 'field-0-notes');
      await page.locator('#group-0-tags input[value="a"]').uncheck();
      await page.locator('#field-0-flag').selectOption('');
      await page.getByRole('textbox', { name: 'odd / name', exact: true }).fill('escaped');
      await page.locator('#field-0-title').fill('Saved immediately');
      await page.locator('#save-bar summary').click();
      await page.getByRole('button', { name: 'Save', exact: true }).click();
      await page.getByRole('heading', { name: 'Saved', exact: true }).waitFor();
      const result = fs.readFileSync(output, 'utf8');
      for (const pattern of [/title: Saved immediately/, /flag: null/, /tags: \[\]/, /nested: 5/, /odd \/ name: escaped/]) assert.match(result, pattern);
      assert.deepEqual(errors, []);
      await page.close();
      console.log(`PASS ${mode}: fixed launch mode, edits, focus, drafts, Reset, Save, undo/redo, confirmation, history after reload${mode === 'table' ? ', grid, borderless cells and column resizing' : ''}`);
    }
    // A required single-option enum must remain fixable with native keys.
    fs.writeFileSync(view, `version: 1
schema:
  fields: [{name: status, type: enum, required: true, options: [todo]}]
`);
    fs.writeFileSync(data, JSON.stringify([{status:'invalid-original'}]));
    const {page,output,errors}=await open('cards');
    await page.locator('#field-0-status').focus();
    await page.keyboard.press('t'); await page.keyboard.press('Tab');
    await page.locator('#save-bar summary').click();
    await page.getByRole('button',{name:'Save',exact:true}).click();
    await page.getByRole('heading',{name:'Saved',exact:true}).waitFor();
    assert.match(fs.readFileSync(output,'utf8'),/status: todo/);
    assert.deepEqual(errors,[]);
    await page.close();
    console.log('PASS native single-option enum correction');
  } finally {
    if (child) child.kill('SIGTERM');
    await browser.close();
  }
})().catch(error => { console.error(error); process.exit(1); });
