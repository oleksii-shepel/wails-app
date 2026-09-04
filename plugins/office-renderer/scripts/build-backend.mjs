import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';

const backend = resolve(import.meta.dirname, '..', 'backend');
const env = { ...process.env, GOOS: 'windows', GOARCH: 'amd64', CGO_ENABLED: '0', GOWORK: 'off' };
const tidy = spawnSync('go', ['mod', 'tidy'], {
  cwd: backend,
  env,
  stdio: 'inherit',
  shell: false,
});
if (tidy.error) throw tidy.error;
if (tidy.status !== 0) process.exit(tidy.status ?? 1);

const tests = spawnSync('go', ['test', '.'], {
  cwd: backend,
  env,
  stdio: 'inherit',
  shell: false,
});
if (tests.error) throw tests.error;
if (tests.status !== 0) process.exit(tests.status ?? 1);

const result = spawnSync('go', [
  'build', '-trimpath', '-ldflags=-s -w -H=windowsgui', '-o', 'plugin.exe', '.',
], {
  cwd: backend,
  env,
  stdio: 'inherit',
  shell: false,
});

if (result.error) throw result.error;
if (result.status !== 0) process.exit(result.status ?? 1);
console.log('Built backend/plugin.exe for windows-x64');
