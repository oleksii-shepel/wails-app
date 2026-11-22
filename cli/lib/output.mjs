const ANSI = {
  reset: '\x1b[0m',
  bold: '\x1b[1m',
  dim: '\x1b[2m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  gray: '\x1b[90m',
};

export function shouldUseColor(noColorFlag = false) {
  if (noColorFlag) return false;
  if (process.env.NO_COLOR !== undefined && process.env.NO_COLOR !== '') return false;
  if (process.env.FORCE_COLOR && process.env.FORCE_COLOR !== '0') return true;
  return Boolean(process.stdout.isTTY);
}

export function createStyler(useColor) {
  const wrap = (code) => (text) => (useColor ? `${code}${text}${ANSI.reset}` : String(text));
  return {
    useColor,
    bold: wrap(ANSI.bold),
    dim: wrap(ANSI.dim),
    red: wrap(ANSI.red),
    green: wrap(ANSI.green),
    yellow: wrap(ANSI.yellow),
    blue: wrap(ANSI.blue),
    magenta: wrap(ANSI.magenta),
    cyan: wrap(ANSI.cyan),
    gray: wrap(ANSI.gray),
  };
}

export function formatBytes(bytes) {
  const n = Number(bytes || 0);
  if (!Number.isFinite(n) || n < 0) return '0 B';
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB', 'TB', 'PB'];
  let value = n / 1024;
  let idx = 0;
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024;
    idx++;
  }
  return `${value >= 100 ? value.toFixed(0) : value >= 10 ? value.toFixed(1) : value.toFixed(2)} ${units[idx]}`;
}

export function parseSize(raw, fallbackBytes = 1024 * 1024) {
  if (raw === undefined || raw === null || raw === '') return fallbackBytes;
  const str = String(raw).trim();
  const match = str.match(/^(\d+(?:\.\d+)?)\s*(b|k|kb|kib|m|mb|mib|g|gb|gib|t|tb|tib)?$/i);
  if (!match) {
    throw new CliUsageError(`Invalid size value: "${raw}". Use e.g. 512K, 10M, 1G, or bytes.`);
  }
  const num = Number(match[1]);
  const unit = (match[2] || 'b').toLowerCase();
  const multipliers = {
    b: 1,
    k: 1024,
    kb: 1024,
    kib: 1024,
    m: 1024 ** 2,
    mb: 1024 ** 2,
    mib: 1024 ** 2,
    g: 1024 ** 3,
    gb: 1024 ** 3,
    gib: 1024 ** 3,
    t: 1024 ** 4,
    tb: 1024 ** 4,
    tib: 1024 ** 4,
  };
  return Math.floor(num * multipliers[unit]);
}

export function formatDate(isoOrMs) {
  if (!isoOrMs) return '-';
  const d = new Date(isoOrMs);
  if (Number.isNaN(d.getTime())) return String(isoOrMs);
  return d.toISOString().replace('T', ' ').slice(0, 19);
}

export function renderBar(ratio, width = 18) {
  const clamped = Math.max(0, Math.min(1, Number(ratio) || 0));
  const filled = Math.round(clamped * width);
  return '█'.repeat(filled) + '░'.repeat(Math.max(0, width - filled));
}

export function stripAnsi(str) {
  // Remove ANSI escape sequences for column width calculation
  return String(str).replace(/\x1b\[[0-9;]*m/g, '');
}

export function renderTable(headers, rows, { alignRight = [] } = {}) {
  const widths = headers.map((h) => stripAnsi(h).length);
  for (const row of rows) {
    row.forEach((cell, i) => {
      widths[i] = Math.max(widths[i] || 0, stripAnsi(cell ?? '').length);
    });
  }
  const formatRow = (row) =>
    row
      .map((cell, i) => {
        const text = String(cell ?? '');
        const pad = Math.max(0, (widths[i] || 0) - stripAnsi(text).length);
        return alignRight[i] ? ' '.repeat(pad) + text : text + ' '.repeat(pad);
      })
      .join('  ');

  const lines = [formatRow(headers), widths.map((w) => '-'.repeat(w)).join('  ')];
  for (const row of rows) lines.push(formatRow(row));
  return lines.join('\n');
}

export function printJson(data) {
  process.stdout.write(`${JSON.stringify(data, null, 2)}\n`);
}

export class CliUsageError extends Error {
  constructor(message) {
    super(message);
    this.name = 'CliUsageError';
    this.exitCode = 2;
  }
}

export class CliRuntimeError extends Error {
  constructor(message, exitCode = 1) {
    super(message);
    this.name = 'CliRuntimeError';
    this.exitCode = exitCode;
  }
}
