import { Injectable } from '@angular/core';
import { ApiService } from './api.service';

export type DiffRowKind = 'equal' | 'add' | 'remove' | 'modify';

export interface DiffRow {
  readonly index: number;
  readonly kind: DiffRowKind;
  readonly leftLine: number | null;
  readonly rightLine: number | null;
  readonly leftText: string;
  readonly rightText: string;
  readonly hunkIndex: number | null;
}

export interface DiffHunk {
  readonly index: number;
  readonly firstRowIndex: number;
  readonly leftStart: number;
  readonly leftCount: number;
  readonly rightStart: number;
  readonly rightCount: number;
  readonly header: string;
}

export interface DiffStats {
  readonly added: number;
  readonly removed: number;
  readonly modified: number;
  readonly unchanged: number;
  readonly totalLeftLines: number;
  readonly totalRightLines: number;
}

export interface DiffOptions {
  readonly ignoreWhitespace?: boolean;
  readonly ignoreCase?: boolean;
}

export interface DiffResult {
  readonly rows: readonly DiffRow[];
  readonly hunks: readonly DiffHunk[];
  readonly stats: DiffStats;
  readonly isIdentical: boolean;
}

export interface LoadedDiffFile {
  readonly path: string;
  readonly content: string;
  readonly size: number;
  readonly isBinary: boolean;
  readonly tooLarge: boolean;
  readonly error?: string;
}

const MAX_DIFF_BYTES = 2 * 1024 * 1024;
const MAX_LCS_CELLS = 300_000;

@Injectable({ providedIn: 'root' })
export class FileDiffService {
  constructor(private readonly api: ApiService) {}

  async loadFileForDiff(path: string): Promise<LoadedDiffFile> {
    const target = path.trim();
    if (!target) {
      return {
        path: '',
        content: '',
        size: 0,
        isBinary: false,
        tooLarge: false,
        error: 'No file path selected.',
      };
    }

    try {
      const info = await this.api.getFileInfo(target);
      if (info.size > MAX_DIFF_BYTES) {
        return {
          path: target,
          content: '',
          size: info.size,
          isBinary: false,
          tooLarge: true,
          error: `File exceeds ${Math.round(MAX_DIFF_BYTES / (1024 * 1024))} MB inline diff limit.`,
        };
      }

      const probe = await this.api.probeFile(target).catch(() => null);
      if (probe && !probe.isText) {
        return {
          path: target,
          content: '',
          size: info.size,
          isBinary: true,
          tooLarge: false,
          error: `Binary file (${probe.mimeType || 'application/octet-stream'}) cannot be diffed as text.`,
        };
      }

      const fileContent = await this.api.readFile(target, 'utf8');
      const text = fileContent.content ?? '';
      if (text.includes('\u0000')) {
        return {
          path: target,
          content: '',
          size: fileContent.size ?? info.size,
          isBinary: true,
          tooLarge: false,
          error: 'Binary file detected (contains NUL bytes).',
        };
      }

      return {
        path: target,
        content: text,
        size: fileContent.size ?? info.size,
        isBinary: false,
        tooLarge: false,
      };
    } catch (error) {
      return {
        path: target,
        content: '',
        size: 0,
        isBinary: false,
        tooLarge: false,
        error: error instanceof Error ? error.message : String(error),
      };
    }
  }

