import type { FileInfo } from '../shared/protocol';

export type SearchResultSort = 'relevance' | 'name' | 'path' | 'type';

export interface SearchResultWindow {
  readonly offset: number;
  readonly limit: number;
  readonly totalCount: number;
  readonly sourceCount: number;
  readonly entries: readonly FileInfo[];
}

export interface SearchDuplicateCandidateGroup {
  readonly key: string;
  readonly size: number;
  readonly count: number;
  readonly samplePaths: readonly string[];
}

export interface SearchResultIndexSnapshot {
  readonly sourceCount: number;
  readonly resultCount: number;
  readonly folderCount: number;
  readonly fileCount: number;
  readonly duplicateCandidateGroups: number;
}

type EntryFilter = (entry: FileInfo) => boolean;

interface IndexedEntry {
  readonly entry: FileInfo;
  readonly sequence: number;
}

interface HeapNode {
  readonly runIndex: number;
  readonly itemIndex: number;
  readonly value: IndexedEntry;
}

export class SearchResultIndex {
  private readonly chunks: IndexedEntry[][] = [];
  private runs: IndexedEntry[][] = [];
  private identity = new Map<string, IndexedEntry>();
  private duplicateCandidates = new Map<string, { size: number; paths: string[] }>();

  private sourceCountValue = 0;
  private resultCountValue = 0;
  private folderCountValue = 0;
  private nextSequence = 0;

  private sortValue: SearchResultSort = 'relevance';
  private filterValue: EntryFilter | null = null;

  get sourceCount(): number { return this.sourceCountValue; }
  get resultCount(): number { return this.resultCountValue; }
  get folderCount(): number { return this.folderCountValue; }
  get fileCount(): number { return this.resultCountValue - this.folderCountValue; }
  get sort(): SearchResultSort { return this.sortValue; }

  reset(sort: SearchResultSort = this.sortValue): void {
    this.chunks.length = 0;
    this.runs = [];
    this.identity.clear();
    this.duplicateCandidates.clear();
    this.sourceCountValue = 0;
    this.resultCountValue = 0;
    this.folderCountValue = 0;
    this.nextSequence = 0;
    this.sortValue = sort;
  }

  append(batch: readonly FileInfo[], sourceCount?: number): number {
    if (sourceCount !== undefined) {
      this.sourceCountValue = Math.max(this.sourceCountValue, Math.max(0, sourceCount));
    } else {
      this.sourceCountValue += batch.length;
    }

    const indexed: IndexedEntry[] = [];
    for (const candidate of batch) {
      const path = candidate.path ?? '';
      if (!path || this.identity.has(path)) continue;

      const value: IndexedEntry = {
        entry: candidate,
        sequence: this.nextSequence++,
      };
      this.identity.set(path, value);

      if (this.filterValue && !this.filterValue(candidate)) {
        continue;
      }

      indexed.push(value);
      this.resultCountValue += 1;
      if (candidate.type === 'DIRECTORY') this.folderCountValue += 1;
      this.indexDuplicateCandidate(candidate);
    }

    if (!indexed.length) return this.resultCountValue;

    this.chunks.push(indexed);
    this.runs.push(this.sortedRun(indexed));
    return this.resultCountValue;
  }

  setSort(sort: SearchResultSort): void {
    if (sort === this.sortValue) return;
    this.sortValue = sort;
    this.runs = this.chunks.map(chunk => this.sortedRun(chunk));
  }

  setFilter(filter: EntryFilter | null): void {
    if (filter === this.filterValue) return;
    this.filterValue = filter;
    this.rebuildProjection();
  }

  at(index: number): FileInfo | null {
    if (index < 0 || index >= this.resultCountValue) return null;
    return this.window(index, 1).entries[0] ?? null;
  }

  window(offset: number, limit: number): SearchResultWindow {
    const safeOffset = Math.max(0, Math.min(this.resultCountValue, Math.floor(offset)));
    const safeLimit = Math.max(0, Math.floor(limit));
    const entries = safeLimit > 0
      ? this.mergeWindow(safeOffset, safeLimit)
      : [];

    return {
      offset: safeOffset,
      limit: entries.length,
      totalCount: this.resultCountValue,
      sourceCount: Math.max(this.sourceCountValue, this.identity.size),
      entries,
    };
  }

  materialize(limit = Number.MAX_SAFE_INTEGER): FileInfo[] {
    return [...this.window(0, Math.min(this.resultCountValue, limit)).entries];
  }

  duplicateCandidateGroups(minCount = 2): readonly SearchDuplicateCandidateGroup[] {
    const groups: SearchDuplicateCandidateGroup[] = [];
    for (const [key, group] of this.duplicateCandidates) {
      if (group.paths.length < minCount) continue;
      groups.push({
        key,
        size: group.size,
        count: group.paths.length,
        samplePaths: group.paths.slice(0, 8),
      });
    }
    return groups.sort((a, b) => b.size - a.size || b.count - a.count);
  }

  snapshot(): SearchResultIndexSnapshot {
    return {
      sourceCount: Math.max(this.sourceCountValue, this.identity.size),
      resultCount: this.resultCountValue,
      folderCount: this.folderCountValue,
      fileCount: this.fileCount,
      duplicateCandidateGroups: this.duplicateCandidateGroups().length,
    };
  }

