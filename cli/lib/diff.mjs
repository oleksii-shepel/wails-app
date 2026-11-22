import fs from 'node:fs/promises';
import path from 'node:path';
import { CliRuntimeError, CliUsageError, printJson } from './output.mjs';

const MAX_DIFF_LINES = 8000;

function normalizeLine(line, { ignoreWhitespace = false, ignoreCase = false } = {}) {
  let out = line;
  if (ignoreWhitespace) {
    out = out.replace(/\s+/g, ' ').trim();
  }
  if (ignoreCase) {
    out = out.toLowerCase();
  }
  return out;
}

export function computeLineDiff(leftText, rightText, options = {}) {
  const rawLeft = leftText.length === 0 ? [] : leftText.split(/\r?\n/);
  const rawRight = rightText.length === 0 ? [] : rightText.split(/\r?\n/);

  if (rawLeft.length * rawRight.length > MAX_DIFF_LINES * MAX_DIFF_LINES) {
    throw new CliRuntimeError(
      `Files too large for full LCS diff (${rawLeft.length} x ${rawRight.length} lines).`,
    );
  }

  const normLeft = rawLeft.map((l) => normalizeLine(l, options));
  const normRight = rawRight.map((l) => normalizeLine(l, options));

  const n = normLeft.length;
  const m = normRight.length;

  // Trim common prefix & suffix to keep DP small on large mostly-matching files
  let prefix = 0;
  while (prefix < n && prefix < m && normLeft[prefix] === normRight[prefix]) {
    prefix++;
  }
  let suffix = 0;
  while (
    suffix < n - prefix &&
    suffix < m - prefix &&
    normLeft[n - 1 - suffix] === normRight[m - 1 - suffix]
  ) {
    suffix++;
  }

  const midLeft = normLeft.slice(prefix, n - suffix);
  const midRight = normRight.slice(prefix, m - suffix);
  const ln = midLeft.length;
  const rm = midRight.length;

  const ops = [];
  for (let i = 0; i < prefix; i++) {
    ops.push({ type: 'equal', leftLine: i + 1, rightLine: i + 1, text: rawLeft[i] });
  }

  if (ln > 0 || rm > 0) {
    const dp = Array.from({ length: ln + 1 }, () => new Uint32Array(rm + 1));
    for (let i = ln - 1; i >= 0; i--) {
      for (let j = rm - 1; j >= 0; j--) {
        if (midLeft[i] === midRight[j]) {
          dp[i][j] = dp[i + 1][j + 1] + 1;
        } else {
          dp[i][j] = dp[i + 1][j] >= dp[i][j + 1] ? dp[i + 1][j] : dp[i][j + 1];
        }
      }
    }

    let i = 0;
    let j = 0;
    while (i < ln && j < rm) {
      if (midLeft[i] === midRight[j]) {
        ops.push({
          type: 'equal',
          leftLine: prefix + i + 1,
          rightLine: prefix + j + 1,
          text: rawLeft[prefix + i],
        });
        i++;
        j++;
      } else if (dp[i + 1][j] >= dp[i][j + 1]) {
        ops.push({
          type: 'delete',
          leftLine: prefix + i + 1,
          rightLine: null,
          text: rawLeft[prefix + i],
        });
        i++;
      } else {
        ops.push({
          type: 'insert',
          leftLine: null,
          rightLine: prefix + j + 1,
          text: rawRight[prefix + j],
        });
        j++;
      }
    }
    while (i < ln) {
      ops.push({
        type: 'delete',
        leftLine: prefix + i + 1,
        rightLine: null,
        text: rawLeft[prefix + i],
      });
      i++;
    }
    while (j < rm) {
      ops.push({
        type: 'insert',
        leftLine: null,
        rightLine: prefix + j + 1,
        text: rawRight[prefix + j],
      });
      j++;
    }
  }

  for (let s = 0; s < suffix; s++) {
    const li = n - suffix + s;
    const ri = m - suffix + s;
    ops.push({ type: 'equal', leftLine: li + 1, rightLine: ri + 1, text: rawLeft[li] });
  }

  let added = 0;
  let removed = 0;
  let unchanged = 0;
  for (const op of ops) {
    if (op.type === 'insert') added++;
    else if (op.type === 'delete') removed++;
    else unchanged++;
  }

  return {
    identical: added === 0 && removed === 0,
    stats: {
      added,
      removed,
      unchanged,
      leftLines: n,
      rightLines: m,
    },
    ops,
  };
}

