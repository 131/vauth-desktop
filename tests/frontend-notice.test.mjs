import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

test('frontend notice preserves pinned bundled package licenses and ISC attribution', async () => {
  const notice = await readFile('ui/FRONTEND-NOTICE.txt', 'utf8');
  for (const [name, license] of [
    ['vauth-ui', 'LICENSE'], ['svelte', 'LICENSE.md'], ['clsx', 'license'], ['esm-env', 'LICENSE'],
  ]) {
    const root = `node_modules/${name}`;
    const pkg = JSON.parse(await readFile(`${root}/package.json`, 'utf8'));
    assert.ok(notice.includes(`${name === 'svelte' ? 'Svelte' : name} ${pkg.version}`), `${name}: pinned version attribution`);
    assert.ok(notice.includes((await readFile(`${root}/${license}`, 'utf8')).trim()), `${name}: complete license notice`);
  }
  assert.ok(notice.includes((await readFile('LICENSE', 'utf8')).trim()), 'own ISC attribution');
});