  private rebuildProjection(): void {
    this.runs = [];
    this.resultCountValue = 0;
    this.folderCountValue = 0;
    this.duplicateCandidates.clear();

    for (const chunk of this.chunks) {
      const filtered = this.filterValue
        ? chunk.filter(value => this.filterValue!(value.entry))
        : [...chunk];
      if (!filtered.length) continue;

      this.resultCountValue += filtered.length;
      for (const value of filtered) {
        if (value.entry.type === 'DIRECTORY') this.folderCountValue += 1;
        this.indexDuplicateCandidate(value.entry);
      }
      this.runs.push(this.sortedRun(filtered));
    }
  }

  private sortedRun(values: readonly IndexedEntry[]): IndexedEntry[] {
    if (this.sortValue === 'relevance') {
      return [...values];
    }
    return [...values].sort((a, b) => this.compare(a, b));
  }

  private compare(a: IndexedEntry, b: IndexedEntry): number {
    const left = a.entry;
    const right = b.entry;

    let result = 0;
    if (this.sortValue === 'name') {
      result = this.text(left.name).localeCompare(this.text(right.name), undefined, {
        numeric: true,
        sensitivity: 'base',
      });
    } else if (this.sortValue === 'path') {
      result = this.text(left.path).localeCompare(this.text(right.path), undefined, {
        numeric: true,
        sensitivity: 'base',
      });
    } else if (this.sortValue === 'type') {
      result = this.text(left.type).localeCompare(this.text(right.type)) ||
        this.text(left.name).localeCompare(this.text(right.name), undefined, {
          numeric: true,
          sensitivity: 'base',
        });
    }

    return result ||
      this.text(left.path).localeCompare(this.text(right.path), undefined, {
        numeric: true,
        sensitivity: 'base',
      }) ||
      a.sequence - b.sequence;
  }

  private mergeWindow(offset: number, limit: number): FileInfo[] {
    if (!this.runs.length || limit <= 0) return [];

    if (this.sortValue === 'relevance') {
      return this.insertionWindow(offset, limit);
    }

    const heap: HeapNode[] = [];
    for (let runIndex = 0; runIndex < this.runs.length; runIndex += 1) {
      const first = this.runs[runIndex][0];
      if (first) this.heapPush(heap, { runIndex, itemIndex: 0, value: first });
    }

    const output: FileInfo[] = [];
    let logicalIndex = 0;
    const end = offset + limit;

    while (heap.length && logicalIndex < end) {
      const node = this.heapPop(heap)!;
      if (logicalIndex >= offset) output.push(node.value.entry);
      logicalIndex += 1;

      const nextIndex = node.itemIndex + 1;
      const next = this.runs[node.runIndex][nextIndex];
      if (next) {
        this.heapPush(heap, {
          runIndex: node.runIndex,
          itemIndex: nextIndex,
          value: next,
        });
      }
    }
    return output;
  }

  private insertionWindow(offset: number, limit: number): FileInfo[] {
    const output: FileInfo[] = [];
    let skipped = 0;

    for (const run of this.runs) {
      if (skipped + run.length <= offset) {
        skipped += run.length;
        continue;
      }
      const start = Math.max(0, offset - skipped);
      for (let index = start; index < run.length && output.length < limit; index += 1) {
        output.push(run[index].entry);
      }
      skipped += run.length;
      if (output.length >= limit) break;
    }
    return output;
  }

  private heapPush(heap: HeapNode[], node: HeapNode): void {
    heap.push(node);
    let index = heap.length - 1;
    while (index > 0) {
      const parent = Math.floor((index - 1) / 2);
      if (this.compare(heap[parent].value, node.value) <= 0) break;
      heap[index] = heap[parent];
      index = parent;
    }
    heap[index] = node;
  }

  private heapPop(heap: HeapNode[]): HeapNode | undefined {
    if (!heap.length) return undefined;
    const root = heap[0];
    const last = heap.pop()!;
    if (!heap.length) return root;

    let index = 0;
    while (true) {
      const left = index * 2 + 1;
      const right = left + 1;
      if (left >= heap.length) break;

      let smallest = left;
      if (
        right < heap.length &&
        this.compare(heap[right].value, heap[left].value) < 0
      ) {
        smallest = right;
      }
      if (this.compare(last.value, heap[smallest].value) <= 0) break;

      heap[index] = heap[smallest];
      index = smallest;
    }
    heap[index] = last;
    return root;
  }

  private indexDuplicateCandidate(entry: FileInfo): void {
    if (entry.type === 'DIRECTORY' || !Number.isFinite(entry.size) || entry.size <= 0) return;

    // This is deliberately a cheap *candidate* grouping, not an exact-content
    // duplicate claim. Exact duplicate confirmation can hash only these groups.
    const extension = this.extension(entry.name ?? '');
    const key = `${entry.size}|${extension}`;
    let group = this.duplicateCandidates.get(key);
    if (!group) {
      group = { size: entry.size, paths: [] };
      this.duplicateCandidates.set(key, group);
    }
    group.paths.push(entry.path);
  }

  private extension(name: string): string {
    const index = name.lastIndexOf('.');
    return index > 0 ? name.slice(index + 1).toLocaleLowerCase() : '';
  }

  private text(value: unknown): string {
    return String(value ?? '');
  }
}