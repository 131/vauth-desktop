import { compile } from 'svelte/compiler';
import { readFile } from 'node:fs/promises';
import path from 'node:path';

export function sveltePlugin() {
  const styles = new Map();
  return { name: 'svelte', setup(builder) {
    builder.onResolve({ filter: /^svelte(?:\/|$)/ }, args => {
      if (args.pluginData?.deduped) return;
      return builder.resolve(args.path, { kind: args.kind, resolveDir: process.cwd(), pluginData: { deduped: true } });
    });
    builder.onLoad({ filter: /\.svelte$/, namespace: 'file' }, async ({ path: filename }) => {
      const source = await readFile(filename, 'utf8');
      const { js, css, warnings } = compile(source, {
        filename: path.relative(process.cwd(), filename), generate: 'client', css: 'external',
      });
      for (const warning of warnings) console.warn(`${filename}: ${warning.code}: ${warning.message}`);
      let contents = js.code;
      if (css?.code) {
        styles.set(filename, css.code);
        contents += `\nimport ${JSON.stringify('svelte-css:' + filename)};`;
      }
      return { contents, resolveDir: path.dirname(filename) };
    });
    builder.onResolve({ filter: /^svelte-css:/ }, ({ path: name }) => ({ path: name.slice('svelte-css:'.length), namespace: 'svelte-css' }));
    builder.onLoad({ filter: /.*/, namespace: 'svelte-css' }, ({ path: filename }) => ({ contents: styles.get(filename), loader: 'css', resolveDir: path.dirname(filename) }));
  } };
}
