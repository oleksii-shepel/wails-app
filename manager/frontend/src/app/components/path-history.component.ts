import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  Input,
  OnDestroy,
  OnInit,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputEngineService } from '../services/input-engine.service';
import { PATH_HISTORY_INPUT, PATH_HISTORY_INPUT_COMMANDS } from '../services/dialog-input-catalog';
import {
  PathHistoryService,
  type LocationHistoryEntry,
} from '../services/path-history.service';

import { discoveryScore } from './discovery';
type HistoryMode = 'pane' | 'all' | 'frequent';

const CONTEXT_ID = PATH_HISTORY_INPUT.context.id;
const OWNER_ID = 'dialog.pathHistory';
const COMMAND = PATH_HISTORY_INPUT_COMMANDS;

@Component({
  selector:'app-path-history',
  standalone:true,
  imports: [CommonModule,FormsModule],
  changeDetection:ChangeDetectionStrategy.OnPush,
  template:`
    <div class="backdrop palette-dialog-backdrop dialog-group" *ngIf="visible || isClosing" [class.dialog-group--closing]="isClosing" (mousedown)="close()" (animationend)="onAnimationEnd($event)">
      <section class="history palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="Location history"
        (mousedown)="$event.stopPropagation()"
        tabindex="-1">
        <header class="palette-dialog-header">
          <div class="palette-dialog-title">
            <strong>Location History</strong>
            <span class="palette-dialog-subtitle">{{ filteredEntries.length }} / {{ modeEntries.length }}</span>
          </div>
          <div class="header-actions">
            <input [(ngModel)]="filterText" (ngModelChange)="onFilterChange()"
              placeholder="Filter history" autocomplete="off" spellcheck="false"
              aria-label="Filter location history" aria-controls="history-list" />
            <button type="button" class="dialog-close-btn" aria-label="Close" title="Close" (click)="close()">×</button>
          </div>
        </header>

        <nav class="modes palette-dialog-tabbar" aria-label="History views">
          <button type="button" [class.active]="mode==='pane'" (click)="setMode('pane')">
            This Pane
          </button>
          <button type="button" [class.active]="mode==='all'" (click)="setMode('all')">
            All
          </button>
          <button type="button" [class.active]="mode==='frequent'" (click)="setMode('frequent')">
            Frequent
          </button>
        </nav>

        <div id="history-list" class="list palette-dialog-body palette-dialog-list" role="listbox" aria-label="Location history">
          <button *ngFor="let entry of filteredEntries; let i=index; trackBy:trackEntry"
            type="button" class="entry palette-dialog-item" [class.selected]="i===selectedIndex" [class.is-selected]="i===selectedIndex"
            role="option" [attr.aria-selected]="i===selectedIndex"
            [attr.aria-posinset]="i + 1" [attr.aria-setsize]="filteredEntries.length"
            (mouseenter)="selectedIndex=i" (click)="openEntry(entry)">
            <span class="main">
              <strong>{{ labelFor(entry.location) }}</strong>
              <span class="path">{{ entry.location }}</span>
            </span>
            <span class="meta">
              <span>{{ entry.scheme || 'filesystem' }}</span>
              <span>{{ entry.paneId || '—' }}</span>
              <span *ngIf="entry.accessCount">×{{ entry.accessCount }}</span>
              <span>{{ formatTime(entry.timestamp) }}</span>
            </span>
          </button>
          <div class="empty palette-dialog-empty" *ngIf="filteredEntries.length===0">
            {{ entries.length===0 ? 'No path history yet.' : 'No matching locations.' }}
          </div>
        </div>

        <footer class="palette-dialog-footer">
          <div class="palette-dialog-footer-actions">
            <button type="button" class="palette-dialog-button" [disabled]="!selectedEntry" (click)="removeSelected()">Remove</button>
            <button type="button" class="palette-dialog-button" [disabled]="entries.length===0" (click)="copyJson()">Copy JSON</button>
            <button type="button" class="palette-dialog-button palette-dialog-button--danger" [disabled]="entries.length===0" (click)="clearAll()">Clear all</button>
          </div>
          <span class="keys palette-dialog-footnote">Ctrl+PgUp/PgDn views · ↑ ↓ · Enter · Delete · Ctrl+C · Ctrl+Delete</span>
        </footer>
      </section>
    </div>
  `,
  styles:[`
    :host{position:fixed;inset:0;z-index:9450;pointer-events:none}
    .history{box-sizing:border-box;display:flex;flex-direction:column;width:min(780px,calc(100vw - 24px));max-height:min(700px,calc(100vh - 40px))}
    .header-actions{display:flex;align-items:center;gap:8px}
    .meta,.keys{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    input{box-sizing:border-box;width:min(300px,42vw);height:28px;padding:0 8px;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:4px;outline:none;background:var(--vsc-input-background,var(--vsc-popover-background));color:inherit;font:inherit}
    .list{min-height:0;overflow:auto}.entry{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:12px;align-items:center;width:100%;min-height:45px;padding:5px 8px}.entry:hover,.entry.selected{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}.entry.selected{border-color:color-mix(in srgb,var(--vsc-accent-blue,var(--vsc-accent-blue)) 52%,transparent)}
    .main{min-width:0;display:grid}.main strong,.path{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.path{color:var(--vsc-description-foreground,var(--vsc-foreground-dim));font:10px var(--vsc-font-family-mono,ui-monospace,monospace)}.meta{display:flex;gap:8px;align-items:center;font-size:10px}.empty{padding:34px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim));text-align:center}
    .keys{margin-left:auto;font-size:10px}
  `],
})
export class PathHistoryComponent implements OnInit, OnDestroy {
  @Input() paneId:'left'|'right'='left';
  @Input() currentPath='';
  @Output() navigate=new EventEmitter<string>();

