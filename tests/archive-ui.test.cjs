const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { chromium } = require('playwright');

for (const viewport of [{ width: 390, height: 844 }, { width: 1280, height: 900 }]) {
 test(`request documents flow ${viewport.width}px`, async () => {
  const browser = await chromium.launch({ headless: true, ...(process.env.BROWSER_EXECUTABLE ? { executablePath: process.env.BROWSER_EXECUTABLE } : {}) });
  try {
   const page = await browser.newPage({ viewport, acceptDownloads: true });
   const errors = [];
   page.on('pageerror', e => errors.push(e.message));
   page.setDefaultTimeout(10000);
   page.on('dialog', dialog => dialog.accept());
   let saved = false;
   let destination;
   const req = () => ({ id: 1, uid: 'REQ-1', title: 'Тестовая заявка', status: 'in_progress', created_by: 7, created_name: 'Alice', created_at: '2026-10-06T10:00:00Z', files_count: 1, messages_count: 1, files: [], payments: [], approvals: [], appeals: [], document_id: saved ? 100 : 0, document_by: saved ? 7 : 0, document_place: saved ? 'Acme · внешний контур · Счета' : '' });
   const file = () => ({ id: 100, request_id: 1, request_uid: 'REQ-1', request_title: 'Тестовая заявка', name: 'REQ-1.zip', mime: 'application/zip', created_by: 7, created_name: 'Alice', created_at: '2026-10-06T10:00:00Z', mine: true });
   await page.route('**/*', async route => {
    const url = new URL(route.request().url());
    if (url.hostname !== 'miniapp.test') return route.fulfill({ body: '', contentType: 'text/javascript' });
    const method = route.request().method();
    let data = {};
    if (url.pathname === '/api/me') data = { id: 7, name: 'Alice', owner: true, sections: ['requests', 'documents'], unread: {} };
    else if (url.pathname === '/api/directory') data = { companies: [{ id: 20, name: 'Acme' }], managers: [], clients: [], counterparties: [] };
    else if (url.pathname === '/api/requests/archive') { destination = route.request().postDataJSON(); saved = true; data = { ok: true }; }
    else if (url.pathname === '/api/requests') data = url.searchParams.has('id') ? req() : [req()];
    else if (url.pathname === '/api/documents') {
     if (method === 'DELETE') { saved = false; data = { ok: true }; }
     else if (url.searchParams.has('destinations')) data = [{ scope: 'company', owner_id: 20, folder: 'external:21', label: 'Acme · внешний контур · Счета' }, { scope: 'misc', owner_id: 0, folder: 'other', label: 'Прочее' }];
     else if (url.searchParams.has('archive')) data = { file: file(), report: 'Сообщение заявки <script>bad()</script>\nОплата: 123 USD', entries: [{ path: 'files/2_invoice.txt', name: 'invoice.txt', mime: 'text/plain', size: 14 }] };
     else if (url.searchParams.has('id')) return route.fulfill({ contentType: url.searchParams.has('entry') ? 'text/plain' : 'application/zip', body: url.searchParams.has('entry') ? 'Текст вложения' : 'zip-fixture' });
     else data = { companies: [{ id: 20, name: 'Acme' }], counterparties: [], tabs: [{ id: 21, company_id: 20, contour: 'external', name: 'Счета' }], files: saved ? [file()] : [], can_edit: true };
    } else if (!url.pathname.startsWith('/api/')) {
     const name = url.pathname === '/' ? 'index.html' : path.basename(url.pathname);
     return route.fulfill({ body: await fs.readFile(path.join(__dirname, '..', 'web', name)), contentType: name.endsWith('.js') ? 'text/javascript' : name.endsWith('.css') ? 'text/css' : 'text/html' });
    }
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(data) });
   });
   await page.goto('http://miniapp.test/?go=requests');
   await page.locator('[data-openreq="1"]').click();
   await page.getByRole('button', { name: 'Сохранить в документы', exact: true }).click();
   await page.locator('#archiveDestination').selectOption('0');
   await page.getByRole('button', { name: 'Сохранить всё', exact: true }).click();
   await page.locator('[data-document-delete="100"]').waitFor();
   assert.equal(destination.folder, 'external:21');
   assert.equal(destination.owner_id, 20);
   await page.locator('[data-document-view="100"]').click();
   await page.locator('.archive-report').waitFor();
   assert.match(await page.locator('.archive-report').textContent(), /<script>bad/);
   await page.locator('[data-entry-view="0"]').click();
   await page.getByText('Текст вложения', { exact: true }).waitFor();
   await page.locator('dialog').last().getByRole('button', { name: 'Закрыть', exact: true }).click();
   const download = page.waitForEvent('download');
   await page.locator('[data-entry-download="0"]').click();
   assert.equal((await download).suggestedFilename(), 'invoice.txt');
   const all = page.waitForEvent('download');
   await page.locator('[data-download-all]').click();
   assert.equal((await all).suggestedFilename(), 'REQ-1.zip');
   if (process.env.SCREENSHOT_DIR) await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, `archive-${viewport.width}.png`) });
   const bounds = await page.locator('dialog').boundingBox();
   assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= viewport.width + 1, 'modal overflow');
   await page.locator('dialog').getByRole('button', { name: 'Закрыть', exact: true }).click();
   await page.locator('[data-document-delete="100"]').click();
   await page.locator('[data-arc="1"]').waitFor();
   assert.equal(saved, false);
   // Save from the list as well, then remove through Documents.
   await page.goto('http://miniapp.test/?go=requests');
   await page.locator('[data-arc="1"]').click();
   await page.getByRole('button', { name: 'Сохранить всё', exact: true }).click();
   await page.locator('[data-document-view="100"]').waitFor();
   await page.goto('http://miniapp.test/');
   await page.locator('[data-go="documents"]').click();
   await page.locator('#docCo').click();
   await page.locator('[data-co="20"]').click();
   await page.locator('[data-fold="external"]').click();
   await page.locator('[data-subfold="external:21"]').click();
   await page.locator('[data-vopen="100"]').click();
   await page.locator('.archive-report').waitFor();
   await page.locator('[data-remove]').click();
   await page.getByText('Пока пусто', { exact: false }).waitFor();
   assert.equal(saved, false);
   assert.deepEqual(errors, []);
  } finally { await browser.close(); }
 });
}
