import { test } from 'node:test';
import assert from 'node:assert/strict';
import { build } from 'esbuild';
import { sveltePlugin } from '../scripts/svelte-plugin.mjs';

test('Svelte component styles are emitted as external CSS, not runtime injection', async () => {
  const result = await build({
    entryPoints: ['tests/Styled.svelte'], bundle: true, write: false,
    outfile: 'ui/generated/fixture.js', plugins: [sveltePlugin()],
  });
  const css = result.outputFiles.find(file => file.path.endsWith('.css'))?.text;
  const js = result.outputFiles.find(file => file.path.endsWith('.js')).text;
  assert.match(css, /color: rgb\(11, 22, 33\)/);
  const scopedClass = css.match(/p\.(svelte-[\w-]+)/)[1];
  assert.ok(js.includes(scopedClass), 'DOM and external CSS share the scoped selector');
  assert.ok(!js.includes('color: rgb(11, 22, 33)'), 'styles are not injected into the document');
});
