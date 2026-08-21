#!/usr/bin/env node

import { existsSync, readFileSync, rmSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const fix = process.argv.includes('--fix');

const required = [
  'manager/frontend/src/app/components/registry-pane.component.ts',
  'plugins/registry/src/registry.plugin.ts',
  'plugins/registry/src/registry-panel-source.ts',
  'plugins/registry/src/registry-key-editor.component.ts',
  'plugins/registry/src/registry-value-editor.component.ts',
  'plugins/registry/src/registry-key-editor.service.ts',
  'plugins/registry/src/registry-value-editor.service.ts',
  'plugins/registry/src/registry-plugin-host.component.ts',
];

const obsoletePluginPresentation = [
  'plugins/registry/src/registry-pane.component.ts',
  'plugins/registry/src/registry-pane.component.d.ts',
  'plugins/registry/src/registry-pane.component.js',
  'plugins/registry/src/registry-pane.component.js.map',
  'plugins/registry/src/registry-pane.component.spec.ts',
  'plugins/registry/src/registry-pane-core-presentation.spec.ts',
  'plugins/registry/src/registry-pane-storage.spec.ts',
];

if (fix) {
  for (const rel of obsoletePluginPresentation) {
    const path = resolve(root, rel);
    if (!existsSync(path)) continue;
    rmSync(path);
    console.log(`removed  ${rel}`);
  }
}

const errors = [];
for (const rel of required) {
  if (!existsSync(resolve(root, rel))) errors.push(`missing required ownership file: ${rel}`);
}
for (const rel of obsoletePluginPresentation) {
  if (existsSync(resolve(root, rel))) errors.push(`obsolete plugin presentation remains: ${rel}`);
}

const publicApiPath = resolve(root, 'plugins/registry/src/public-api.ts');
if (existsSync(publicApiPath)) {
  const text = readFileSync(publicApiPath, 'utf8');
  if (text.includes("./registry-pane.component")) {
    errors.push('Registry plugin public API still exports registry-pane.component.');
  }
}

const pluginPath = resolve(root, 'plugins/registry/src/registry.plugin.ts');
if (existsSync(pluginPath)) {
  const text = readFileSync(pluginPath, 'utf8');
  if (/RegistryPanelPresentationContribution|registerPanelPresentation|unregisterPanelPresentation/.test(text)) {
    errors.push('Registry plugin still claims pane-presentation ownership.');
  }
  for (const label of ['New key', 'New value', 'Edit', 'Delete', 'Refresh']) {
    if (!text.includes(`label: '${label}'`)) errors.push(`missing Registry toolbar action: ${label}`);
  }
  for (const label of ['New Key', 'New Value', 'Edit', 'Delete', 'Refresh']) {
    if (!text.includes(`label: '${label}'`)) errors.push(`missing Registry context-menu action: ${label}`);
  }
}

const hostPath = resolve(root, 'manager/frontend/src/app/components/pane-host.component.ts');
if (existsSync(hostPath)) {
  const text = readFileSync(hostPath, 'utf8');
  if (!text.includes("from './registry-pane.component'")) {
    errors.push('PaneHost must render the Manager-owned Registry Directory clone.');
  }
  if (/RegistryPaneComponent[^\r\n]*@reassembly\/registry-plugin|@reassembly\/registry-plugin[^\r\n]*RegistryPaneComponent/.test(text)) {
    errors.push('PaneHost imports the obsolete plugin-owned Registry pane.');
  }
}

if (errors.length) {
  console.error(`Registry ownership audit failed with ${errors.length} issue(s):`);
  for (const error of errors) console.error(`  - ${error}`);
  process.exit(1);
}

console.log('OK: Registry ownership is split consistently.');
console.log('  Manager: Directory-clone pane presentation + generic menu/toolbar renderer.');
console.log('  Plugin: panel source, Registry commands, toolbar/context contributions, editors.');
