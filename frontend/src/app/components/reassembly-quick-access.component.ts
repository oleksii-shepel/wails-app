import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, OnDestroy, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { ApiService } from '../services/api.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { PathHistoryService } from '../services/path-history.service';
import { PinnedLocationService } from '../services/pinned-location.service';
import type { PinnedLocation } from './address-bar.types';
import type { SpecialFolder } from '../shared/protocol';

interface QuickItem {
  id: string;
  label: string;
  path: string;
  icon: string;
  kind: 'system' | 'pin' | 'frequent' | 'recent';
  meta?: string;
}

interface QuickSection {
  id: string;
  title: string;
  items: QuickItem[];
}

const CONTEXT_ID = 'overlay.quickAccess';
const OWNER_ID = 'overlay.quickAccess';

const COMMAND = {
  close: 'reassembly.quickAccess.close',
  previous: 'reassembly.quickAccess.previous',
  next: 'reassembly.quickAccess.next',
  previousSection: 'reassembly.quickAccess.previousSection',
  nextSection: 'reassembly.quickAccess.nextSection',
  open: 'reassembly.quickAccess.open',
  removePin: 'reassembly.quickAccess.removePin',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  { id:'reassembly.quickAccess.escape', commandId:COMMAND.close, contextId:CONTEXT_ID, source:'default', description:'Close Quick Access', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'escape'}]}}},
  { id:'reassembly.quickAccess.up', commandId:COMMAND.previous, contextId:CONTEXT_ID, source:'default', description:'Previous item', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup'}]}}},
  { id:'reassembly.quickAccess.down', commandId:COMMAND.next, contextId:CONTEXT_ID, source:'default', description:'Next item', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown'}]}}},
  { id:'reassembly.quickAccess.shiftTab', commandId:COMMAND.previousSection, contextId:CONTEXT_ID, source:'default', description:'Previous section', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'tab', modifiers:['shift']}]}}},
  { id:'reassembly.quickAccess.tab', commandId:COMMAND.nextSection, contextId:CONTEXT_ID, source:'default', description:'Next section', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'tab'}]}}},
  { id:'reassembly.quickAccess.enter', commandId:COMMAND.open, contextId:CONTEXT_ID, source:'default', description:'Open item', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'enter'}]}}},
  { id:'reassembly.quickAccess.delete', commandId:COMMAND.removePin, contextId:CONTEXT_ID, source:'default', description:'Remove pin', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'delete'}]}}},
];

