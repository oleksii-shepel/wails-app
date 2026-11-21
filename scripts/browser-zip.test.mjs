import { spawnSync } from 'node:child_process';
import assert from 'node:assert/strict';
import { existsSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, dirname, join } from 'node:path';
import { test } from 'node:test';

const server = readFileSync(new URL('../server.mjs', import.meta.url), 'utf8');
const script = server.match(/const ARCHIVE_PY_SCRIPT = String\.raw`([\s\S]*?)`;/)?.[1];
assert.ok(script, 'embedded archive script exists');
const python = process.env.PYTHON || (process.platform === 'win32' ? 'python' : 'python3');

function zip(paths, destination) {
  const result = spawnSync(python, ['-c', script], {
    encoding: 'utf8',
    input: JSON.stringify({ op: 'create_zip', paths, destination }),
  });
  if (result.error) throw result.error;
  return result;
}

function fixture(t) {
  const temp = realpathSync(tmpdir());
  const root = mkdtempSync(join(temp, 'reassembly-zip-test-'));
  t.after(() => {
    const resolved = realpathSync(root);
    assert.equal(dirname(resolved), temp);
    assert.ok(basename(resolved).startsWith('reassembly-zip-test-'));
    rmSync(resolved, { recursive: true, force: true });
  });
  return root;
}

test('browser ZIP creation refuses to overwrite a destination', t => {
  const root = fixture(t);
  const source = join(root, 'source.txt');
  const destination = join(root, 'existing.zip');
  writeFileSync(source, 'source');
  writeFileSync(destination, 'unchanged');
  assert.notEqual(zip([source], destination).status, 0);
  assert.equal(readFileSync(destination, 'utf8'), 'unchanged');
});

test('browser ZIP creation excludes its staging output', t => {
  const root = fixture(t);
  writeFileSync(join(root, 'source.txt'), 'contents');
  const destination = join(root, 'output.zip');
  const result = zip([root], destination);
  assert.equal(result.status, 0, result.stderr);
  const contents = spawnSync(python, ['-m', 'zipfile', '-l', destination], { encoding: 'utf8' });
  assert.equal(contents.status, 0, contents.stderr);
  assert.match(contents.stdout, /source\.txt/);
  assert.doesNotMatch(contents.stdout, /\.archive-create-|output\.zip/);
});

test('browser ZIP creation leaves no partial destination after a missing source', t => {
  const root = fixture(t);
  const source = join(root, 'source.txt');
  const destination = join(root, 'output.zip');
  writeFileSync(source, 'source');
  assert.notEqual(zip([source, join(root, 'missing.txt')], destination).status, 0);
  assert.equal(existsSync(destination), false);
});