export function buildUnifiedHunks(ops, contextLines = 3) {
  const changeIndices = [];
  for (let i = 0; i < ops.length; i++) {
    if (ops[i].type !== 'equal') changeIndices.push(i);
  }
  if (changeIndices.length === 0) return [];

  const ranges = [];
  for (const idx of changeIndices) {
    const start = Math.max(0, idx - contextLines);
    const end = Math.min(ops.length - 1, idx + contextLines);
    const last = ranges[ranges.length - 1];
    if (last && start <= last.end + 1) {
      last.end = Math.max(last.end, end);
    } else {
      ranges.push({ start, end });
    }
  }

  return ranges.map(({ start, end }) => {
    const slice = ops.slice(start, end + 1);
    let leftStart = null;
    let rightStart = null;
    let leftCount = 0;
    let rightCount = 0;
    const lines = [];

    for (const op of slice) {
      if (op.type === 'equal') {
        if (leftStart === null) leftStart = op.leftLine;
        if (rightStart === null) rightStart = op.rightLine;
        leftCount++;
        rightCount++;
        lines.push({ prefix: ' ', op });
      } else if (op.type === 'delete') {
        if (leftStart === null) leftStart = op.leftLine;
        leftCount++;
        lines.push({ prefix: '-', op });
      } else {
        if (rightStart === null) rightStart = op.rightLine;
        rightCount++;
        lines.push({ prefix: '+', op });
      }
    }

    return {
      leftStart: leftStart ?? 0,
      leftCount,
      rightStart: rightStart ?? 0,
      rightCount,
      lines,
    };
  });
}

function isBinaryBuffer(buf) {
  const len = Math.min(buf.length, 4096);
  for (let i = 0; i < len; i++) {
    if (buf[i] === 0) return true;
  }
  return false;
}

export async function runDiffCommand(args, ctx) {
  let unified = true;
  let ignoreWhitespace = false;
  let ignoreCase = false;
  let statOnly = false;
  let context = 3;
  const positional = [];

  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '-u' || a === '--unified') unified = true;
    else if (a === '--full') unified = false;
    else if (a === '-w' || a === '--ignore-whitespace') ignoreWhitespace = true;
    else if (a === '-i' || a === '--ignore-case') ignoreCase = true;
    else if (a === '--stat') statOnly = true;
    else if (a === '-C' || a === '--context') {
      const next = args[++i];
      if (next === undefined || !/^\d+$/.test(next)) {
        throw new CliUsageError('--context requires a non-negative integer.');
      }
      context = Number(next);
    } else if (a.startsWith('-')) {
      throw new CliUsageError(`Unknown option for diff: ${a}`);
    } else {
      positional.push(a);
    }
  }

  if (positional.length !== 2) {
    throw new CliUsageError('Usage: aether diff <leftFile> <rightFile> [--unified|--full] [-w] [-i] [--stat] [--context N]');
  }

  const leftPath = path.resolve(ctx.cwd, positional[0]);
  const rightPath = path.resolve(ctx.cwd, positional[1]);
  const [leftBuf, rightBuf] = await Promise.all([fs.readFile(leftPath), fs.readFile(rightPath)]);

  if (isBinaryBuffer(leftBuf) || isBinaryBuffer(rightBuf)) {
    const identical = leftBuf.equals(rightBuf);
    if (ctx.json) {
      printJson({ left: leftPath, right: rightPath, binary: true, identical });
      return identical ? 0 : 1;
    }
    console.log(
      identical
        ? ctx.style.green(`Binary files ${leftPath} and ${rightPath} are identical.`)
        : ctx.style.yellow(`Binary files ${leftPath} and ${rightPath} differ (${leftBuf.length} vs ${rightBuf.length} bytes).`),
    );
    return identical ? 0 : 1;
  }

  const result = computeLineDiff(leftBuf.toString('utf8'), rightBuf.toString('utf8'), {
    ignoreWhitespace,
    ignoreCase,
  });

  const hunks = buildUnifiedHunks(result.ops, context);

  if (ctx.json) {
    printJson({
      left: leftPath,
      right: rightPath,
      identical: result.identical,
      stats: result.stats,
      hunks: statOnly ? undefined : hunks,
    });
    return result.identical ? 0 : 1;
  }

  if (result.identical) {
    console.log(ctx.style.green('Files are identical.'));
    return 0;
  }

  const { added, removed, unchanged } = result.stats;
  if (statOnly) {
    console.log(
      `${ctx.style.bold(path.basename(leftPath))} -> ${ctx.style.bold(path.basename(rightPath))}: ` +
        `${ctx.style.green(`+${added}`)} ${ctx.style.red(`-${removed}`)} ${ctx.style.gray(`(${unchanged} unchanged)`)}`,
    );
    return 1;
  }

  console.log(ctx.style.red(`--- ${leftPath}`));
  console.log(ctx.style.green(`+++ ${rightPath}`));

  if (!unified) {
    for (const op of result.ops) {
      if (op.type === 'delete') console.log(ctx.style.red(`- ${op.text}`));
      else if (op.type === 'insert') console.log(ctx.style.green(`+ ${op.text}`));
      else console.log(ctx.style.gray(`  ${op.text}`));
    }
    return 1;
  }

  for (const h of hunks) {
    console.log(ctx.style.cyan(`@@ -${h.leftStart},${h.leftCount} +${h.rightStart},${h.rightCount} @@`));
    for (const { prefix, op } of h.lines) {
      if (prefix === '-') console.log(ctx.style.red(`-${op.text}`));
      else if (prefix === '+') console.log(ctx.style.green(`+${op.text}`));
      else console.log(` ${op.text}`);
    }
  }
  return 1;
}