  computeLineDiff(
    leftText: string,
    rightText: string,
    options: DiffOptions = {},
  ): DiffResult {
    const leftLines = this.splitLines(leftText);
    const rightLines = this.splitLines(rightText);

    const normalize = (line: string): string => {
      let out = line;
      if (options.ignoreWhitespace) {
        out = out.replace(/\s+/g, ' ').trim();
      }
      if (options.ignoreCase) {
        out = out.toLowerCase();
      }
      return out;
    };

    const normLeft = leftLines.map(normalize);
    const normRight = rightLines.map(normalize);

    // Trim common prefix and suffix to keep LCS fast on realistic files.
    let prefix = 0;
    while (
      prefix < normLeft.length &&
      prefix < normRight.length &&
      normLeft[prefix] === normRight[prefix]
    ) {
      prefix += 1;
    }

    let suffix = 0;
    while (
      suffix < normLeft.length - prefix &&
      suffix < normRight.length - prefix &&
      normLeft[normLeft.length - 1 - suffix] === normRight[normRight.length - 1 - suffix]
    ) {
      suffix += 1;
    }

    const midLeftStart = prefix;
    const midLeftEnd = normLeft.length - suffix;
    const midRightStart = prefix;
    const midRightEnd = normRight.length - suffix;

    interface RawOp {
      type: 'equal' | 'remove' | 'add';
      leftIdx?: number;
      rightIdx?: number;
    }

    const rawOps: RawOp[] = [];
    for (let i = 0; i < prefix; i += 1) {
      rawOps.push({ type: 'equal', leftIdx: i, rightIdx: i });
    }

    const leftLen = midLeftEnd - midLeftStart;
    const rightLen = midRightEnd - midRightStart;

    if (leftLen > 0 || rightLen > 0) {
      if (leftLen * rightLen <= MAX_LCS_CELLS) {
        const dp = new Int32Array((leftLen + 1) * (rightLen + 1));
        const stride = rightLen + 1;
        for (let i = leftLen - 1; i >= 0; i -= 1) {
          for (let j = rightLen - 1; j >= 0; j -= 1) {
            if (normLeft[midLeftStart + i] === normRight[midRightStart + j]) {
              dp[i * stride + j] = dp[(i + 1) * stride + (j + 1)] + 1;
            } else {
              const down = dp[(i + 1) * stride + j];
              const right = dp[i * stride + (j + 1)];
              dp[i * stride + j] = down >= right ? down : right;
            }
          }
        }

        let i = 0;
        let j = 0;
        while (i < leftLen && j < rightLen) {
          if (normLeft[midLeftStart + i] === normRight[midRightStart + j]) {
            rawOps.push({
              type: 'equal',
              leftIdx: midLeftStart + i,
              rightIdx: midRightStart + j,
            });
            i += 1;
            j += 1;
          } else if (dp[(i + 1) * stride + j] >= dp[i * stride + (j + 1)]) {
            rawOps.push({ type: 'remove', leftIdx: midLeftStart + i });
            i += 1;
          } else {
            rawOps.push({ type: 'add', rightIdx: midRightStart + j });
            j += 1;
          }
        }
        while (i < leftLen) {
          rawOps.push({ type: 'remove', leftIdx: midLeftStart + i });
          i += 1;
        }
        while (j < rightLen) {
          rawOps.push({ type: 'add', rightIdx: midRightStart + j });
          j += 1;
        }
      } else {
        // Fallback linear scan for huge multi-thousand-line blocks
        const common = Math.min(leftLen, rightLen);
        for (let k = 0; k < common; k += 1) {
          const lIdx = midLeftStart + k;
          const rIdx = midRightStart + k;
          if (normLeft[lIdx] === normRight[rIdx]) {
            rawOps.push({ type: 'equal', leftIdx: lIdx, rightIdx: rIdx });
          } else {
            rawOps.push({ type: 'remove', leftIdx: lIdx });
            rawOps.push({ type: 'add', rightIdx: rIdx });
          }
        }
        for (let k = common; k < leftLen; k += 1) {
          rawOps.push({ type: 'remove', leftIdx: midLeftStart + k });
        }
        for (let k = common; k < rightLen; k += 1) {
          rawOps.push({ type: 'add', rightIdx: midRightStart + k });
        }
      }
    }

    for (let s = 0; s < suffix; s += 1) {
      rawOps.push({
        type: 'equal',
        leftIdx: midLeftEnd + s,
        rightIdx: midRightEnd + s,
      });
    }

    // Group adjacent remove + add runs into paired 'modify' rows for clean alignment
    const rows: DiffRow[] = [];
    const hunks: DiffHunk[] = [];
    let added = 0;
    let removed = 0;
    let modified = 0;
    let unchanged = 0;

    let idx = 0;
    let currentHunkIndex = -1;
    let hunkFirstRow = -1;
    let hunkLeftStart = 0;
    let hunkLeftCount = 0;
    let hunkRightStart = 0;
    let hunkRightCount = 0;

    const flushHunk = () => {
      if (currentHunkIndex < 0) return;
      const hIndex = currentHunkIndex;
      const header = `@@ -${hunkLeftStart},${hunkLeftCount} +${hunkRightStart},${hunkRightCount} @@`;
      hunks.push({
        index: hIndex,
        firstRowIndex: hunkFirstRow,
        leftStart: hunkLeftStart,
        leftCount: hunkLeftCount,
        rightStart: hunkRightStart,
        rightCount: hunkRightCount,
        header,
      });
      currentHunkIndex = -1;
    };

    const ensureHunk = (leftLine: number | null, rightLine: number | null): number => {
      if (currentHunkIndex < 0) {
        currentHunkIndex = hunks.length;
        hunkFirstRow = rows.length;
        hunkLeftStart = leftLine ?? 1;
        hunkRightStart = rightLine ?? 1;
        hunkLeftCount = 0;
        hunkRightCount = 0;
      }
      return currentHunkIndex;
    };

    while (idx < rawOps.length) {
      const op = rawOps[idx];
      if (op.type === 'equal') {
        flushHunk();
        const lIdx = op.leftIdx!;
        const rIdx = op.rightIdx!;
        rows.push({
          index: rows.length,
          kind: 'equal',
          leftLine: lIdx + 1,
          rightLine: rIdx + 1,
          leftText: leftLines[lIdx] ?? '',
          rightText: rightLines[rIdx] ?? '',
          hunkIndex: null,
        });
        unchanged += 1;
        idx += 1;
        continue;
      }

      const removes: number[] = [];
      const adds: number[] = [];
      while (idx < rawOps.length && rawOps[idx].type === 'remove') {
        removes.push(rawOps[idx].leftIdx!);
        idx += 1;
      }
      while (idx < rawOps.length && rawOps[idx].type === 'add') {
        adds.push(rawOps[idx].rightIdx!);
        idx += 1;
      }

      const paired = Math.min(removes.length, adds.length);
      for (let p = 0; p < paired; p += 1) {
        const lIdx = removes[p];
        const rIdx = adds[p];
        const hIdx = ensureHunk(lIdx + 1, rIdx + 1);
        hunkLeftCount += 1;
        hunkRightCount += 1;
        rows.push({
          index: rows.length,
          kind: 'modify',
          leftLine: lIdx + 1,
          rightLine: rIdx + 1,
          leftText: leftLines[lIdx] ?? '',
          rightText: rightLines[rIdx] ?? '',
          hunkIndex: hIdx,
        });
        modified += 1;
      }

      for (let r = paired; r < removes.length; r += 1) {
        const lIdx = removes[r];
        const hIdx = ensureHunk(lIdx + 1, null);
        hunkLeftCount += 1;
        rows.push({
          index: rows.length,
          kind: 'remove',
          leftLine: lIdx + 1,
          rightLine: null,
          leftText: leftLines[lIdx] ?? '',
          rightText: '',
          hunkIndex: hIdx,
        });
        removed += 1;
      }

      for (let a = paired; a < adds.length; a += 1) {
        const rIdx = adds[a];
        const hIdx = ensureHunk(null, rIdx + 1);
        hunkRightCount += 1;
        rows.push({
          index: rows.length,
          kind: 'add',
          leftLine: null,
          rightLine: rIdx + 1,
          leftText: '',
          rightText: rightLines[rIdx] ?? '',
          hunkIndex: hIdx,
        });
        added += 1;
      }
    }

    flushHunk();

    return {
      rows,
      hunks,
      stats: {
        added,
        removed,
        modified,
        unchanged,
        totalLeftLines: leftLines.length,
        totalRightLines: rightLines.length,
      },
      isIdentical: added === 0 && removed === 0 && modified === 0,
    };
  }

