export interface BatchRenameTemplateContext {
  readonly originalName: string;
  readonly index: number;
  readonly counter: number;
  readonly counterWidth: number;
  readonly date?: Date;
}

export interface BatchRenameParts {
  readonly stem: string;
  readonly ext: string;
}

export function splitBatchRenameName(name: string): BatchRenameParts {
  if (name.startsWith('.')) {
    const dot = name.indexOf('.', 1);
    if (dot < 0) return { stem: name, ext: '' };
    return { stem: name.slice(0, dot), ext: name.slice(dot) };
  }

  const dot = name.lastIndexOf('.');
  if (dot < 0) return { stem: name, ext: '' };
  return { stem: name.slice(0, dot), ext: name.slice(dot) };
}

function pad(value: number, width: number): string {
  return String(value).padStart(Math.max(1, width), '0');
}

function isoDate(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

function applyToken(token: string, context: BatchRenameTemplateContext): string | null {
  const { stem, ext } = splitBatchRenameName(context.originalName);
  const normalized = token.trim();

  if (normalized === 'name') return stem;
  if (normalized === 'ext') return ext.replace(/^\./, '');
  if (normalized === '.ext') return ext;
  if (normalized === 'index') return String(context.index + 1);
  if (normalized === 'date') return isoDate(context.date ?? new Date());

  if (normalized === 'upper:name') return stem.toLocaleUpperCase();
  if (normalized === 'lower:name') return stem.toLocaleLowerCase();
  if (normalized === 'kebab:name') {
    return stem
      .trim()
      .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
      .replace(/[^a-zA-Z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .toLocaleLowerCase();
  }
  if (normalized === 'snake:name') {
    return stem
      .trim()
      .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
      .replace(/[^a-zA-Z0-9]+/g, '_')
      .replace(/^_+|_+$/g, '')
      .toLocaleLowerCase();
  }
  if (normalized === 'title:name') {
    return stem
      .replace(/[_-]+/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
      .replace(/\b\w/g, (ch) => ch.toLocaleUpperCase());
  }

  const counter = /^n(?::(\d+))?$/.exec(normalized);
  if (counter) {
    return pad(context.counter, Number(counter[1]) || context.counterWidth);
  }

  const replace = /^replace:(.*?)=>(.*)$/.exec(normalized);
  if (replace) {
    const from = replace[1];
    const to = replace[2];
    return from ? stem.split(from).join(to) : stem;
  }

  return null;
}

export function applyBatchRenameTemplate(
  pattern: string,
  context: BatchRenameTemplateContext,
): string {
  const { stem, ext } = splitBatchRenameName(context.originalName);

  // Keep the original compact syntax fully compatible.
  let result = '';
  for (let i = 0; i < pattern.length; i += 1) {
    const char = pattern[i];

    if (char === '*') {
      result += stem;
      continue;
    }

    if (char === '&') {
      result += ext;
      continue;
    }

    if (char === '#') {
      let width = 1;
      while (pattern[i + width] === '#') width += 1;
      result += pad(context.counter, Math.max(width, context.counterWidth));
      i += width - 1;
      continue;
    }

    if (char === '{') {
      const end = pattern.indexOf('}', i + 1);
      if (end >= 0) {
        const raw = pattern.slice(i + 1, end);
        const token = applyToken(raw, context);
        if (token !== null) {
          result += token;
          i = end;
          continue;
        }
      }
    }

    result += char;
  }

  return result;
}