@Component({
  selector: 'app-reassembly-quick-access',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="close()">
      <section class="quick" role="dialog" aria-modal="true" aria-label="Quick Access" (mousedown)="$event.stopPropagation()">
        <header>
          <div><strong>Quick Access</strong><span>{{ totalItems }} items</span></div>
          <input [(ngModel)]="filterText" (ngModelChange)="resetSelection()" placeholder="Filter locations"
            autocomplete="off" spellcheck="false" aria-label="Filter Quick Access" />
        </header>

        <nav class="sections" *ngIf="filteredSections.length > 1">
          <button *ngFor="let section of filteredSections; let i=index" type="button"
            [class.active]="i === sectionIndex" (click)="selectSection(i)">
            {{ section.title }} <span>{{ section.items.length }}</span>
          </button>
        </nav>

        <div class="items" role="listbox">
          <button *ngFor="let item of activeItems; let i=index; trackBy:trackItem" type="button"
            class="item" [class.selected]="i === itemIndex" role="option"
            [attr.aria-selected]="i === itemIndex" (mouseenter)="itemIndex=i" (click)="openItem(item)">
            <span class="icon">{{ item.icon }}</span>
            <span class="details"><strong>{{ item.label }}</strong><small>{{ item.path }}</small></span>
            <span class="meta">{{ item.meta || item.kind }}</span>
          </button>
          <div class="empty" *ngIf="activeItems.length === 0">No matching locations.</div>
        </div>

        <footer>
          <span>{{ activeSection?.title || '' }}</span>
          <button type="button" [disabled]="selectedItem?.kind !== 'pin'" (click)="removeSelectedPin()">Unpin</button>
          <span class="keys">Tab · ↑ ↓ · Enter · Delete · Esc</span>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:9460;pointer-events:none}
    .backdrop{position:fixed;inset:0;display:grid;place-items:start center;padding-top:min(10vh,90px);background:rgba(0,0,0,.23);pointer-events:auto}
    .quick{box-sizing:border-box;display:flex;flex-direction:column;width:min(720px,calc(100vw - 24px));max-height:min(680px,calc(100vh - 40px));overflow:hidden;border:1px solid var(--vsc-border,#454545);border-radius:8px;background:var(--vsc-menu-background,#252526);color:var(--vsc-foreground,#cccccc);box-shadow:0 18px 60px rgba(0,0,0,.48);font:12px/1.4 var(--vsc-font-family,system-ui,sans-serif)}
    header,footer{display:flex;align-items:center;gap:9px;padding:9px 10px} header{justify-content:space-between;border-bottom:1px solid var(--vsc-border-subtle,#2b2b2b)} header>div{display:flex;gap:8px;align-items:baseline}
    header span,.meta,.keys,footer>span,small{color:var(--vsc-description-foreground,#969696)}
    input{box-sizing:border-box;width:min(300px,42vw);height:28px;padding:0 8px;border:1px solid var(--vsc-border,#454545);border-radius:4px;outline:none;background:var(--vsc-input-background,#181818);color:inherit;font:inherit} input:focus{border-color:var(--vsc-focus-border,#007fd4)}
    .sections{display:flex;gap:2px;padding:5px;overflow-x:auto;border-bottom:1px solid var(--vsc-border-subtle,#2b2b2b)}
    .sections button,footer button{min-height:27px;padding:3px 8px;border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;cursor:pointer;font:inherit}
    .sections button{color:var(--vsc-description-foreground,#969696)} .sections button.active{background:var(--vsc-list-hover-background,#2a2d2e);color:var(--vsc-foreground,#cccccc);border-color:var(--vsc-border,#454545)} .sections span{margin-left:5px;opacity:.6;font-size:10px}
    .items{min-height:0;overflow:auto;padding:5px}.item{display:grid;grid-template-columns:30px minmax(0,1fr) auto;gap:8px;align-items:center;width:100%;min-height:46px;padding:5px 8px;border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;text-align:left;cursor:pointer;font:inherit}.item:hover,.item.selected{background:var(--vsc-list-hover-background,#2a2d2e)}.item.selected{border-color:color-mix(in srgb,var(--vsc-accent-blue,#007acc) 50%,transparent)}
    .icon{text-align:center;font-size:16px}.details{min-width:0;display:grid}.details strong,.details small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.details small{font-family:var(--vsc-font-family-mono,ui-monospace,monospace)}.meta{font-size:10px}.empty{padding:38px;text-align:center;color:var(--vsc-description-foreground,#969696)}
    footer{border-top:1px solid var(--vsc-border-subtle,#2b2b2b)} footer button:hover:not(:disabled){background:var(--vsc-list-hover-background,#2a2d2e);border-color:var(--vsc-border,#454545)}footer button:disabled{opacity:.35;cursor:default}.keys{margin-left:auto;font-size:10px}
  `],
})
export class ReassemblyQuickAccessComponent implements OnInit, OnDestroy {
  @Output() navigate = new EventEmitter<string>();

  visible = false;
  filterText = '';
  sections: QuickSection[] = [];
  sectionIndex = 0;
  itemIndex = 0;
  private pins: PinnedLocation[] = [];

  constructor(
    private readonly api: ApiService,
    private readonly history: PathHistoryService,
    private readonly pinnedLocations: PinnedLocationService,
    private readonly input: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get filteredSections(): QuickSection[] {
    const query = this.filterText.trim().toLocaleLowerCase();
    if (!query) return this.sections;
    return this.sections.map(section => ({
      ...section,
      items: section.items.filter(item =>
        item.label.toLocaleLowerCase().includes(query) ||
        item.path.toLocaleLowerCase().includes(query)),
    })).filter(section => section.items.length > 0);
  }

  get activeSection(): QuickSection | null { return this.filteredSections[this.sectionIndex] ?? null; }
  get activeItems(): QuickItem[] { return this.activeSection?.items ?? []; }
  get selectedItem(): QuickItem | null { return this.activeItems[this.itemIndex] ?? null; }
  get totalItems(): number { return this.filteredSections.reduce((sum,s) => sum + s.items.length, 0); }

  ngOnInit(): void {
    this.input.registerContext({id:CONTEXT_ID,kind:'overlay',priority:380,parentId:'global',description:'Quick Access'});
    this.input.registerCommandDescriptor({id:COMMAND.close,title:'Close Quick Access',category:'Navigation',execute:()=>this.close()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previous,title:'Previous Quick Access Item',category:'Navigation',execute:()=>this.moveItem(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.next,title:'Next Quick Access Item',category:'Navigation',execute:()=>this.moveItem(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previousSection,title:'Previous Quick Access Section',category:'Navigation',execute:()=>this.moveSection(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.nextSection,title:'Next Quick Access Section',category:'Navigation',execute:()=>this.moveSection(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.open,title:'Open Quick Access Item',category:'Navigation',execute:()=>{const item=this.selectedItem;if(item)this.openItem(item)}},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.removePin,title:'Remove Quick Access Pin',category:'Navigation',execute:()=>this.removeSelectedPin(),isEnabled:()=>this.selectedItem?.kind==='pin'},OWNER_ID);
    for (const binding of BINDINGS) this.input.registerBinding(binding);
  }

  ngOnDestroy(): void {
    this.input.deactivateContext(CONTEXT_ID);
    for (const binding of BINDINGS) this.input.unregisterBinding(binding.id);
    for (const id of Object.values(COMMAND)) this.input.unregisterCommandDescriptor(id, OWNER_ID);
  }

  async open(sectionId?: string): Promise<void> {
    await this.reload();

    this.filterText = '';
    this.itemIndex = 0;

    if (sectionId) {
      const index = this.sections.findIndex(section => section.id === sectionId);
      this.sectionIndex = index >= 0 ? index : 0;
    } else {
      this.sectionIndex = 0;
    }

    this.visible = true;
    this.input.activateContext(CONTEXT_ID);
    this.cdr.markForCheck();
  }

  close(): void { if(!this.visible)return; this.visible=false; this.input.deactivateContext(CONTEXT_ID); this.cdr.markForCheck(); }
  openItem(item: QuickItem): void { this.navigate.emit(item.path); this.close(); }
  selectSection(index:number):void{this.sectionIndex=Math.max(0,Math.min(index,this.filteredSections.length-1));this.itemIndex=0;this.cdr.markForCheck()}
  resetSelection():void{this.sectionIndex=0;this.itemIndex=0;this.cdr.markForCheck()}
  trackItem(_index:number,item:QuickItem):string{return item.id}

  removeSelectedPin(): void {
    const item=this.selectedItem; if(!item||item.kind!=='pin')return;
    this.pins=this.pinnedLocations.remove(this.pins,item.path,'dir');
    this.pinnedLocations.save(this.pins);
    void this.reload();
  }

  private async reload(): Promise<void> {
    const specialFolders: SpecialFolder[] = await this.api.getSpecialFolders().catch(()=>[]);
    this.pins=this.pinnedLocations.load();
    const frequent=this.history.getFrequentLocations(8,2);
    const recent=this.history.getRecentLocations(10);

    this.sections=[
      {id:'system',title:'System',items:specialFolders.map(folder=>({id:`system:${folder.path}`,label:folder.name,path:folder.path,icon:folder.icon||'📁',kind:'system' as const}))},
      {id:'pins',title:'Pinned',items:this.pins.filter(pin=>pin.surfaceId==='dir'||!pin.surfaceId).map(pin=>({id:`pin:${pin.id}`,label:pin.label,path:pin.location,icon:pin.icon||'📌',kind:'pin' as const}))},
      {id:'frequent',title:'Frequent',items:frequent.map(entry=>({id:`frequent:${entry.location}`,label:this.labelFor(entry.location),path:entry.location,icon:'↻',meta:entry.accessCount?`×${entry.accessCount}`:undefined,kind:'frequent' as const}))},
      {id:'recent',title:'Recent',items:recent.map(entry=>({id:`recent:${entry.location}`,label:this.labelFor(entry.location),path:entry.location,icon:'◷',kind:'recent' as const}))},
    ].filter(section=>section.items.length>0);

    this.sectionIndex=Math.min(this.sectionIndex,Math.max(0,this.sections.length-1));
    this.itemIndex=0; this.cdr.markForCheck();
  }

  private moveItem(delta:-1|1):void{const count=this.activeItems.length;if(!count)return;this.itemIndex=(this.itemIndex+delta+count)%count;this.cdr.markForCheck()}
  private moveSection(delta:-1|1):void{const count=this.filteredSections.length;if(!count)return;this.sectionIndex=(this.sectionIndex+delta+count)%count;this.itemIndex=0;this.cdr.markForCheck()}
  private labelFor(path:string):string{const normalized=(path||'').replace(/\\/g,'/').replace(/\/+$/,'');const parts=normalized.split('/').filter(Boolean);return parts[parts.length-1]||normalized||path}
}