  formatUnifiedPatch(
    leftLabel: string,
    rightLabel: string,
    diff: DiffResult,
    contextLines = 3,
  ): string {
    const out: string[] = [
      `--- ${leftLabel || 'left'}`,
      `+++ ${rightLabel || 'right'}`,
    ];

    if (diff.hunks.length === 0) {
      return out.join('\n');
    }

    const rows = diff.rows;
    const included = new Set<number>();
    for (const row of rows) {
      if (row.kind !== 'equal') {
        const start = Math.max(0, row.index - contextLines);
        const end = Math.min(rows.length - 1, row.index + contextLines);
        for (let k = start; k <= end; k += 1) {
          included.add(k);
        }
      }
    }

    const sorted = Array.from(included).sort((a, b) => a - b);
    let blockStart = -1;
    const blocks: number[][] = [];
    let current: number[] = [];

    for (const rowIdx of sorted) {
      if (blockStart === -1 || rowIdx === current[current.length - 1] + 1) {
        if (blockStart === -1) blockStart = rowIdx;
        current.push(rowIdx);
      } else {
        blocks.push(current);
        current = [rowIdx];
      }
    }
    if (current.length > 0) blocks.push(current);

    for (const block of blocks) {
      let lStart = 0;
      let rStart = 0;
      let lCount = 0;
      let rCount = 0;
      const lines: string[] = [];

      for (const rowIdx of block) {
        const r = rows[rowIdx];
        if (!lStart && r.leftLine) lStart = r.leftLine;
        if (!rStart && r.rightLine) rStart = r.rightLine;
        if (r.kind === 'equal') {
          lCount += 1;
          rCount += 1;
          lines.push(` ${r.leftText}`);
        } else if (r.kind === 'remove') {
          lCount += 1;
          lines.push(`-${r.leftText}`);
        } else if (r.kind === 'add') {
          rCount += 1;
          lines.push(`+${r.rightText}`);
        } else {
          lCount += 1;
          rCount += 1;
          lines.push(`-${r.leftText}`);
          lines.push(`+${r.rightText}`);
        }
      }

      out.push(`@@ -${lStart || 1},${lCount} +${rStart || 1},${rCount} @@`);
      out.push(...lines);
    }

    return out.join('\n');
  }

  private splitLines(content: string): string[] {
    if (!content) return [];
    const normalized = content.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    const lines = normalized.split('\n');
    if (lines.length > 1 && lines[lines.length - 1] === '') {
      lines.pop();
    }
    return lines;
  }
}
