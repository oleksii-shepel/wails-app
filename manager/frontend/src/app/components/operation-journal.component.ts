import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, Input, OnChanges, Output, SimpleChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OperationHistoryService, type OperationJournalEntry } from '../services/operation-history.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';

type JournalFilter = 'all' | 'undoable' | 'failed';

@Component({
  selector: 'app-operation-journal',
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop palette-dialog-backdrop dialog-group" *ngIf="renderVisible" [class.dialog-group--closing]="isClosing" (mousedown)="close()" (animationend)="onAnimationEnd($event)">
      <section class="journal palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="Operation History"
        (mousedown)="$event.stopPropagation()" (keydown.escape)="close(); $event.preventDefault(); $event.stopPropagation()">
        <header class="palette-dialog-header"><div class="palette-dialog-title"><strong>Operation History</strong><span class="palette-dialog-subtitle">{{ entries.length }} recent operations</span></div>
          <span class="dialog-close" aria-label="Close tab" title="Close" (click)="close()" role="button" tabindex="0" (keydown.enter)="close()" (keydown.space)="close(); $event.preventDefault()"> × </span></header>
        <div class="journal-toolbar palette-dialog-toolbar">
          <div class="filter-tabs" role="group" aria-label="Journal filters">
            <button type="button" [class.active]="filter === 'all'" (click)="setFilter('all')">All</button>
            <button type="button" [class.active]="filter === 'undoable'" (click)="setFilter('undoable')">Undoable</button>
            <button type="button" [class.active]="filter === 'failed'" (click)="setFilter('failed')">Failed</button>
          </div>
          <input
            type="search"
            class="journal-search"
            [value]="searchText"
            (input)="onSearchInput($event)"
            placeholder="Filter operation, summary, or path…"
            aria-label="Filter operation journal" />
        </div>
        <div class="entries palette-dialog-body">
          <article *ngFor="let entry of filteredEntries; trackBy: trackEntry" [class.failed]="entry.state === 'failed' || entry.state === 'partial'">
            <span class="state">{{ stateIcon(entry) }}</span>
            <div class="main"><strong>{{ entry.label }}</strong>
              <small>{{ entry.kind }} · {{ entry.completedAt | date:'short' }} · {{ entry.reversible ? 'undoable' : 'not undoable' }}</small>
              <span *ngIf="entry.summary">{{ entry.summary }}</span>
              <span class="error" *ngIf="entry.error">{{ entry.error }}</span>
              <details *ngIf="entry.affectedPaths.length"><summary>{{ entry.affectedPaths.length }} affected paths</summary>
                <code *ngFor="let path of entry.affectedPaths">{{ path }}</code></details>
            </div>
          </article>
          <div class="empty palette-dialog-empty" *ngIf="entries.length === 0">No file operations recorded in this session.</div>
          <div class="empty palette-dialog-empty" *ngIf="entries.length > 0 && filteredEntries.length === 0">No operations match the active filter.</div>
        </div>
        <footer class="palette-dialog-footer"><div class="palette-dialog-footer-actions"><button class="palette-dialog-button" [disabled]="!history.canUndo" (click)="undo()">Undo</button>
          <button class="palette-dialog-button" [disabled]="!history.canRedo" (click)="redo()">Redo</button></div>
          <div class="palette-dialog-footer-actions"><button class="palette-dialog-button" [disabled]="entries.length === 0" (click)="clear()">Clear History</button>
          <button class="palette-dialog-button palette-dialog-button--primary" (click)="close()">Close</button></div></footer>
      </section>
    </div>`,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    :host{position:fixed;inset:0;z-index:8700;pointer-events:none}
    .journal{display:grid;grid-template-rows:auto auto minmax(0,1fr) auto;width:min(780px,calc(100vw - 24px));height:min(680px,calc(100vh - 40px))}
    .journal-toolbar{display:flex;align-items:center;flex-wrap:wrap;gap:6px;padding:7px 10px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border))}
    .filter-tabs{display:inline-flex;gap:3px}
    .filter-tabs button{min-height:25px;padding:2px 8px;border:1px solid var(--vsc-border-subtle,var(--vsc-border));border-radius:4px;background:transparent;color:var(--vsc-description-foreground);font:inherit;font-size:11px;cursor:pointer}
    .filter-tabs button.active{color:var(--vsc-foreground);border-color:color-mix(in srgb,var(--vsc-accent-blue) 55%,var(--vsc-border));background:color-mix(in srgb,var(--vsc-accent-blue) 14%,transparent)}
    .journal-search{flex:1;min-width:170px;height:25px;padding:0 8px;border:1px solid var(--vsc-input-border,var(--vsc-border));border-radius:4px;background:var(--vsc-input-background);color:var(--vsc-input-foreground,var(--vsc-foreground));font:inherit;font-size:12px}
    .entries{min-height:0;overflow:auto;font-variant-numeric:tabular-nums}.entries article{display:grid;grid-template-columns:22px minmax(0,1fr);gap:7px;padding:8px 10px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border))}
    .entries article.failed{background:var(--vsc-error-surface)}
    .state{font-size:14px;text-align:center}.main{display:grid;min-width:0;gap:2px}.main small,.main>span{color:var(--vsc-description-foreground)}.main .error{color:var(--vsc-error)}details code{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font:10px var(--vsc-font-family-mono,ui-monospace,monospace)}.empty{padding:42px;text-align:center;color:var(--vsc-description-foreground)}
  `]
})
export class OperationJournalComponent implements OnChanges {
  @Input() visible=false;
  renderVisible=false;
  isClosing=false;
  filter: JournalFilter = 'all';
  searchText = '';
  private emitClosedOnFinish=false;
  @Output() closed=new EventEmitter<void>();
  @Output() undoRequested=new EventEmitter<void>();
  @Output() redoRequested=new EventEmitter<void>();
  constructor(readonly history:OperationHistoryService, private readonly dialogs:ConfirmDialogService, private readonly cdr:ChangeDetectorRef){}
  get entries():readonly OperationJournalEntry[]{return this.history.recentOperations();}
  get filteredEntries(): readonly OperationJournalEntry[] {
    const query = this.searchText.trim().toLowerCase();
    return this.entries.filter(entry => {
      if (this.filter === 'undoable' && !entry.reversible) return false;
      if (this.filter === 'failed' && entry.state !== 'failed' && entry.state !== 'partial') return false;
      if (!query) return true;
      return (
        entry.label.toLowerCase().includes(query) ||
        entry.kind.toLowerCase().includes(query) ||
        (entry.summary ?? '').toLowerCase().includes(query) ||
        (entry.error ?? '').toLowerCase().includes(query) ||
        entry.affectedPaths.some(p => p.toLowerCase().includes(query))
      );
    });
  }
  setFilter(next: JournalFilter): void { this.filter = next; this.cdr.markForCheck(); }
  onSearchInput(event: Event): void { this.searchText = (event.target as HTMLInputElement).value; this.cdr.markForCheck(); }
  ngOnChanges(changes: SimpleChanges): void {
    if (!changes['visible']) return;
    if (this.visible) {
      this.renderVisible = true;
      this.isClosing = false;
      this.emitClosedOnFinish = false;
      this.cdr.markForCheck();
    } else if (this.renderVisible && !this.isClosing) {
      this.isClosing = true;
      this.cdr.markForCheck();
    }
  }
  close():void{
    if (!this.renderVisible || this.isClosing) return;
    this.isClosing=true;
    this.emitClosedOnFinish=true;
    this.cdr.markForCheck();
  }
  onAnimationEnd(event: AnimationEvent):void{
    if(event.target!==event.currentTarget||!this.isClosing)return;
    this.renderVisible=false;
    this.isClosing=false;
    const emit=this.emitClosedOnFinish;
    this.emitClosedOnFinish=false;
    if(emit)this.closed.emit();
    this.cdr.markForCheck();
  }
  undo():void{this.undoRequested.emit();this.cdr.markForCheck();}
  redo():void{this.redoRequested.emit();this.cdr.markForCheck();}
  async clear():Promise<void>{const ok=await this.dialogs.confirm({title:'Clear Operation History',message:'Clear the visible operation journal? Undo/redo stacks are left unchanged.',confirmText:'Clear',danger:true});if(!ok)return;this.history.clearJournal();this.cdr.markForCheck();}
  stateIcon(entry:OperationJournalEntry):string{if(entry.state==='failed')return '×';if(entry.state==='partial')return '!';if(entry.state==='undone')return '↶';if(entry.state==='redone')return '↷';return '✓';}
  trackEntry(_index:number,entry:OperationJournalEntry):string{return entry.id;}
}