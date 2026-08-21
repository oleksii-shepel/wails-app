#!/usr/bin/env node

import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const corePackagePath = resolve(root, 'packages/core/package.json');
const managerTsconfigPath = resolve(root, 'manager/frontend/tsconfig.json');
const registryTsconfigPath = resolve(root, 'plugins/registry/tsconfig.json');
const registryLibTsconfigPath = resolve(root, 'plugins/registry/tsconfig.lib.json');

const corePackage = readJson(corePackagePath);
const managerTsconfig = readJson(managerTsconfigPath);
const registryTsconfig = readJson(registryTsconfigPath);
const registryLibTsconfig = readJson(registryLibTsconfigPath);

const sourceTargets = new Map([
  ['@reassembly/core', 'packages/core/frontend/src/index.ts'],
  ['@reassembly/core/contracts/files', 'packages/core/frontend/src/contracts/files/index.ts'],
  ['@reassembly/core/contracts/input', 'packages/core/frontend/src/contracts/input/index.ts'],
  ['@reassembly/core/contracts/sources', 'packages/core/frontend/src/contracts/sources/index.ts'],
  ['@reassembly/core/contracts/panes', 'packages/core/frontend/src/contracts/panes/index.ts'],
  ['@reassembly/core/runtime/panes', 'packages/core/frontend/src/runtime/panes/index.ts'],
  ['@reassembly/core/files', 'packages/core/frontend/src/contracts/files/index.ts'],
  ['@reassembly/core/input', 'packages/core/frontend/src/contracts/input/index.ts'],
  ['@reassembly/core/sources', 'packages/core/frontend/src/contracts/sources/index.ts'],
  ['@reassembly/core/panes', 'packages/core/frontend/src/contracts/panes/index.ts'],
  ['@reassembly/core/renderer', 'packages/core/frontend/src/runtime/panes/index.ts'],
  ['@reassembly/core/ui/panes', 'packages/core/frontend/src/ui/panes/index.ts'],
]);

const expectedExports = new Set(
  [...sourceTargets.keys()].map(specifier =>
    specifier === '@reassembly/core'
      ? '.'
      : `./${specifier.slice('@reassembly/core/'.length)}`,
  ),
);

let failed = false;

for (const exportName of expectedExports) {
  if (!corePackage.exports?.[exportName]) {
    fail(`core package is missing export ${exportName}`);
  }
}

for (const [specifier, sourceRelativePath] of sourceTargets) {
  const sourcePath = resolve(root, sourceRelativePath);
  if (!existsSync(sourcePath)) {
    fail(`core source entry point does not exist: ${sourceRelativePath}`);
  }

  verifyPathAlias('manager/frontend/tsconfig.json', managerTsconfig, specifier, sourcePath);
  verifyPathAlias('plugins/registry/tsconfig.json', registryTsconfig, specifier, sourcePath);
  verifyPathAlias('plugins/registry/tsconfig.lib.json', registryLibTsconfig, specifier, sourcePath);
}

for (const config of [
  ['manager/frontend/tsconfig.json', managerTsconfig],
  ['plugins/registry/tsconfig.json', registryTsconfig],
  ['plugins/registry/tsconfig.lib.json', registryLibTsconfig],
]) {
  const [label, json] = config;
  if (json.compilerOptions?.paths?.['@reassembly/core/*']) {
    fail(`${label} still contains the broad @reassembly/core/* escape hatch`);
  }
}

const paneExport = corePackage.exports?.['./ui/panes'];
if (paneExport) {
  assertFileTarget('types', paneExport.types);
  assertFileTarget('default', paneExport.default);
}

if (!failed) {
  console.log('ok: @reassembly/core package exports and workspace source aliases are aligned');
}

process.exit(failed ? 1 : 0);

function verifyPathAlias(label, config, specifier, expectedAbsolutePath) {
  const values = config.compilerOptions?.paths?.[specifier];
  if (!Array.isArray(values) || values.length !== 1) {
    fail(`${label} must map ${specifier} to exactly one public source entry point`);
    return;
  }

  const configDir = resolve(root, label, '..');
  const actualAbsolutePath = resolve(configDir, values[0]);
  if (actualAbsolutePath !== expectedAbsolutePath) {
    fail(
      `${label} maps ${specifier} to ${values[0]}, expected ${expectedAbsolutePath}`,
    );
  }
}

function assertFileTarget(kind, packageRelativePath) {
  if (typeof packageRelativePath !== 'string') {
    fail(`./ui/panes is missing its ${kind} target`);
    return;
  }

  const absolute = resolve(root, 'packages/core', packageRelativePath);
  if (!existsSync(absolute)) {
    // Dist is allowed to be absent before the Core build. Source alignment is the
    // editor contract; this diagnostic simply makes the build prerequisite clear.
    console.warn(`pending build: ${packageRelativePath}`);
  }
}

function readJson(path) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

function fail(message) {
  console.error(`core-package: ${message}`);
  failed = true;
}
