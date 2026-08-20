import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';

const backend = resolve(import.meta.dirname, '..', 'backend');

const args = [
  'build',
  '-trimpath',
  '-ldflags=-s -w',
  '-o',
  'plugin.exe',
  '.',
];

const result = spawnSync('go', args, {
  cwd: backend,
  env: {
    ...process.env,
    GOOS: 'windows',
    GOARCH: 'amd64',
    CGO_ENABLED: '0',

    // Important: plugin backend is its own Go module.
    // Do not inherit the manager repository's go.work.
    GOWORK: 'off',
  },
  stdio: 'inherit',
  shell: false,
});

if (result.error) {
  if (result.error.code === 'ENOENT') {
    throw new Error(
      'Go was not found on PATH. Install Go or add go.exe to PATH before running npm run build:backend.',
    );
  }

  throw result.error;
}

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}

console.log('Built backend/plugin.exe for windows-x64');