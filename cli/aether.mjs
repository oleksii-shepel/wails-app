#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { runArchiveCommand } from './lib/archive.mjs';
import { runDiffCommand } from './lib/diff.mjs';
import {
  runCompareCommand,
  runDuCommand,
  runDupCommand,
  runFindCommand,
  runHashCommand,
  runJoinCommand,
  runLsCommand,
  runRenameCommand,
  runSplitCommand,
  runVerifyCommand,
} from './lib/fs-ops.mjs';
import { CliUsageError, createStyler, printJson, shouldUseColor } from './lib/output.mjs';
import { runDoctorCommand, runTrashCommand, runWorkspaceCommand } from './lib/trash-ws.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

function readPackageVersion() {
  try {
    const pkg = JSON.parse(fs.readFileSync(path.resolve(__dirname, '..', 'package.json'), 'utf8'));
    return pkg.version || '0.1.0';
  } catch {
    return '0.1.0';
  }
}

const COMMAND_HELP = {
  ls: 'aether ls [path] [-a|--all] [-l|--long] [-s|--sort name|size|mtime] [--no-dirs-first]\nList files and folders with permissions, sizes, and timestamps.',
  du: 'aether du [path] [-d|--depth N] [--top N] [-a|--all]\nAnalyze recursive disk usage and rank largest files/directories.',
  find: 'aether find [path] [-p|--pattern glob] [-g|--grep text] [-c|--case-sensitive] [-d N] [--no-recursive] [-a]\nFind files by glob pattern and/or grep text content (skips binaries & node_modules).',
  dup: 'aether dup [path] [--min-size 1K] [-a|--all]\nDetect duplicate files using size bucketing + SHA-256 verification.',
  hash: 'aether hash <files...> [--algo sha256|sha512|sha1|md5|all] [--manifest]\nCompute cryptographic digests or emit a .sha256 manifest.',
  verify: 'aether verify <manifest.sha256> [--root <dir>]\nVerify checksum manifest against files on disk (exits 1 on mismatch).',
  diff: 'aether diff <leftFile> <rightFile> [-u|--unified|--full] [-w] [-i] [--stat] [--context N]\nLine-by-line LCS diff with unified hunks (exits 1 when files differ).',
  compare: 'aether compare <leftDir> <rightDir> [-r|--recursive] [--hash]\nCompare two directories (identical, modified, only-left, only-right).',
  split: 'aether split <file> [--size 1M] [--out-dir <dir>]\nSplit a file into sequential .part001, .part002, ... chunks.',
  join: 'aether join <part001 | parts...> [-o|--output <dest>]\nJoin split chunks back into a single file (auto-discovers sequence from .part001).',
  rename: 'aether rename <paths...> (--pattern "{name}_{nnn}.{ext}" | --find X --replace Y) [--regex] [--apply]\nBatch rename files with collision checks (dry-run by default; pass --apply to commit).',
  archive: 'aether archive <list|zip|extract> ...\nInspect, create ZIP archives, or safely extract archives with zip-slip protection.',
  trash: 'aether trash <list|put|restore|empty> ...\nManage Reassembly trash bin (shares storage with the GUI).',
  ws: 'aether ws <list|active|paths>\nInspect saved Reassembly workspaces and active left/right pane locations.',
  doctor: 'aether doctor\nCheck Node.js, Python 3, Go/Wails, frontend build status, and writable state dirs.',
};

function printMainHelp(style) {
  const version = readPackageVersion();
  console.log(`${style.bold('aether')} ${style.cyan(`v${version}`)} — Reassembly File Manager CLI\n`);
  console.log(style.bold('USAGE'));
  console.log('  aether [global options] <command> [command options]\n');
  console.log(style.bold('GLOBAL OPTIONS'));
  console.log('  -h, --help        Show help (or `aether help <command>`)');
  console.log('  -v, --version     Show CLI version');
  console.log('  --json            Emit machine-readable JSON output');
  console.log('  --no-color        Disable ANSI colors (also respects NO_COLOR)');
  console.log('  --cwd <dir>       Execute as if started in <dir>\n');
  console.log(style.bold('COMMANDS'));
  const entries = [
    ['ls', 'List directory entries'],
    ['du, disk-usage', 'Analyze disk usage breakdown'],
    ['find, search', 'Find files by glob and/or grep text content'],
    ['dup, duplicates', 'Find duplicate files by SHA-256'],
    ['hash', 'Compute SHA-256/512/SHA-1/MD5 or manifest'],
    ['verify', 'Verify a .sha256 checksum manifest'],
    ['diff', 'Unified LCS line diff between two files'],
    ['compare', 'Compare two directories'],
    ['split', 'Split a file into .part001, .part002 chunks'],
    ['join', 'Join .partNNN chunks back together'],
    ['rename', 'Batch rename with dry-run & collision checks'],
    ['archive', 'List, zip, or safely extract archives'],
    ['trash', 'List, put, restore, or empty Reassembly trash'],
    ['ws, workspace', 'Inspect active workspaces & pane paths'],
    ['doctor', 'Diagnose runtime, archive & build health'],
  ];
  for (const [cmd, desc] of entries) {
    console.log(`  ${style.cyan(cmd.padEnd(18))}${desc}`);
  }
}

