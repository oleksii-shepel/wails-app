import { readFileSync, readdirSync, statSync } from 'node:fs';
import { extname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../src/', import.meta.url));
const themeFile = join(root, 'vscode-theme.scss');
const extensions = new Set(['.ts', '.scss', '.css', '.html']);
// Spec files assert on styles; they are test fixtures, not presentation code.
const excluded = /(^|[\\/])[^\\/]*\.spec\.ts$/;
const rawColor = /(?<![\w-])(#[0-9a-fA-F]{3,8}\b|rgba?\([^)]*\)|hsla?\([^)]*\))/g;
const namedColor = /(?:color|background|border(?:-color)?|fill|stroke|shadow)[^;\n]*:\s*(white|black|red|green|blue|yellow|orange|purple|gray|grey)(?=[; ,}\)])/gi;
const variableRef = /var\((--[\w-]+)/g;
const variableDef = /(--[\w-]+)\s*:/g;
const localVariables = new Set(['--tx', '--ty', '--workspace-ratio', '--cols']);

function filesUnder(directory) {
  const result = [];
  for (const name of readdirSync(directory)) {
    const path = join(directory, name);
    const stat = statSync(path);
    if (stat.isDirectory()) result.push(...filesUnder(path));
    else if (extensions.has(extname(path)) && !excluded.test(path)) result.push(path);
  }
  return result;
}

const files = filesUnder(root);
const definitions = new Set();
const references = new Map();
const violations = [];

for (const path of files) {
  const text = readFileSync(path, 'utf8');
  for (const match of text.matchAll(variableDef)) definitions.add(match[1]);
  for (const match of text.matchAll(variableRef)) {
    const sites = references.get(match[1]) ?? [];
    sites.push(relative(root, path));
    references.set(match[1], sites);
  }

  if (path === themeFile) continue;
  for (const match of text.matchAll(rawColor)) {
    violations.push(`${relative(root, path)}: raw color ${match[1]}`);
  }
  for (const match of text.matchAll(namedColor)) {
    violations.push(`${relative(root, path)}: named presentation color ${match[1]}`);
  }
}

for (const [token, sites] of references) {
  if (definitions.has(token) || localVariables.has(token)) continue;
  violations.push(`${sites[0]}: unresolved theme token ${token}`);
}

if (violations.length) {
  console.error('Theme audit failed:');
  for (const violation of violations) console.error(`  - ${violation}`);
  process.exit(1);
}

console.log(`Theme audit passed: ${files.length - 1} visual source files use theme-owned colors only.`);