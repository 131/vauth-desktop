import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

test('version hook stages target lock roots before package marker changes', async () => {
  const old = { version: '2.3.5', packages: { '': { version: '2.3.5' }, 'node_modules/svelte': { version: '5.57.3' } } };
  let written, staged;
  vm.runInNewContext(await readFile('scripts/version-lock.cjs', 'utf8'), {
    process: { env: { npm_package_version: '2.4.0' } },
    require(name) {
      if (name === 'node:fs') return {
        readFileSync: () => JSON.stringify(old),
        writeFileSync: (file, contents) => { assert.equal(file, 'package-lock.json'); written = JSON.parse(contents); },
      };
      if (name === 'node:child_process') return { execFileSync: (command, args) => { staged = [command, ...args]; } };
      throw new Error(`Unexpected module ${name}`);
    },
  });
  assert.equal(written.version, '2.4.0');
  assert.equal(written.packages[''].version, '2.4.0');
  assert.equal(written.packages['node_modules/svelte'].version, '5.57.3');
  assert.deepEqual(staged, ['git', 'add', 'package-lock.json']);
  assert.equal(old.version, '2.3.5', 'root package marker is still the previous version');
});
