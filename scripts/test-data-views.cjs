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
  async function open(mode, input = data, viewPath = view) {
    const output = path.join(dir, `${mode}-out.yaml`);
    child = spawn(executable, ['--view', viewPath, '--data', input, '--mode', mode, '--output', output]);
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
        assert.equal(await page.locator('th[data-column-key="1"] [data-column-sort]').getAttribute('aria-label') !== null, true);
        const handle = page.locator('[data-column-resize="1"]');
        const box = await handle.boundingBox();
        const neighbor = await page.locator('col[data-column-key="2"]').evaluate(el => el.style.width);
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        await page.mouse.down();
        await page.mouse.move(box.x + box.width / 2 + 80, box.y + box.height / 2, { steps: 6 });
        await page.mouse.up();
        assert.equal(await handle.getAttribute('aria-valuenow'), '280');
        assert.equal(await page.locator('col[data-column-key="2"]').evaluate(el => el.style.width), neighbor);
        await handle.focus(); await handle.press('Home');
        assert.equal(await handle.getAttribute('aria-valuenow'), '96');
        await handle.press('ArrowRight');
        resizedWidth = await page.locator('col[data-column-key="1"]').evaluate(el => el.style.width);
        assert.equal(resizedWidth, '112px');
      }
      await page.locator('#field-0-title').focus();
      await page.evaluate(() => { window.originalEditor = document.querySelector('#field-0-title'); });
      await page.locator('#field-0-title').fill('Focus check');
      await page.waitForFunction(() => document.querySelector('#save-status').textContent.includes('1 changed'));
      assert.equal(await page.evaluate(() => window.originalEditor === document.activeElement), true);
      await page.locator('#field-0-n').fill('1e-');
      await page.waitForFunction(() => document.querySelector('#status-0-n').textContent.includes('number'));
      assert.equal(await page.locator('#field-0-n').inputValue(), '1e-');
      await page.locator('#field-0-due').fill('2026-');
      await page.waitForFunction(() => document.querySelector('#status-0-due').textContent.includes('date'));
      if (mode === 'cards') await page.locator('details[data-menu] summary').click();
      await page.getByRole(mode === 'cards' ? 'menuitem' : 'button', { name: 'Reset', exact: true }).click();
      await page.waitForFunction(() => document.querySelector('#field-0-n').value === '5' && !document.querySelector('.htmx-request,[inert]'));
      if (mode === 'table') {
        assert.equal(await page.locator('col[data-column-key="1"]').evaluate(el => el.style.width), resizedWidth);
        await page.locator('td details').filter({ has: page.locator('#group-0-tags') }).locator('summary').click();
        assert.equal(await page.locator('#group-0-tags').evaluate(el => getComputedStyle(el).borderTopWidth), '0px');
      }
      await page.locator('#group-0-tags input[value="a"]').uncheck();
      await page.locator('#field-0-flag').selectOption('');
      await page.getByRole('textbox', { name: 'odd / name', exact: true }).fill('escaped');
      await page.locator('#field-0-title').fill('Saved immediately');
      await page.getByRole('button', { name: 'Save', exact: true }).click();
      await page.getByRole('heading', { name: 'Saved', exact: true }).waitFor();
      const result = fs.readFileSync(output, 'utf8');
      for (const pattern of [/title: Saved immediately/, /flag: null/, /tags: \[\]/, /nested: 5/, /odd \/ name: escaped/]) assert.match(result, pattern);
      assert.deepEqual(errors, []);
      await page.close();
      console.log(`PASS ${mode}: fixed launch mode, edits, focus, drafts, Reset, Save${mode === 'table' ? ', grid, borderless cells and column resizing' : ''}`);
    }
    // A required single-option enum must remain fixable with native keys.
    fs.writeFileSync(view, `version: 1
schema:
  fields: [{name: status, type: enum, required: true, options: [todo]}]
`);
    fs.writeFileSync(data, JSON.stringify([{status:'invalid-original'}]));
    const {page,output,errors}=await open('cards');
    await page.locator('#field-0-status').focus();
    // Native select popup arrow behavior differs between desktop platforms.
    await page.keyboard.press('t'); await page.keyboard.press('Tab');
    await page.getByRole('button',{name:'Save',exact:true}).click();
    await page.getByRole('heading',{name:'Saved',exact:true}).waitFor();
    assert.match(fs.readFileSync(output,'utf8'),/status: todo/);
    assert.deepEqual(errors,[]);
    await page.close();
    console.log('PASS native single-option enum correction');

    // Keep source identity, decimal precision, and editor nodes across table operations.
    fs.writeFileSync(view, `version: 1
schema:
  fields:
    - {name: title, type: text, required: true}
    - {name: n, type: number}
    - {name: completed, type: bool}
view:
  mode: table
  table:
    timezone: UTC
    row_highlights:
      - intent: warning
        when: {field: n, op: gt, value: 9007199254740992}
      - intent: success
        when: {field: completed, op: eq, value: true}
`);
    fs.writeFileSync(data, `[
      {"title":"Source first","n":9007199254740993,"completed":false},
      {"title":"Source second","n":9007199254740992,"completed":true},
      {"title":"A deliberately long title that makes fitting the column useful","n":9007199254740992.0,"completed":false},
      {"title":"No number","n":null,"completed":false}
    ]`);
    {
      const {page, output, errors} = await open('table');
      const order = () => page.locator('tbody tr').evaluateAll(rows => rows.map(row => Number(row.dataset.entityIndex)));
      const keys = () => page.locator('thead th').evaluateAll(cells => cells.map(cell => cell.dataset.columnKey));
      async function sort(key, direction) {
        const heading = page.locator(`th[data-column-key="${key}"]`);
        await heading.locator('[data-column-sort]').click();
        await page.waitForFunction(({key, direction}) => document.querySelector(`th[data-column-key="${key}"]`).getAttribute('aria-sort') === direction, {key, direction});
      }
      assert.deepEqual(await order(), [0, 1, 2, 3]);
      assert.equal(await page.locator('th[data-column-key="number"]').getAttribute('aria-sort'), 'ascending');
      assert.deepEqual(await page.locator('tbody [data-column-key="number"]').allTextContents(), ['1', '2', '3', '4']);
      assert.equal(await page.locator('tr[data-entity-index="0"]').evaluate(row => row.classList.contains('row-highlight-warning')), true);
      assert.equal(await page.locator('tr[data-entity-index="1"]').evaluate(row => row.classList.contains('row-highlight-success')), true);
      await page.evaluate(() => { window.originalEditor = document.querySelector('#field-0-title'); });
      await sort('1', 'ascending');
      assert.deepEqual(await order(), [1, 2, 0, 3]);
      await sort('1', 'descending');
      assert.deepEqual(await order(), [0, 1, 2, 3]);
      await sort('number', 'ascending');
      assert.deepEqual(await order(), [0, 1, 2, 3]);
      assert.equal(await page.evaluate(() => window.originalEditor === document.querySelector('#field-0-title')), true);

      const resize = page.locator('[data-column-resize="0"]');
      await resize.focus(); await resize.press('Home');
      await resize.press('Enter');
      const fitted = await page.locator('col[data-column-key="0"]').evaluate(col => parseFloat(col.style.width));
      assert.ok(fitted > 96 && fitted <= 900, `Unexpected fitted width ${fitted}`);
      await resize.press('Home');
      await resize.dblclick();
      assert.ok(await page.locator('col[data-column-key="0"]').evaluate(col => parseFloat(col.style.width)) > 96);
      const width = await page.locator('col[data-column-key="0"]').evaluate(col => col.style.width);

      const move = page.locator('[data-column-drag="0"]');
      await move.focus(); await move.press('Space'); await move.press('ArrowRight'); await move.press('Enter');
      assert.deepEqual(await keys(), ['number', '1', '0', '2', 'actions']);
      assert.equal(await page.locator('col[data-column-key="0"]').evaluate(col => col.style.width), width);
      // Escape restores the order before keyboard movement.
      await move.press('Space'); await move.press('ArrowRight'); await move.press('Escape');
      assert.deepEqual(await keys(), ['number', '1', '0', '2', 'actions']);
      const from = await move.boundingBox();
      const to = await page.locator('th[data-column-key="1"]').boundingBox();
      await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
      await page.mouse.down();
      await page.mouse.move(to.x + 4, to.y + to.height / 2, {steps: 8});
      await page.mouse.up();
      assert.deepEqual(await keys(), ['number', '0', '1', '2', 'actions']);
      assert.equal(await page.locator('col[data-column-key="0"]').evaluate(col => col.style.width), width);

      for (const theme of ['light', 'dark']) {
        await page.emulateMedia({colorScheme: theme});
        const cell = page.locator('tr[data-entity-index="1"] td[data-column-key="1"]');
        await cell.hover();
        assert.equal(await cell.getAttribute('data-cell-hover'), 'true');
        const before = await cell.evaluate(el => getComputedStyle(el).backgroundColor);
        await page.keyboard.down('Shift');
        assert.equal(await page.locator('tbody [data-crosshair="true"]').count(), 8);
        assert.notEqual(await cell.evaluate(el => getComputedStyle(el).backgroundColor), before);
        await page.keyboard.up('Shift');
        assert.equal(await page.locator('[data-crosshair="true"]').count(), 0);
      }
      await page.mouse.move(0, 0);
      assert.equal(await page.locator('[data-cell-hover="true"]').count(), 0);

      await move.focus(); await move.press('Space'); await move.press('ArrowRight'); await move.press('Enter');
      await page.locator('#field-0-n').fill('invalid');
      await page.waitForFunction(() => document.querySelector('#status-0-n').textContent.includes('number'));
      await page.waitForFunction(() => !document.querySelector('tr[data-entity-index="0"]').classList.contains('row-highlight-warning'));
      await page.locator('tr[data-entity-index="0"]').getByRole('button', {name: 'Reset', exact: true}).click();
      await page.waitForFunction(() => document.querySelector('#field-0-n').value === '9007199254740993');
      assert.deepEqual(await keys(), ['number', '1', '0', '2', 'actions']);
      assert.deepEqual(await page.locator('tr[data-entity-index="0"] td').evaluateAll(cells => cells.map(cell => cell.dataset.columnKey)), await keys());
      assert.equal(await page.locator('col[data-column-key="0"]').evaluate(col => col.style.width), width);
      await page.waitForFunction(() => document.querySelector('tr[data-entity-index="0"]').classList.contains('row-highlight-warning'));
      await sort('1', 'ascending');
      await page.locator('#field-0-title').fill('Changed source first');
      await page.locator('#field-0-n').fill('1');
      await page.waitForFunction(() => document.querySelector('#save-status').textContent.includes('2 changed'));
      assert.deepEqual(await order(), [1, 2, 0, 3]);
      // Sorting immediately after input must wait for the queued edit.
      await page.locator('#field-0-n').fill('9007199254740994');
      await sort('1', 'descending');
      assert.deepEqual(await order(), [0, 1, 2, 3]);
      await page.getByRole('button', {name: 'Save', exact: true}).click();
      await page.getByRole('heading', {name: 'Saved', exact: true}).waitFor();
      const result = fs.readFileSync(output, 'utf8');
      assert.match(result, /record_index: 0/);
      assert.ok(result.indexOf('title: Changed source first') < result.indexOf('title: Source second'));
      assert.match(result, /9007199254740992\.0/);
      assert.deepEqual(errors, []);
      await page.close();
      console.log('PASS table interactions: exact stable sorting, source numbering, auto-fit, pointer/keyboard movement, themes, highlights, Reset and Save identity');
    }

    // A visible-page refresh recalculates relative rules through the edit queue.
    fs.writeFileSync(view, `version: 1
schema:
  fields: [{name: due, type: date}]
view:
  mode: table
  table:
    timezone: UTC
    row_highlights:
      - intent: warning
        when:
          field: due
          measure: {distance_from: today, unit: days}
          op: lt
          value: 0
`);
    fs.writeFileSync(data, '[{"due":"2000-01-01"}]');
    {
      const {page, errors} = await open('table');
      const refreshed = page.waitForResponse(response => response.url().endsWith('/table/highlights'));
      await page.evaluate(() => document.dispatchEvent(new Event('visibilitychange')));
      assert.equal((await refreshed).status(), 200);
      await page.waitForFunction(() => document.querySelector('#entity-0').classList.contains('row-highlight-warning'));
      assert.deepEqual(errors, []);
      await page.getByRole('button', {name: 'Cancel', exact: true}).click();
      await page.getByRole('heading', {name: 'Cancelled', exact: true}).waitFor();
      await page.close();
      console.log('PASS relative highlights refresh');
    }

    {
      const input = path.join(process.cwd(), 'examples', 'orders.csv');
      const orderView = path.join(process.cwd(), 'examples', 'orders.view.yaml');
      const {page, output, errors} = await open('table', input, orderView);
      assert.equal(await page.locator('tbody tr').count(), 500);
      assert.equal(await page.locator('tbody tr').first().getAttribute('data-entity-index'), '0');
      assert.equal(await page.locator('tbody tr').last().getAttribute('data-entity-index'), '499');
      await page.locator('[data-column-sort="3"]').click();
      await page.waitForFunction(() => document.querySelector('th[data-column-key="3"]').getAttribute('aria-sort') === 'ascending');
      const prices = await page.locator('tbody [data-column-key="3"] input[name="value"]').evaluateAll(inputs => inputs.map(input => Number(input.value)));
      assert.equal(prices.length, 500);
      assert.deepEqual(prices, [...prices].sort((a, b) => a - b));
      await page.locator('[data-column-sort="number"]').click();
      await page.waitForFunction(() => document.querySelector('th[data-column-key="number"]').getAttribute('aria-sort') === 'ascending');
      assert.deepEqual(await page.locator('tbody tr').evaluateAll(rows => rows.map(row => Number(row.dataset.entityIndex))), Array.from({length: 500}, (_, i) => i));
      const handle = page.locator('[data-column-resize="6"]');
      await handle.focus(); await handle.press('Enter');
      assert.ok(await page.locator('col[data-column-key="6"]').evaluate(col => parseFloat(col.style.width)) >= 96);
      await page.getByRole('button', {name: 'Save', exact: true}).click();
      await page.getByRole('heading', {name: 'Saved', exact: true}).waitFor();
      const result = fs.readFileSync(output, 'utf8');
      assert.match(result, /changes: \[\]/);
      assert.equal((result.match(/id: ORD-/g) || []).length, 500);
      assert.deepEqual(errors, []);
      await page.close();
      console.log('PASS 500 orders: all rows, price sorting, file-order restoration, auto-fit and unchanged Save');
    }
  } finally {
    if (child) child.kill('SIGTERM');
    await browser.close();
  }
})().catch(error => { console.error(error); process.exit(1); });
