#!/usr/bin/env node

import { existsSync, readFileSync, readdirSync, rmSync, statSync } from 'node:fs';
import { relative, resolve, sep } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const managerFrontend = resolve(root, 'manager/frontend/src');
const pluginRoot = resolve(root, 'plugins/registry');
const fix = process.argv.includes('--fix');

const migratedManagerFiles = [
  // Compatibility component/service re-exports.
  'manager/frontend/src/app/components/registry-pane.component.ts',
  'manager/frontend/src/app/components/registry-key-editor.component.ts',
  'manager/frontend/src/app/components/registry-value-editor.component.ts',
  'manager/frontend/src/app/services/registry-key-editor.service.ts',
  'manager/frontend/src/app/services/registry-value-editor.service.ts',

  // Registry presentation tests belong to the plugin package now.
  'manager/frontend/src/app/components/registry-key-editor-interaction.spec.ts',
  'manager/frontend/src/app/components/registry-pane.spec.ts',
  'manager/frontend/src/app/components/registry-pane-contract.spec.ts',
  'manager/frontend/src/app/components/registry-value-editor-type.spec.ts',

  // Transitional extraction guard; plugin/package-level tests replace it.
  'manager/frontend/src/app/components/registry-plugin-extraction.spec.ts',
];

const requiredPluginFiles = [
  'plugins/registry/package.json',
  'plugins/registry/src/public-api.ts',
  'plugins/registry/src/registry.plugin.ts',
  'plugins/registry/src/registry-panel-source.ts',
  'plugins/registry/src/registry-pane.component.ts',
  'plugins/registry/src/registry-key-editor.component.ts',
  'plugins/registry/src/registry-value-editor.component.ts',
  'plugins/registry/src/registry-key-editor.service.ts',
  'plugins/registry/src/registry-value-editor.service.ts',
];

const allowedContentByFile = new Map([
  // PaneHost may mount the public plugin-owned sibling renderer. It must not
  // contain Registry data semantics, commands, editors, or backend behavior.
  ['manager/frontend/src/app/components/pane-host.component.ts', [
    /@reassembly\/registry-plugin/,
    /\bRegistryPaneComponent\b/,
  ]],
  // Manager may register the plugin and adapt generic host capabilities.
  ['manager/frontend/src/app/services/plugin-manager.service.ts', [
    /@reassembly\/registry-plugin/,
    /\bcreateRegistryPlugin\b/,
    /\bregistry\b/i,
    /\bRegistry\b/,
  ]],

  // Wails/host capability; backend Registry implementation remains in Manager.
  ['manager/frontend/src/app/services/api.service.ts', [
    /\bwriteRegistryValue\b/,
    /\bWriteRegistryValue\b/,
    /registry/i,
  ]],

  // Root composition may mount a plugin-owned host component.
  ['manager/frontend/src/app/app.component.ts', [
    /@reassembly\/registry-plugin/,
    /\bRegistryPluginHostComponent\b/,
    /reassembly-registry-plugin-host/,
  ]],
]);

const forbiddenPatterns = [
  { label: 'Registry scheme', regex: /registry:\/\//i },
  { label: 'Registry command id', regex: /builtin\.registry/i },
  { label: 'Registry value type', regex: /\bREG_(?:NONE|SZ|EXPAND_SZ|BINARY|DWORD|DWORD_BIG_ENDIAN|MULTI_SZ|QWORD)\b/ },
  { label: 'Registry presentation symbol', regex: /\bRegistry(?:Pane|KeyEditor|ValueEditor)(?:Component|Service)?\b/ },
  { label: 'Registry metadata UI', regex: /\bregistry(?:Kind|ValueType|Preview)\b/ },
  { label: 'Registry-specific surface branch', regex: /\bisRegistrySurface\b/ },
];

const textExtensions = new Set([
  '.ts', '.tsx', '.js', '.mjs', '.cjs', '.html', '.scss', '.css', '.json', '.md',
]);

function normalize(path) {
  return relative(root, path).split(sep).join('/');
}

function extension(path) {
  const slash = path.lastIndexOf('/');
  const dot = path.lastIndexOf('.');
  return dot > slash ? path.slice(dot) : '';
}

function walk(dir, out = []) {
  if (!existsSync(dir)) return out;
  for (const name of readdirSync(dir)) {
    const path = resolve(dir, name);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      if (name === 'node_modules' || name === 'dist' || name === '.angular') continue;
      walk(path, out);
    } else {
      out.push(path);
    }
  }
  return out;
}

