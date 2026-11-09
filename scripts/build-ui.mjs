import { build } from 'esbuild';
import { sveltePlugin } from './svelte-plugin.mjs';
import { readFile, writeFile, mkdir } from 'node:fs/promises';

// Local library symlinks are supported; the plugin deduplicates their runtime.
const result = await build({
  entryPoints: ['src/main.js'], bundle: true, format: 'iife', target: 'es2020',
  outfile: 'ui/generated/app.js', write: false, minify: true, metafile: true,
  preserveSymlinks: true, loader: { '.js': 'js' },
  plugins: [sveltePlugin(), { name: 'native-controller', setup(builder) {
    builder.onLoad({ filter: /[/\\]ui[/\\]app\.js$/ }, async ({ path: filename }) => ({ contents: `export default ${JSON.stringify(await readFile(filename, 'utf8'))}` }));
  } }],
});
const runtimes = Object.keys(result.metafile.inputs).filter(name => name.endsWith('/internal/client/dom/operations.js'));
if (runtimes.length !== 1) throw new Error(`Expected one shared Svelte runtime, found ${runtimes.length}`);
const notice = await readFile('ui/FRONTEND-NOTICE.txt', 'utf8');
await mkdir('ui/generated', { recursive: true });
for (const file of result.outputFiles) {
  // Script/style end tags must never terminate an inline asset in Go's document.
  const text = file.path.endsWith('.js') ? `/*!\n${notice.replace(/\*\//g, '* /')}*/\n${file.text}` : file.text;
  const contents = text.replace(/<\/(script|style)/gi, '<\\/$1');
  if (process.argv.includes('--check')) {
    if (await readFile(file.path, 'utf8') !== contents) throw new Error(`Stale generated asset: ${file.path}; run npm run build`);
  } else await writeFile(file.path, contents);
}
