import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');

const required = [
  'node_modules/@angular/animations/package.json',
  'node_modules/@angular/animations/fesm2022/animations.mjs',
  'plugins/registry/dist/package.json',
];

let failed = false;
for (const rel of required) {
  const path = resolve(root, rel);
  if (!existsSync(path)) {
    console.error(`missing: ${rel}`);
    failed = true;
  } else {
    console.log(`ok: ${rel}`);
  }
}

const animationsPackage = resolve(root, 'node_modules/@angular/animations/package.json');
if (existsSync(animationsPackage)) {
  const pkg = JSON.parse(readFileSync(animationsPackage, 'utf8'));
  const browser = pkg.exports?.['./browser'];
  if (!browser) {
    console.error('missing export: @angular/animations/browser');
    failed = true;
  } else {
    console.log('ok: @angular/animations/browser export');
  }
}

process.exit(failed ? 1 : 0);
