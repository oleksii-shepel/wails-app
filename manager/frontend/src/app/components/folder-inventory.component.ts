import { ChangeDetectionStrategy, Component, EventEmitter, Input, OnChanges, OnDestroy, Output, inject, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { ApiService } from '../services/api.service';
import { SelectionBasketService } from '../services/selection-basket.service';
import { FileType, type FileInfo } from '../shared/protocol';

@Component({
  selector: 'app-folder-inventory',
  imports: [DatePipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="palette-dialog-toolbar inventory-toolbar">
      <input class="palette-dialog-field" type="search" placeholder="Filter by name or path"
        aria-label="Filter inventory" [value]="query" (input)="filter($event)" />
      <select class="palette-dialog-field" aria-label="Sort inventory" [value]="sort"
        (change)="changeSort($event)">
        <option value="name">Name</option>
        <option value="size">Largest first</option>
        <option value="modified">Newest first</option>
        <option value="size-asc">Smallest first</option>
        <option value="modified-asc">Oldest first</option>
      </select>
      <select class="palette-dialog-field" aria-label="Entry type" [value]="kind" (change)="changeKind($event)">
        <option value="all">All entries</option>
        <option value="folders">Folders</option>
        <option value="files">Files and links</option>
      </select>
      <button type="button" class="palette-dialog-button" [disabled]="!query && kind === 'all' && sort === 'name'" (click)="resetFilters()">Reset filters</button>
      <label><input type="checkbox" [checked]="showHidden" (change)="changeHidden($event)" /> Hidden</label>
      <button type="button" class="palette-dialog-button" [disabled]="!rootPath || loading()" (click)="load()">Refresh</button>
    </div>
    <div class="palette-dialog-toolbar inventory-toolbar">
      <button type="button" class="palette-dialog-button" [disabled]="!visible.length" (click)="copyPaths()">Copy visible paths</button>
      <button type="button" class="palette-dialog-button" [disabled]="!visible.length" (click)="download()">Export CSV</button>
      <button type="button" class="palette-dialog-button palette-dialog-button--primary"
        [disabled]="!selected.size" (click)="addToBasket()">Add selected to Basket</button>
      <button type="button" class="palette-dialog-button" [disabled]="!selected.size" (click)="clearSelection()">Clear selection</button>
      <span role="status" class="palette-dialog-meta">{{ notice() || (visible.length + ' of ' + totalEntries + ' match · ' + selected.size + ' selected') }}</span>
    </div>
    @if (loading()) {
      <p class="palette-dialog-empty" role="status">Loading folder…</p>
    } @else if (error()) {
      <p class="palette-dialog-empty" role="alert">{{ error() }}</p>
    } @else if (!visible.length) {
      <p class="palette-dialog-empty">
        {{ !rootPath ? 'Open a folder in a file pane to create an inventory.' : entriesExist ? 'No entries match the current filters.' : 'This folder is empty.' }}
      </p>
    } @else {
      <table>
        <thead><tr>
          <th scope="col"><input type="checkbox" aria-label="Select all visible entries" [checked]="allVisibleSelected"
            [indeterminate]="!allVisibleSelected && someVisibleSelected" (change)="toggleVisible($event)" /></th>
          <th scope="col">Name</th><th scope="col">Type</th><th scope="col">Bytes</th><th scope="col">Modified</th>
        </tr></thead>
        <tbody>
          @for (entry of visible.slice(0, limit); track entry.path) {
            <tr>
              <td><input type="checkbox" [attr.aria-label]="'Select ' + entry.name" [checked]="selected.has(entry.path)"
                (change)="toggle(entry.path, $event)" /></td>
              <td [title]="entry.path">
                @if (entry.type === directoryType) {
                  <button type="button" class="palette-dialog-button folder-link" (click)="navigateTo.emit(entry.path)">{{ entry.name }}</button>
                } @else {
                  {{ entry.name }}
                }
              </td>
              <td>{{ entry.type.toLowerCase() }}</td>
              <td>{{ entry.type === directoryType ? '—' : entry.size }}</td>
              <td>{{ entry.modified > 0 ? (entry.modified * 1000 | date:'short') : '—' }}</td>
            </tr>
          }
        </tbody>
      </table>
      @if (visible.length > limit) {
        <div class="palette-dialog-toolbar">
          <button type="button" class="palette-dialog-button" (click)="limit = limit + 250">Show more rows</button>
        </div>
      }
    }
  `,
  styles: [`
    .inventory-toolbar { flex-wrap: wrap; }
    .inventory-toolbar input[type='search'] { flex: 1; min-width: min(14rem, 100%); }
    table { width: 100%; table-layout: fixed; border-collapse: collapse; font-variant-numeric: tabular-nums; }
    th, td { padding: 6px 8px; text-align: start; border-block-end: 1px solid var(--vsc-border-subtle, var(--vsc-border)); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    th { color: var(--vsc-description-foreground); font-weight: 600; }
    th:first-child { width: 36px; }
    th:nth-child(3), th:nth-child(4) { width: 15%; }
    th:last-child { width: 25%; }
    tbody tr:hover, tbody tr:focus-within { background: var(--vsc-list-hover-background); }
    .folder-link { max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
    @media (max-width: 520px) { th:last-child, td:last-child { display: none; } }
  `],
})
export class FolderInventoryComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) rootPath = '';
  @Output() navigateTo = new EventEmitter<string>();

  private readonly api = inject(ApiService);
  private readonly basket = inject(SelectionBasketService);
  private request = 0;
  private entries: FileInfo[] = [];
  readonly loading = signal(false);
  readonly error = signal('');
  readonly notice = signal('');
  readonly directoryType = FileType.DIRECTORY;
  readonly selected = new Set<string>();
  visible: FileInfo[] = [];
  query = '';
  sort = 'name';
  kind = 'all';
  showHidden = false;
  limit = 250;

  get someVisibleSelected(): boolean {
    return this.visible.some(entry => this.selected.has(entry.path));
  }

  get totalEntries(): number { return this.entries.length; }

  get entriesExist(): boolean { return this.totalEntries > 0; }

  get allVisibleSelected(): boolean {
    return this.visible.length > 0 && this.visible.every(entry => this.selected.has(entry.path));
  }

  ngOnChanges(): void { void this.load(); }
  ngOnDestroy(): void { this.request++; }

  async load(): Promise<void> {
    const request = ++this.request;
    this.entries = [];
    this.selected.clear();
    this.error.set('');
    this.notice.set('');
    this.updateVisible();
    this.loading.set(!!this.rootPath);
    if (!this.rootPath) return;
    try {
      const result = await this.api.listDirectory(this.rootPath, this.showHidden);
      if (request !== this.request) return;
      this.entries = result.entries ?? [];
      this.updateVisible();
    } catch (error) {
      if (request === this.request) this.error.set(error instanceof Error ? error.message : String(error));
    } finally {
      if (request === this.request) this.loading.set(false);
    }
  }

  filter(event: Event): void {
    this.query = (event.target as HTMLInputElement).value;
    this.updateVisible();
  }

  changeSort(event: Event): void {
    this.sort = (event.target as HTMLSelectElement).value;
    this.updateVisible();
  }

  changeHidden(event: Event): void {
    this.showHidden = (event.target as HTMLInputElement).checked;
    void this.load();
  }

  resetFilters(): void {
    this.query = '';
    this.kind = 'all';
    this.sort = 'name';
    this.updateVisible();
  }

  changeKind(event: Event): void {
    this.kind = (event.target as HTMLSelectElement).value;
    this.updateVisible();
  }

  toggle(path: string, event: Event): void {
    this.notice.set('');
    if ((event.target as HTMLInputElement).checked) this.selected.add(path);
    else this.selected.delete(path);
  }

  toggleVisible(event: Event): void {
    this.notice.set('');
    const checked = (event.target as HTMLInputElement).checked;
    for (const entry of this.visible) {
      if (checked) this.selected.add(entry.path);
      else this.selected.delete(entry.path);
    }
  }

  addToBasket(): void {
    const added = this.basket.add([...this.selected], 'folder inventory');
    this.notice.set(added ? `Added ${added} to Selection Basket.` : 'Selected paths are already in Selection Basket.');
    this.selected.clear();
  }

  clearSelection(): void {
    this.selected.clear();
    this.notice.set('');
  }

  async copyPaths(): Promise<void> {
    const paths = this.visible.map(entry => entry.path);
    try {
      await navigator.clipboard.writeText(paths.join('\n'));
      this.notice.set(`Copied ${paths.length} paths.`);
    } catch {
      this.notice.set('Clipboard unavailable. Export CSV to save the paths.');
    }
  }

  download(): void {
    const csvCell = (value: string | number): string => {
      const text = String(value);
      const safe = typeof value === 'string' && /^[\s]*[=+@-]|^[\t\r\n]/.test(text) ? "'" + text : text;
      return '"' + safe.replace(/"/g, '""') + '"';
    };
    const rows = this.visible.map(entry => [
      entry.name, entry.path, entry.type,
      entry.type === FileType.DIRECTORY ? '' : entry.size, entry.modified,
    ]);
    const csv = [['Name', 'Path', 'Type', 'Bytes', 'Modified (Unix seconds)'], ...rows]
      .map(row => row.map(csvCell).join(',')).join('\r\n');
    const url = URL.createObjectURL(new Blob(['\uFEFF', csv], { type: 'text/csv;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'folder-inventory.csv';
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  private updateVisible(): void {
    this.notice.set('');
    const query = this.query.trim().toLowerCase();
    this.limit = 250;
    this.visible = this.entries.filter(entry => {
      const folder = entry.type === FileType.DIRECTORY;
      if (this.kind !== 'all' && folder !== (this.kind === 'folders')) return false;
      return entry.name.toLowerCase().includes(query) || entry.path.toLowerCase().includes(query);
    }).sort((a, b) => {
      const order = this.sort === 'size' ? b.size - a.size
        : this.sort === 'size-asc' ? a.size - b.size
        : this.sort === 'modified' ? b.modified - a.modified
        : this.sort === 'modified-asc' ? a.modified - b.modified : 0;
      return order || a.name.localeCompare(b.name, undefined, { numeric: true });
    });
  }
}