  visible=false;
  isClosing=false;
  filterText='';
  entries:LocationHistoryEntry[]=[];
  selectedIndex=0;
  mode:HistoryMode='pane';

  constructor(
    private readonly historyService:PathHistoryService,
    private readonly confirmDialog:ConfirmDialogService,
    private readonly input:InputEngineService,
    private readonly cdr:ChangeDetectorRef,
  ){}

  get modeEntries():LocationHistoryEntry[]{
    if(this.mode==='pane'){
      const scoped=this.entries.filter(entry=>entry.paneId===this.paneId);
      return scoped.length?scoped:this.entries;
    }
    if(this.mode==='frequent'){
      return [...this.entries].sort((a,b)=>
        b.accessCount-a.accessCount||b.timestamp-a.timestamp
      ).filter(entry=>entry.accessCount>=2);
    }
    return this.entries;
  }

  get filteredEntries():LocationHistoryEntry[]{
    const query=this.filterText.trim();
    const current=this.locationKey(this.currentPath);
    const entries=this.modeEntries.filter(entry=>
      this.locationKey(entry.location)!==current
    );

    if(!query)return entries;

    return entries
      .map(entry=>({
        entry,
        score:discoveryScore(query,[
          {value:this.labelFor(entry.location),weight:1.6},
          {value:entry.location,weight:1.2},
          {value:entry.scheme,weight:.8},
          {value:entry.paneId,weight:.5},
        ]),
      }))
      .filter(candidate=>candidate.score>=0)
      .sort((a,b)=>
        b.score-a.score||
        b.entry.timestamp-a.entry.timestamp
      )
      .map(candidate=>candidate.entry);
  }

  get selectedEntry():LocationHistoryEntry|null{
    return this.filteredEntries[this.selectedIndex]??null;
  }