function verifyPluginExtraction() {
  const missing = requiredPluginFiles.filter(path => !existsSync(resolve(root, path)));
  if (missing.length) {
    console.error('Registry plugin extraction is incomplete; cleanup is blocked.');
    for (const path of missing) console.error(`  missing: ${path}`);
    process.exitCode = 2;
    return false;
  }
  return true;
}

function cleanupMigratedFiles() {
  if (!verifyPluginExtraction()) return false;

  let removed = 0;
  for (const rel of migratedManagerFiles) {
    const path = resolve(root, rel);
    if (!existsSync(path)) continue;
    rmSync(path);
    console.log(`removed  ${rel}`);
    removed++;
  }
  console.log(`\nRemoved ${removed} migrated Manager file(s).`);
  return true;
}

function lineOf(text, index) {
  let line = 1;
  for (let i = 0; i < index; i++) if (text.charCodeAt(i) === 10) line++;
  return line;
}

function isAllowed(rel, matchText) {
  const rules = allowedContentByFile.get(rel);
  return !!rules?.some(regex => regex.test(matchText));
}

function auditManagerFrontend() {
  const findings = [];
  const registryNamedFiles = [];

  for (const path of walk(managerFrontend)) {
    const rel = normalize(path);

    if (/\/registry[^/]*\.(?:ts|tsx|js|mjs|html|scss|css)$/i.test(rel)) {
      registryNamedFiles.push(rel);
    }

    if (!textExtensions.has(extension(rel))) continue;
    const text = readFileSync(path, 'utf8');

    for (const pattern of forbiddenPatterns) {
      const flags = pattern.regex.flags.includes('g')
        ? pattern.regex.flags
        : pattern.regex.flags + 'g';
      const regex = new RegExp(pattern.regex.source, flags);

      for (const match of text.matchAll(regex)) {
        const token = match[0];
        if (isAllowed(rel, token)) continue;
        findings.push({
          file: rel,
          line: lineOf(text, match.index ?? 0),
          label: pattern.label,
          token,
        });
      }
    }
  }

  // A registry-named source file in Manager is always suspicious after extraction,
  // even if its contents do not currently match a forbidden token.
  for (const file of registryNamedFiles) {
    if (!findings.some(item => item.file === file)) {
      findings.push({
        file,
        line: 1,
        label: 'Registry-owned filename remains in Manager',
        token: file.split('/').pop(),
      });
    }
  }

  findings.sort((a, b) =>
    a.file.localeCompare(b.file) || a.line - b.line || a.label.localeCompare(b.label));

  return findings;
}

if (fix) {
  console.log('Cleaning migrated Registry frontend residue from Manager...\n');
  if (!cleanupMigratedFiles()) process.exit();
  console.log('');
}

console.log('Auditing Registry plugin boundary...\n');

if (!existsSync(pluginRoot)) {
  console.error('plugins/registry does not exist.');
  process.exit(2);
}

const findings = auditManagerFrontend();

if (!findings.length) {
  console.log('OK: Manager frontend contains no Registry-specific implementation residue.');
  console.log('Allowed host references are limited to plugin bootstrap/composition and Registry backend capability calls.');
  process.exit(0);
}

console.error(`Found ${findings.length} Registry boundary violation(s):\n`);
for (const finding of findings) {
  console.error(
    `${finding.file}:${finding.line}  ${finding.label}: ${JSON.stringify(finding.token)}`,
  );
}

console.error('\nMove presentation/commands/editors/tests into plugins/registry.');
console.error('Keep only generic host adapters/capabilities in Manager.');
process.exit(1);
