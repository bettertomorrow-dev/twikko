// Run after make build with Playwright available through NODE_PATH.
const { chromium } = require('playwright');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert/strict');

(async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'twikko-keyboard-'));
  const view = path.join(dir, 'view.yaml');
  const data = path.join(dir, 'data.json');
  fs.writeFileSync(view, `version: 1
schema:
  fields:
    - {name: status, type: enum, options: [open, active, done]}
    - {name: title, type: text}
    - {name: notes, type: longtext}
    - {name: due, type: date}
    - {name: reference, type: text, readonly: true}
    - {name: enabled, type: bool, required: true}
view:
  cards:
    arrow_field: status
    header: [{source: status}]
`);
  fs.writeFileSync(data, JSON.stringify([
    {status: 'open', title: 'First', notes: 'Notes', due: '2026-11-01', reference: 'A', enabled: true},
    {status: 'active', title: 'Second', notes: 'More', due: '2026-11-02', reference: 'B', enabled: false}
  ]));
  const browser = await chromium.launch({headless: true});
  let child;
  async function open(mode) {
    child = spawn(path.resolve('bin/twikko'), ['--view', view, '--data', data, '--output', path.join(dir, mode + '.yaml'), '--mode', mode]);
    const url = await new Promise((resolve, reject) => {
      child.stdout.on('data', b => { const m = String(b).match(/url=(\S+)/); if (m) resolve(m[1]); });
      child.on('exit', code => reject(Error('Exited before URL: ' + code)));
    });
    const page = await browser.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(url);
    return {page, errors};
  }
  async function focused(page, selector) {
    assert.equal(await page.locator(selector).evaluate(el => document.activeElement === el), true, selector);
  }
  try {
    let {page, errors} = await open('cards');
    const card = page.locator('#entity-0');
    await page.keyboard.press('ArrowUp');
    await focused(page, '#card-nav-0');
    await page.reload();
    await page.keyboard.press('ArrowDown');
    await focused(page, '#card-nav-0');
    for (const colorScheme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme });
      const outline = await card.evaluate(el => {
        const style = getComputedStyle(el);
        const swatch = document.createElement('span');
        swatch.style.color = 'var(--info)';
        el.append(swatch);
        const matchesTheme = style.outlineColor === getComputedStyle(swatch).color;
        swatch.remove();
        return [style.outlineStyle, style.outlineWidth, matchesTheme];
      });
      assert.deepEqual(outline, ['solid', '2px', true]);
    }
    assert.equal(await page.locator('#card-nav-0').evaluate(el => getComputedStyle(el).outlineStyle), 'none');
    await page.keyboard.press('ArrowDown');
    await focused(page, '#card-nav-1');
    assert.equal(await card.evaluate(el => getComputedStyle(el).outlineStyle), 'none');
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('ArrowRight');
    await page.waitForFunction(() => document.querySelector('#field-0-status').value === 'active');
    await page.keyboard.press('ArrowRight');
    await page.waitForFunction(() => document.querySelector('#field-0-status').value === 'done');
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+z' : 'Control+z');
    await page.waitForFunction(() => document.querySelector('#field-0-status').value === 'active');
    await focused(page, '#card-nav-0');
    await page.keyboard.press('Shift+Backspace');
    await page.waitForFunction(() => document.querySelector('#entity-0 [data-card-toggle]').getAttribute('aria-expanded') === 'false');
    await focused(page, '#card-nav-0');
    await page.keyboard.press('Shift+Backspace');
    await page.waitForFunction(() => document.querySelector('#entity-0 [data-card-toggle]').getAttribute('aria-expanded') === 'true');
    await focused(page, '#card-nav-0');
    await page.keyboard.press('Shift+Backspace');
    await page.waitForFunction(() => document.querySelector('#entity-0 [data-card-toggle]').getAttribute('aria-expanded') === 'false');
    await page.keyboard.press('Enter');
    await page.waitForFunction(() => document.querySelector('#entity-0 [data-card-toggle]').getAttribute('aria-expanded') === 'true');
    await focused(page, '#field-0-status');
    const first = card.locator('[data-card-toggle]');
    const last = card.locator('#field-0-enabled');
    const menu = card.locator('details[data-menu] > summary');
    await first.focus();
    await page.keyboard.press('Shift+Tab');
    assert.equal(await last.evaluate(el => document.activeElement === el), true);
    await page.keyboard.press('Tab');
    assert.equal(await first.evaluate(el => document.activeElement === el), true);
    await menu.focus();
    await page.keyboard.press('Enter');
    await page.keyboard.press('Escape');
    assert.equal(await menu.evaluate(el => document.activeElement === el), true);
    await page.locator('#field-0-due').focus();
    await page.keyboard.press('ArrowDown');
    await page.waitForSelector('[data-picker-anchor]');
    await page.locator('[data-picker-anchor] button').first().click();
    assert.equal(await card.getAttribute('data-isolated'), '');
    await page.keyboard.press('Tab');
    assert.equal(await card.getAttribute('data-isolated'), '');
    await page.locator('#field-0-due').focus();
    await page.keyboard.press('ArrowDown');
    await page.waitForSelector('[data-picker-anchor]');
    await page.keyboard.press('Escape');
    await focused(page, '#field-0-due');
    assert.equal(await card.getAttribute('data-isolated'), '');
    await page.keyboard.press('Escape');
    await focused(page, '#card-nav-0');
    await page.keyboard.press('Enter');
    await page.locator('#field-0-title').focus();
    await page.locator('#field-0-title').evaluate(el => el.setSelectionRange(el.value.length, el.value.length));
    await page.keyboard.press('Shift+Backspace');
    await focused(page, '#field-0-title');
    await page.keyboard.press('Backspace');
    assert.notEqual(await page.locator('#field-0-title').inputValue(), 'First');
    await page.locator('#field-0-enabled').focus();
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({colorScheme: scheme});
      const outline = await page.locator('#field-0-enabled + span').evaluate(el => getComputedStyle(el).outlineWidth);
      assert.equal(outline, '2px');
    }
    await page.keyboard.press('Escape');
    await focused(page, '#card-nav-0');
    assert.deepEqual(errors, []);
    await page.close();
    child.kill();

    ({page, errors} = await open('table'));
    const edits = [];
    page.on('request', request => {
      if (request.method() === 'POST' && request.url().includes('/fields/')) edits.push(request.postData());
    });
    await focused(page, '#cell-0-0');
    assert.equal(await page.locator('#cell-0-0').evaluate(el => getComputedStyle(el).outlineWidth), '2px');
    await page.keyboard.press('ArrowDown');
    await focused(page, '#entity-1 [data-column-key="0"]');
    await page.keyboard.press('ArrowRight');
    await focused(page, '#entity-1 [data-column-key="1"]');
    await page.keyboard.press('Enter');
    await focused(page, '#field-1-title');
    assert.deepEqual(await page.locator('#field-1-title').evaluate(el => [el.selectionStart, el.selectionEnd]), [0, 6]);
    await page.keyboard.press('Home');
    await page.keyboard.press('ArrowRight');
    assert.equal(await page.locator('#field-1-title').evaluate(el => el.selectionStart), 1);
    await page.locator('#field-1-title').fill('Discard this');
    await page.keyboard.press('Escape');
    await focused(page, '#entity-1 [data-column-key="1"]');
    assert.equal(await page.locator('#field-1-title').inputValue(), 'Second');
    await page.keyboard.press('Enter');
    await page.keyboard.insertText('Keep this');
    await page.keyboard.press('Enter');
    await focused(page, '#entity-1 [data-column-key="1"]');
    await page.waitForFunction(() => document.querySelector('#status-1-title').dataset.changed === 'true');
    assert.equal(edits.length, 1);
    assert.equal(new URLSearchParams(edits[0]).get('value'), 'Keep this');
    await page.keyboard.press('Tab');
    assert.equal(await page.locator('#field-1-title').evaluate(el => document.activeElement === el), false);
    await page.locator('#field-1-title').click();
    assert.equal(await page.locator('#field-1-title').evaluate(el => el.selectionStart === el.selectionEnd), true);
    await page.keyboard.press('Escape');
    await page.locator('#entity-0 [data-column-key="2"]').focus();
    await page.keyboard.press('Enter');
    await focused(page, '#field-0-notes');
    assert.deepEqual(await page.locator('#field-0-notes').evaluate(el => [el.selectionStart, el.selectionEnd]), [0, 5]);
    await page.keyboard.press('End');
    await page.keyboard.press('Shift+Enter');
    assert.equal(await page.locator('#field-0-notes').inputValue(), 'Notes\n');
    await page.keyboard.press('Escape');
    await focused(page, '#entity-0 [data-column-key="2"]');
    assert.equal(await page.locator('#field-0-notes').inputValue(), 'Notes');
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Enter');
    await page.locator('#field-0-due').fill('2026-11-28');
    await page.keyboard.press('ArrowDown');
    await page.waitForSelector('[data-picker-anchor]');
    await page.keyboard.press('Escape');
    await focused(page, '#cell-0-3');
    assert.equal(await page.locator('#field-0-due').inputValue(), '2026-11-01');
    assert.equal(await page.locator('#cell-0-3 input[type="hidden"]').inputValue(), '2026-11-01');
    assert.equal(await page.locator('[data-picker-anchor]').count(), 0);
    await page.keyboard.press('ArrowRight');
    await focused(page, '#entity-0 [data-column-key="4"]');
    await page.keyboard.press('Enter');
    await focused(page, '#entity-0 [data-column-key="4"]');
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Enter');
    await page.keyboard.press('Space');
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('#field-0-enabled').isChecked(), true);
    assert.equal(edits.length, 1);
    await page.keyboard.press('Enter');
    await page.keyboard.press('Space');
    await page.keyboard.press('Enter');
    await focused(page, '#cell-0-5');
    await page.waitForFunction(() => document.querySelector('#status-0-enabled').dataset.changed === 'true');
    assert.equal(new URLSearchParams(edits[1]).get('value'), 'false');
    assert.deepEqual(errors, []);
    await page.close();
  } finally {
    if (child) child.kill();
    await browser.close();
    fs.rmSync(dir, {recursive: true, force: true});
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
