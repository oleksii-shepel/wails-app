import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OperationHistoryService, type OperationJournalEntry } from '../services/operation-history.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { LocalDateTimePipe } from '../utils/local-date-time.pipe';
@Component({
  selector: 'app-operation-journal',
  standalone: true,
  imports: [LocalDateTimePipe, CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="close()">
      <section class="journal" role="dialog" aria-modal="true" aria-label="Operation History"
        (mousedown)="$event.stopPropagation()">
        <header><div><strong>Operation History</strong><span>{{ entries.length }} recent operations</span></div>
          <button type="button" class="close" (click)="close()">×</button></header>
        <div class="entries">
          <article *ngFor="let entry of entries; trackBy: trackEntry">
            <span class="state">{{ stateIcon(entry) }}</span>
            <div class="main"><strong>{{ entry.label }}</strong>
              <small>{{ entry.kind }} · {{ entry.completedAt | localDateTime }} · {{ entry.reversible ? 'undoable' : 'not undoable' }}</small>
              <span *ngIf="entry.summary">{{ entry.summary }}</span>
              <span class="error" *ngIf="entry.error">{{ entry.error }}</span>
              <details *ngIf="entry.affectedPaths.length"><summary>{{ entry.affectedPaths.length }} affected paths</summary>
                <code *ngFor="let path of entry.affectedPaths">{{ path }}</code></details>
            </div>
          </article>
          <div class="empty" *ngIf="entries.length === 0">No file operations recorded in this session.</div>
        </div>
        <footer><div><button [disabled]="!history.canUndo" (click)="undo()">Undo</button>
          <button [disabled]="!history.canRedo" (click)="redo()">Redo</button></div>
          <div><button [disabled]="entries.length === 0" (click)="clear()">Clear History</button>
          <button class="close" (click)="close()">Close</button></div></footer>
      </section>
    </div>`,
  styles: [`
    :host{position:fixed;inset:0;z-index:8700;pointer-events:none}.backdrop{position:absolute;inset:0;display:grid;place-items:center;padding:20px;background:var(--vsc-scrim);pointer-events:auto}
    .journal{display:grid;grid-template-rows:auto minmax(0,1fr) auto;width:min(780px,calc(100vw - 40px));height:min(680px,calc(100vh - 40px));overflow:hidden;border:1px solid var(--vsc-border);border-radius:7px;background:var(--vsc-dialog-background,var(--vsc-panel-background));color:var(--vsc-foreground);box-shadow:0 14px 48px var(--vsc-scrim);font:11px/1.35 var(--vsc-font-family,system-ui,sans-serif)}
    header,footer{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:8px 10px;border-color:var(--vsc-border-subtle,var(--vsc-border))}header{border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border))}header>div{display:grid}header span{color:var(--vsc-description-foreground)}
    footer{border-top:1px solid var(--vsc-border-subtle,var(--vsc-border))}footer>div{display:flex;gap:5px}button{min-height:24px;padding:2px 8px;border:1px solid var(--vsc-border);border-radius:3px;background:var(--vsc-button-secondary-background,var(--vsc-control-background));color:inherit;font:inherit;cursor:pointer}button:disabled{opacity:.4;cursor:default}
    .entries{min-height:0;overflow:auto}.entries article{display:grid;grid-template-columns:22px minmax(0,1fr);gap:7px;padding:8px 10px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border))}.state{font-size:14px;text-align:center}.main{display:grid;min-width:0;gap:2px}.main small,.main>span{color:var(--vsc-description-foreground)}.main .error{color:var(--vsc-error)}details code{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font:10px var(--vsc-font-family-mono,ui-monospace,monospace)}.empty{padding:42px;text-align:center;color:var(--vsc-description-foreground)}
  `]
})
export class OperationJournalComponent {
  @Input() visible=false;
  @Output() closed=new EventEmitter<void>();
  @Output() undoRequested=new EventEmitter<void>();
  @Output() redoRequested=new EventEmitter<void>();
  constructor(readonly history:OperationHistoryService, private readonly dialogs:ConfirmDialogService, private readonly cdr:ChangeDetectorRef){}
  get entries():readonly OperationJournalEntry[]{return this.history.recentOperations();}
  close():void{this.closed.emit();}
  undo():void{this.undoRequested.emit();this.cdr.markForCheck();}
  redo():void{this.redoRequested.emit();this.cdr.markForCheck();}
  async clear():Promise<void>{const ok=await this.dialogs.confirm({title:'Clear Operation History',message:'Clear the visible operation journal? Undo/redo stacks are left unchanged.',confirmText:'Clear',danger:true});if(!ok)return;this.history.clearJournal();this.cdr.markForCheck();}
  stateIcon(entry:OperationJournalEntry):string{if(entry.state==='failed')return '×';if(entry.state==='partial')return '!';if(entry.state==='undone')return '↶';if(entry.state==='redone')return '↷';return '✓';}
  trackEntry(_index:number,entry:OperationJournalEntry):string{return entry.id;}
}