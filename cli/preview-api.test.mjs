import { strict as assert } from 'node:assert';
import { test } from 'node:test';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { registerCliPreviewApi, tokenizeCliCommand } from './preview-api.mjs';

const workspaceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function createRequestHandler() {
  let handler;
  registerCliPreviewApi({ post(url, fn) { assert.equal(url, '/api/cli/exec'); handler = fn; } }, workspaceRoot);
  return async (body) => {
    let status = 200;
    let response;
    await handler({ body }, {
      status(code) { status = code; return this; },
      json(value) { response = value; return this; },
    });
    return { status, ...response };
  };
}

test('argument parser preserves ordinary Windows paths and quoted spaces', () => {
  assert.deepEqual(tokenizeCliCommand('ls C:\\Users\\Public'), ['ls', 'C:\\Users\\Public']);
  assert.deepEqual(tokenizeCliCommand('ls "C:\\Program Files\\App"'), ['ls', 'C:\\Program Files\\App']);
  assert.deepEqual(tokenizeCliCommand('find . -p "*.ts"'), ['find', '.', '-p', '*.ts']);
  assert.throws(() => tokenizeCliCommand('ls "broken'), /Unterminated/);
});

test('preview adapter serves local CLI without shell execution', async () => {
  const run = createRequestHandler();
  const version = await run({ command: 'aether --version', cwd: workspaceRoot, color: false });
  assert.equal(version.status, 200);
  assert.equal(version.ok, true);
  assert.match(version.stdout, /aether v/);
  const unknown = await run({ command: 'sh -c whoami', cwd: workspaceRoot });
  assert.equal(unknown.status, 400);
  assert.equal(unknown.exitCode, 2);
  const excessive = await run({ command: 'a'.repeat(4097) });
  assert.equal(excessive.status, 400);
  const pwd = await run({ command: 'pwd', cwd: os.tmpdir() });
  assert.equal(pwd.ok, true);
  assert.match(pwd.stdout, /tmp|Temp/i);
});