const ALIASES = {
  'disk-usage': 'du',
  search: 'find',
  duplicates: 'dup',
  checksum: 'hash',
  'batch-rename': 'rename',
  workspace: 'ws',
};

const HANDLERS = {
  ls: runLsCommand,
  du: runDuCommand,
  find: runFindCommand,
  dup: runDupCommand,
  hash: runHashCommand,
  verify: runVerifyCommand,
  diff: runDiffCommand,
  compare: runCompareCommand,
  split: runSplitCommand,
  join: runJoinCommand,
  rename: runRenameCommand,
  archive: runArchiveCommand,
  trash: runTrashCommand,
  ws: runWorkspaceCommand,
  doctor: runDoctorCommand,
};

function parseGlobalArgs(rawArgs) {
  let json = false;
  let noColor = false;
  let help = false;
  let version = false;
  let cwd = process.cwd();
  const remaining = [];

  for (let i = 0; i < rawArgs.length; i++) {
    const arg = rawArgs[i];
    // Once a subcommand is seen, pass remaining flags through to it verbatim (except --json / --no-color)
    if (remaining.length > 0) {
      if (arg === '--json') json = true;
      else if (arg === '--no-color') noColor = true;
      else if (arg === '-h' || arg === '--help') help = true;
      else remaining.push(arg);
      continue;
    }

    if (arg === '--json') json = true;
    else if (arg === '--no-color') noColor = true;
    else if (arg === '-h' || arg === '--help') help = true;
    else if (arg === '-v' || arg === '--version') version = true;
    else if (arg === '--cwd') {
      const next = rawArgs[++i];
      if (!next) throw new CliUsageError('--cwd requires a directory path.');
      cwd = path.resolve(cwd, next);
    } else if (arg.startsWith('--cwd=')) {
      cwd = path.resolve(cwd, arg.slice('--cwd='.length));
    } else if (arg.startsWith('-')) {
      throw new CliUsageError(`Unknown global option: ${arg}`);
    } else {
      remaining.push(arg);
    }
  }

  return { json, noColor, help, version, cwd, remaining };
}

async function main() {
  const parsed = parseGlobalArgs(process.argv.slice(2));
  const useColor = shouldUseColor(parsed.noColor || parsed.json);
  const style = createStyler(useColor);

  if (parsed.version) {
    const v = readPackageVersion();
    if (parsed.json) printJson({ name: 'aether', version: v });
    else console.log(`aether v${v}`);
    return 0;
  }

  const [rawCmd, ...cmdArgs] = parsed.remaining;
  if (!rawCmd || (parsed.help && !rawCmd) || (rawCmd === 'help' && cmdArgs.length === 0)) {
    printMainHelp(style);
    return 0;
  }

  if (rawCmd === 'help') {
    const target = ALIASES[cmdArgs[0]] || cmdArgs[0];
    if (!COMMAND_HELP[target]) {
      throw new CliUsageError(`Unknown command "${cmdArgs[0]}". Run \`aether --help\` for available commands.`);
    }
    console.log(COMMAND_HELP[target]);
    return 0;
  }

  const cmd = ALIASES[rawCmd] || rawCmd;
  if (parsed.help) {
    if (COMMAND_HELP[cmd]) {
      console.log(COMMAND_HELP[cmd]);
      return 0;
    }
    throw new CliUsageError(`Unknown command "${rawCmd}". Run \`aether --help\`.`);
  }

  const handler = HANDLERS[cmd];
  if (!handler) {
    throw new CliUsageError(`Unknown command "${rawCmd}". Run \`aether --help\` for available commands.`);
  }

  if (!fs.existsSync(parsed.cwd)) {
    throw new CliUsageError(`Working directory does not exist: ${parsed.cwd}`);
  }

  return await handler(cmdArgs, {
    cwd: parsed.cwd,
    json: parsed.json,
    style,
  });
}

main()
  .then((code) => {
    process.exitCode = typeof code === 'number' ? code : 0;
  })
  .catch((err) => {
    const jsonFlag = process.argv.includes('--json');
    if (jsonFlag) {
      printJson({ error: err?.message || String(err), exitCode: err?.exitCode || 1 });
    } else {
      console.error(`Error: ${err?.message || String(err)}`);
    }
    process.exitCode = typeof err?.exitCode === 'number' ? err.exitCode : 1;
  });
