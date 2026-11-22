/**
 * Browser-preview-only adapter for the Aether CLI.
 * The desktop Wails runtime uses Go bindings rather than this Express route.
 * Importing this module does not start a listener or expose an external port.
 */
import fs from 'node:fs';
import fsp from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { execFile } from 'node:child_process';

const MAX_COMMAND_LENGTH = 4096;
const MAX_OUTPUT_BYTES = 4 * 1024 * 1024;
const ALLOWED_COMMANDS = new Set([
  'ls', 'du', 'disk-usage', 'find', 'search', 'dup', 'duplicates',
  'hash', 'checksum', 'verify', 'diff', 'compare', 'split', 'join',
  'rename', 'batch-rename', 'archive', 'trash', 'ws', 'workspace',
  'doctor', 'help', '--help', '-h', '--version', '-v',
]);

/** Parse CLI arguments without invoking a shell. Preserve Windows backslashes. */
export function tokenizeCliCommand(rawLine) {
  const tokens = [];
  let part = '';
  let quote = null;
  let started = false;
  const input = String(rawLine);
  for (let i = 0; i < input.length; i++) {
    const ch = input[i];
    const next = input[i + 1];
    if (ch === '\\' && (next === '"' || next === "'" || next === ' ' || next === '\\')) {
      // Keep backslashes literal in unquoted Windows paths, except when
      // explicitly escaping a space/quote/backslash.
      if (quote === null && next === '\\') {
        part += ch;
      } else if (quote === null || next === quote) {
        part += next;
        i++;
      } else {
        part += ch;
      }
      started = true;
      continue;
    }
    if ((ch === '"' || ch === "'") && (quote === null || quote === ch)) {
      quote = quote === null ? ch : null;
      started = true;
      continue;
    }
    if (!quote && /\s/.test(ch)) {
      if (started) { tokens.push(part); part = ''; started = false; }
      continue;
    }
    part += ch;
    started = true;
  }
  if (quote) throw new Error('Unterminated quoted argument.');
  if (started) tokens.push(part);
  return tokens;
}

function responseBase(cwd, startedAt, overrides = {}) {
  return {
    ok: true,
    cwd,
    stdout: '',
    stderr: '',
    exitCode: 0,
    durationMs: Date.now() - startedAt,
    ...overrides,
  };
}

/**
 * Register the preview-only CLI bridge *behind* the server's localhost and
 * Origin guards. This route accepts only known Aether subcommands and never
 * launches a shell or arbitrary program from the HTTP request.
 */
export function registerCliPreviewApi(app, workspaceRoot) {
  const cliEntry = path.join(workspaceRoot, 'cli', 'aether.mjs');
  app.post('/api/cli/exec', async (req, res) => {
    const startedAt = Date.now();
    try {
      const raw = req.body?.command;
      if (typeof raw !== 'string' || raw.length > MAX_COMMAND_LENGTH) {
        res.status(400).json(responseBase(workspaceRoot, startedAt, {
          ok: false, exitCode: 2, error: 'Invalid CLI command.', stderr: 'Invalid CLI command.\n',
        }));
        return;
      }
      let cwd = typeof req.body?.cwd === 'string' ? req.body.cwd : workspaceRoot;
      cwd = path.resolve(cwd);
      try {
        if (!(await fsp.stat(cwd)).isDirectory()) cwd = workspaceRoot;
      } catch { cwd = workspaceRoot; }
      if (!raw.trim()) { res.json(responseBase(cwd, startedAt)); return; }
      const tokens = tokenizeCliCommand(raw.trim());
      if (tokens[0] === 'aether' || tokens[0] === 'reassembly') tokens.shift();
      if (!tokens.length) tokens.push('--help');

      if (tokens[0] === 'pwd') {
        res.json(responseBase(cwd, startedAt, { stdout: `${cwd}\n` }));
        return;
      }
      if (tokens[0] === 'cd') {
        if (tokens.length > 2) throw new Error('cd accepts only one path.');
        const target = tokens[1] || '~';
        const home = os.homedir() || workspaceRoot;
        const expanded = target === '~' ? home
          : target.startsWith('~/') || target.startsWith('~\\')
            ? path.join(home, target.slice(2)) : target;
        const nextCwd = path.resolve(cwd, expanded);
        const stat = await fsp.stat(nextCwd).catch(() => null);
        if (!stat?.isDirectory()) {
          res.json(responseBase(cwd, startedAt, {
            ok: false, exitCode: 1, stderr: `cd: no such directory: ${target}\n`,
          }));
          return;
        }
        res.json(responseBase(nextCwd, startedAt, { changedCwd: true }));
        return;
      }
      if (tokens[0] === 'clear') {
        res.json(responseBase(cwd, startedAt));
        return;
      }
      if (!ALLOWED_COMMANDS.has(tokens[0])) {
        res.status(400).json(responseBase(cwd, startedAt, {
          ok: false, exitCode: 2, error: 'Unknown Aether command.', stderr: 'Unknown Aether command.\n',
        }));
        return;
      }
      if (!fs.existsSync(cliEntry)) throw new Error('Aether CLI entry point is missing.');
      const color = req.body?.color !== false;
      const execResult = await new Promise(resolve => {
        execFile(process.execPath, [cliEntry, '--cwd', cwd, ...tokens], {
          cwd,
          shell: false,
          windowsHide: true,
          timeout: 30_000,
          maxBuffer: MAX_OUTPUT_BYTES,
          env: { ...process.env, FORCE_COLOR: color ? '1' : '0', NO_COLOR: color ? '' : '1' },
        }, (error, stdout, stderr) => {
          const exitCode = typeof error?.code === 'number' ? error.code : error ? 1 : 0;
          resolve(responseBase(cwd, startedAt, {
            ok: exitCode === 0,
            stdout: stdout || '',
            stderr: stderr || (error && !stdout ? `${error.message}\n` : ''),
            exitCode,
          }));
        });
      });
      res.json(execResult);
    } catch (error) {
      res.status(400).json(responseBase(workspaceRoot, startedAt, {
        ok: false, exitCode: 2,
        error: error instanceof Error ? error.message : String(error),
        stderr: `${error instanceof Error ? error.message : String(error)}\n`,
      }));
    }
  });
}
