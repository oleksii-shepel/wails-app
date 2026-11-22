import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import fs from 'node:fs';
import fsp from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { after, before, describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const CLI_ENTRY = path.join(__dirname, 'aether.mjs');

function runCli(args, { cwd, env } = {}) {
  return new Promise((resolve) => {
    execFile(
      process.execPath,
      [CLI_ENTRY, ...args],
      {
        cwd: cwd || os.tmpdir(),
        env: { ...process.env, NO_COLOR: '1', ...(env || {}) },
        maxBuffer: 4 * 1024 * 1024,
      },
      (err, stdout, stderr) => {
        const code = typeof err?.code === 'number' ? err.code : err ? 1 : 0;
        resolve({ code, stdout: stdout || '', stderr: stderr || '' });
      },
    );
  });
}

describe('aether CLI suite', () => {
  let tmpRoot;
  let stateDir;

  before(async () => {
    tmpRoot = await fsp.mkdtemp(path.join(os.tmpdir(), 'aether-cli-test-'));
    stateDir = path.join(tmpRoot, '_state');
    await fsp.mkdir(stateDir, { recursive: true });
  });

  after(async () => {
    if (tmpRoot) {
      await fsp.rm(tmpRoot, { recursive: true, force: true });
    }
  });

  it('shows help, version, and rejects unknown commands with exit code 2', async () => {
    const helpRes = await runCli(['--help']);
    assert.equal(helpRes.code, 0);
    assert.match(helpRes.stdout, /Reassembly File Manager CLI/);
    assert.match(helpRes.stdout, /compare/);

    const verRes = await runCli(['--version', '--json']);
    assert.equal(verRes.code, 0);
    assert.equal(JSON.parse(verRes.stdout).name, 'aether');

    const badRes = await runCli(['nonexistent-cmd']);
    assert.equal(badRes.code, 2);
    assert.match(badRes.stderr, /Unknown command/);
  });

  it('lists directories and computes recursive disk usage', async () => {
    const work = path.join(tmpRoot, 'ls-du');
    await fsp.mkdir(path.join(work, 'sub'), { recursive: true });
    await fsp.writeFile(path.join(work, 'alpha.txt'), 'hello world', 'utf8');
    await fsp.writeFile(path.join(work, 'sub', 'big.bin'), Buffer.alloc(4096, 0x41));
    await fsp.writeFile(path.join(work, '.hidden'), 'secret', 'utf8');

    const lsPlain = await runCli(['ls', '--json'], { cwd: work });
    assert.equal(lsPlain.code, 0);
    const lsData = JSON.parse(lsPlain.stdout);
    assert.equal(lsData.entries.some((e) => e.name === '.hidden'), false);
    assert.equal(lsData.entries[0].type, 'dir');

    const lsAll = await runCli(['ls', '-a', '-l', '--json'], { cwd: work });
    assert.equal(JSON.parse(lsAll.stdout).entries.some((e) => e.name === '.hidden'), true);

    const duRes = await runCli(['du', '--top', '5', '--json'], { cwd: work });
    assert.equal(duRes.code, 0);
    const duData = JSON.parse(duRes.stdout);
    assert.equal(duData.totalBytes, 4096 + 11);
    assert.equal(duData.fileCount, 2);
  });

  it('searches filenames and greps text while skipping binaries', async () => {
    const work = path.join(tmpRoot, 'find');
    await fsp.mkdir(path.join(work, 'nested'), { recursive: true });
    await fsp.writeFile(path.join(work, 'notes.md'), 'line 1\nfind-me-needle here\n', 'utf8');
    await fsp.writeFile(path.join(work, 'nested', 'code.ts'), 'const x = "find-me-needle";\n', 'utf8');
    await fsp.writeFile(path.join(work, 'blob.bin'), Buffer.from([0, 1, 2, 3, ...Buffer.from('find-me-needle')]));

    const res = await runCli(['find', '.', '-g', 'find-me-needle', '--json'], { cwd: work });
    assert.equal(res.code, 0);
    const data = JSON.parse(res.stdout);
    assert.equal(data.count, 2);
    assert.equal(data.matches.some((m) => m.relPath === 'blob.bin'), false);

    const globRes = await runCli(['find', '.', '-p', '*.ts', '--json'], { cwd: work });
    assert.equal(JSON.parse(globRes.stdout).count, 1);
  });

  it('detects verified SHA-256 duplicates', async () => {
    const work = path.join(tmpRoot, 'dup');
    await fsp.mkdir(work, { recursive: true });
    const payload = 'identical content payload '.repeat(20);
    await fsp.writeFile(path.join(work, 'a.txt'), payload, 'utf8');
    await fsp.writeFile(path.join(work, 'b.txt'), payload, 'utf8');
    await fsp.writeFile(path.join(work, 'c.txt'), 'unique payload '.repeat(20), 'utf8');

    const res = await runCli(['dup', '.', '--json'], { cwd: work });
    assert.equal(res.code, 0);
    const data = JSON.parse(res.stdout);
    assert.equal(data.groupCount, 1);
    assert.equal(data.reclaimableBytes, Buffer.byteLength(payload));
  });

  it('generates and verifies .sha256 manifests and fails on tampering', async () => {
    const work = path.join(tmpRoot, 'hash-verify');
    await fsp.mkdir(work, { recursive: true });
    await fsp.writeFile(path.join(work, 'one.txt'), 'first file', 'utf8');
    await fsp.writeFile(path.join(work, 'two.txt'), 'second file', 'utf8');

    const gen = await runCli(['hash', 'one.txt', 'two.txt', '--manifest'], { cwd: work });
    assert.equal(gen.code, 0);
    const manifestPath = path.join(work, 'CHECKSUMS.sha256');
    await fsp.writeFile(manifestPath, gen.stdout, 'utf8');

    const okRes = await runCli(['verify', 'CHECKSUMS.sha256', '--json'], { cwd: work });
    assert.equal(okRes.code, 0);
    assert.equal(JSON.parse(okRes.stdout).ok, true);

    await fsp.writeFile(path.join(work, 'two.txt'), 'tampered!', 'utf8');
    const failRes = await runCli(['verify', 'CHECKSUMS.sha256', '--json'], { cwd: work });
    assert.equal(failRes.code, 1);
    assert.equal(JSON.parse(failRes.stdout).failed, 1);
  });

  it('computes unified LCS diffs and directory comparisons', async () => {
    const work = path.join(tmpRoot, 'diff-cmp');
    const dirA = path.join(work, 'a');
    const dirB = path.join(work, 'b');
    await fsp.mkdir(dirA, { recursive: true });
    await fsp.mkdir(dirB, { recursive: true });

    const f1 = path.join(dirA, 'doc.txt');
    const f2 = path.join(dirB, 'doc.txt');
    await fsp.writeFile(f1, 'one\ntwo\nthree\n', 'utf8');
    await fsp.writeFile(f2, 'one\ntwo updated\nthree\nfour\n', 'utf8');

    const diffRes = await runCli(['diff', f1, f2, '--unified']);
    assert.equal(diffRes.code, 1);
    assert.match(diffRes.stdout, /-two/);
    assert.match(diffRes.stdout, /\+two updated/);

    const wsRes = await runCli(['diff', f1, f1, '-w', '--json']);
    assert.equal(wsRes.code, 0);
    assert.equal(JSON.parse(wsRes.stdout).identical, true);

    await fsp.writeFile(path.join(dirA, 'only-a.txt'), 'a', 'utf8');
    await fsp.writeFile(path.join(dirB, 'only-b.txt'), 'b', 'utf8');
    const cmpRes = await runCli(['compare', dirA, dirB, '-r', '--hash', '--json']);
    assert.equal(cmpRes.code, 1);
    const cmp = JSON.parse(cmpRes.stdout);
    assert.deepEqual(cmp.onlyLeft, ['only-a.txt']);
    assert.deepEqual(cmp.onlyRight, ['only-b.txt']);
    assert.equal(cmp.modified.length, 1);
  });

  it('splits and rejoins binary files losslessly', async () => {
    const work = path.join(tmpRoot, 'split-join');
    await fsp.mkdir(work, { recursive: true });
    const src = path.join(work, 'sample.bin');
    const raw = Buffer.alloc(5000);
    for (let i = 0; i < raw.length; i++) raw[i] = i % 251;
    await fsp.writeFile(src, raw);

    const splitRes = await runCli(['split', 'sample.bin', '--size', '2048', '--json'], { cwd: work });
    assert.equal(splitRes.code, 0);
    const splitData = JSON.parse(splitRes.stdout);
    assert.equal(splitData.parts.length, 3);

    const outFile = path.join(work, 'rejoined.bin');
    const joinRes = await runCli(['join', 'sample.bin.part001', '-o', outFile, '--json'], { cwd: work });
    assert.equal(joinRes.code, 0);
    const joinedBuf = await fsp.readFile(outFile);
    assert.equal(joinedBuf.equals(raw), true);
  });

  it('performs dry-run and applied batch renames with collision safety', async () => {
    const work = path.join(tmpRoot, 'rename');
    await fsp.mkdir(work, { recursive: true });
    await fsp.writeFile(path.join(work, 'img_a.png'), '1');
    await fsp.writeFile(path.join(work, 'img_b.png'), '2');

    const dry = await runCli(['rename', 'img_a.png', 'img_b.png', '--pattern', 'photo_{nnn}', '--json'], { cwd: work });
    assert.equal(dry.code, 0);
    assert.equal(fs.existsSync(path.join(work, 'img_a.png')), true);

    const applied = await runCli(
      ['rename', 'img_a.png', 'img_b.png', '--pattern', 'photo_{nnn}', '--apply', '--json'],
      { cwd: work },
    );
    assert.equal(applied.code, 0);
    assert.equal(fs.existsSync(path.join(work, 'photo_001.png')), true);
    assert.equal(fs.existsSync(path.join(work, 'photo_002.png')), true);
  });

  it('creates, lists, and safely extracts archives via zip roundtrip', async () => {
    const work = path.join(tmpRoot, 'archive');
    const srcDir = path.join(work, 'bundle');
    const outDir = path.join(work, 'extracted');
    await fsp.mkdir(srcDir, { recursive: true });
    await fsp.writeFile(path.join(srcDir, 'hello.txt'), 'archive content', 'utf8');

    const zipPath = path.join(work, 'bundle.zip');
    const zipRes = await runCli(['archive', 'zip', zipPath, srcDir, '--json'], { cwd: work });
    assert.equal(zipRes.code, 0);

    const listRes = await runCli(['archive', 'list', zipPath, '--json'], { cwd: work });
    assert.equal(listRes.code, 0);
    const listed = JSON.parse(listRes.stdout);
    assert.equal(listed.entries.some((e) => e.path === 'bundle/hello.txt'), true);

    const extRes = await runCli(['archive', 'extract', zipPath, outDir, '--json'], { cwd: work });
    assert.equal(extRes.code, 0);
    const extracted = await fsp.readFile(path.join(outDir, 'bundle', 'hello.txt'), 'utf8');
    assert.equal(extracted, 'archive content');
  });

  it('moves items to Reassembly trash, lists them, and restores cleanly in isolated state dir', async () => {
    const work = path.join(tmpRoot, 'trash');
    await fsp.mkdir(work, { recursive: true });
    const target = path.join(work, 'temp.txt');
    await fsp.writeFile(target, 'trashable payload', 'utf8');
    const env = { AETHER_STATE_DIR: stateDir };

    const putRes = await runCli(['trash', 'put', target, '--json'], { cwd: work, env });
    assert.equal(putRes.code, 0);
    assert.equal(fs.existsSync(target), false);

    const listRes = await runCli(['trash', 'list', '--json'], { cwd: work, env });
    const listData = JSON.parse(listRes.stdout);
    assert.equal(listData.count, 1);

    const restoreRes = await runCli(['trash', 'restore', 'temp.txt', '--json'], { cwd: work, env });
    assert.equal(restoreRes.code, 0);
    assert.equal(await fsp.readFile(target, 'utf8'), 'trashable payload');
  });

  it('reports doctor diagnostics cleanly in JSON', async () => {
    const res = await runCli(['doctor', '--json'], { env: { AETHER_STATE_DIR: stateDir } });
    assert.equal(res.code, 0);
    const data = JSON.parse(res.stdout);
    assert.equal(data.ok, true);
    assert.equal(data.checks.some((c) => c.name === 'Node.js' && c.status === 'ok'), true);
  });
});
