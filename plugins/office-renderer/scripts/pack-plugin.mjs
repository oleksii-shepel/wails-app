import archiver from 'archiver';
import { createWriteStream, existsSync, mkdirSync, readFileSync, rmSync } from 'node:fs';
import { basename, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const manifest = JSON.parse(readFileSync(resolve(root, 'plugin.json'), 'utf8'));
const target = `${manifest.target.os}-${manifest.target.arch}`;
const outputDirectory = resolve(root, 'artifacts');
const outputPath = resolve(outputDirectory, `${manifest.id}-${manifest.version}-${target}.zip`);

for (const entry of [manifest.frontend.entry, manifest.backend.entry]) {
  if (!existsSync(resolve(root, entry))) {
    throw new Error(`Missing ${entry}; run npm run build:plugin first.`);
  }
}

mkdirSync(outputDirectory, { recursive: true });
rmSync(outputPath, { force: true });

const output = createWriteStream(outputPath);
const archive = archiver('zip', { zlib: { level: 9 } });
const completed = new Promise((resolvePromise, reject) => {
  output.on('close', resolvePromise);
  output.on('error', reject);
  archive.on('error', reject);
});

archive.pipe(output);
archive.file(resolve(root, 'plugin.json'), { name: 'plugin.json' });
archive.file(resolve(root, manifest.frontend.entry), { name: manifest.frontend.entry });
archive.file(resolve(root, manifest.backend.entry), { name: manifest.backend.entry });
await archive.finalize();
await completed;

console.log(`Packed ${basename(outputPath)} (${archive.pointer()} bytes)`);