  ngOnInit():void{
    this.input.registerCommandDescriptor({id:COMMAND.close,title:'Close Path History',category:'Navigation',execute:()=>this.close()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previous,title:'Previous Path History Entry',category:'Navigation',execute:()=>this.move(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.next,title:'Next Path History Entry',category:'Navigation',execute:()=>this.move(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.open,title:'Open Path History Entry',category:'Navigation',execute:()=>{const entry=this.selectedEntry;if(entry)this.openEntry(entry)}},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.remove,title:'Remove Path History Entry',category:'Navigation',execute:()=>this.removeSelected(),isEnabled:()=>!!this.selectedEntry},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.clear,title:'Clear Path History',category:'Navigation',execute:()=>this.clearAll(),isEnabled:()=>this.entries.length>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previousMode,title:'Previous History View',category:'Navigation',execute:()=>this.moveMode(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.nextMode,title:'Next History View',category:'Navigation',execute:()=>this.moveMode(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.copyJson,title:'Copy History JSON',category:'Navigation',execute:()=>this.copyJson(),isEnabled:()=>this.entries.length>0},OWNER_ID);
  }

  ngOnDestroy():void{
    this.input.deactivateContext(CONTEXT_ID);
    for(const id of Object.values(COMMAND))this.input.unregisterCommandDescriptor(id,OWNER_ID);
  }

  open(mode:HistoryMode='pane'):void{
    
    this.entries=this.historyService.getAllHistory();
    this.mode=mode;
    this.filterText='';
    this.selectedIndex=0;
    this.isClosing=false;
    this.visible=true;
    this.input.activateContext(CONTEXT_ID);
    this.cdr.markForCheck();
  }

  close():void{
    if(!this.visible||this.isClosing)return;
    this.isClosing=true;
    this.input.deactivateContext(CONTEXT_ID);
    
    
    this.cdr.markForCheck();
  }

  onAnimationEnd(event:AnimationEvent):void{
    if(event.target!==event.currentTarget||!this.isClosing)return;
    this.visible=false;
    this.isClosing=false;
    this.cdr.markForCheck();
  }

  setMode(mode:HistoryMode):void{
    this.mode=mode;
    this.selectedIndex=0;
    this.cdr.markForCheck();
  }

  openEntry(entry:LocationHistoryEntry):void{
    this.navigate.emit(entry.location);
    this.close();
  }

  removeSelected():void{
    const entry=this.selectedEntry;
    if(!entry)return;
    this.historyService.removeLocation(entry.location);
    this.reload();
  }

  async clearAll():Promise<void>{
    if(!this.entries.length)return;
    const ok=await this.confirmDialog.confirm({
      title:'Clear Location History',
      message:'Clear all path history?',
      confirmText:'Clear',
      danger:true,
    });
    if(!ok)return;
    this.historyService.clearHistory();
    this.reload();
  }

  async copyJson():Promise<void>{
    if(!this.entries.length)return;
    const content=JSON.stringify(this.entries,null,2);
    try{
      await navigator.clipboard.writeText(content);
    }catch{
      await this.confirmDialog.alert({
        title:'Location History JSON',
        message:content,
      });
    }
  }

  onFilterChange():void{
    this.selectedIndex=0;
    this.cdr.markForCheck();
  }

  trackEntry(_index:number,entry:LocationHistoryEntry):string{
    return entry.location;
  }

  labelFor(location:string):string{
    const normalized=(location??'').replace(/\\/g,'/').replace(/\/+$/,'');
    const parts=normalized.split('/').filter(Boolean);
    return parts[parts.length-1]||normalized||location;
  }

  formatTime(timestamp:number):string{
    if(!timestamp)return'';
    const delta=Math.max(0,Date.now()-timestamp);
    if(delta<60_000)return'now';
    if(delta<3_600_000)return`${Math.floor(delta/60_000)}m`;
    if(delta<86_400_000)return`${Math.floor(delta/3_600_000)}h`;
    if(delta<604_800_000)return`${Math.floor(delta/86_400_000)}d`;
    return new Date(timestamp).toLocaleDateString();
  }

  private reload():void{
    this.entries=this.historyService.getAllHistory();
    this.selectedIndex=Math.min(
      this.selectedIndex,
      Math.max(0,this.filteredEntries.length-1),
    );
    this.cdr.markForCheck();
  }

  private move(delta:-1|1):void{
    const count=this.filteredEntries.length;
    if(!count)return;
    this.selectedIndex=(this.selectedIndex+delta+count)%count;
    this.cdr.markForCheck();
  }

  private moveMode(delta:-1|1):void{
    const modes:HistoryMode[]=['pane','all','frequent'];
    const index=modes.indexOf(this.mode);
    this.setMode(modes[(index+delta+modes.length)%modes.length]);
  }

  private locationKey(location:string):string{
    return (location??'').trim().replace(/\\/g,'/').replace(/\/+$/,'').toLocaleLowerCase();
  }
}